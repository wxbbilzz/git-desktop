// Package engine 是 lazygit 的“无界面核心”封装。
//
// 设计要点（对应方案 B：复用核心，重写 UI）：
//
//	UI(任意技术栈) --意图--> Engine --调用--> lazygit pkg/commands/*
//	UI(任意技术栈) <--快照-- Engine
//
// 我们复用的是 lazygit 真正的资产：git 命令封装、模型、patch 解析、配置、i18n。
// 我们重写的是展示与交互：这里完全不引入 pkg/gui 的视图 / 上下文 / 控制器。
//
// 唯一残留的耦合是 gocui.Task（push / pull / fetch 等长任务的取消句柄），
// 因为它的接口带未导出方法，外部包无法自行实现，所以临时借用 gocui.NewFakeTask()。
// 后续要彻底解耦，只需把 git_commands 里那几个方法的 Task 参数换成自定义接口即可。
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jesseduffield/lazygit/pkg/commands"
	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/git_config"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/common"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/i18n"
	"github.com/jesseduffield/lazygit/pkg/logs"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// Engine 持有一个仓库会话，并对外暴露“读快照 / 执行动作”两类方法。
// 所有方法都是并发安全的。
type Engine struct {
	mu sync.Mutex

	log       *logrus.Entry
	cmn       *common.Common
	appConfig *config.AppConfig

	os           *oscommands.OSCommand
	git          *commands.GitCommand
	gitConfig    git_config.IGitConfig
	cmdBuilder   oscommands.ICmdObjBuilder
	mainBranches *git_commands.MainBranches

	repoPath string

	// pushRunner 是推送用的执行器，生产环境下始终是 e.gitRun。
	// 抽成字段是为了在测试里注入「前几次失败、然后成功」的瞬时故障，
	// 用来验证重试确实生效。
	pushRunner func(args ...string) (string, error)

	// onRepoChange 在仓库被外部改动时被调用（由 app 层转成前端事件）。
	// 由 SetRepoChangeHandler 设置；没设置就不监听。
	onRepoChange func()
	// watcher 是当前仓库的文件监听器，切换仓库时会被替换
	watcher *repoWatcher

	// 丢弃文件的回收站（见 restore.go），只存在内存里
	discards   []DiscardRecord
	discardSeq int
}

// New 构造引擎并加载一次用户配置。此时还没有打开任何仓库。
func New() (*Engine, error) {
	log := logs.NewProductionLogger()

	appConfig, err := config.NewAppConfig(
		"bingit",
		"1.0.1.0",
		"",
		"",
		"bingit",
		false,
		os.TempDir(),
	)
	if err != nil {
		return nil, fmt.Errorf("加载用户配置失败: %w", err)
	}

	cmn := &common.Common{
		Log:      log,
		Tr:       i18n.EnglishTranslationSet(),
		AppState: appConfig.GetAppState(),
		Debug:    appConfig.GetDebug(),
		Fs:       afero.NewOsFs(),
	}
	cmn.SetUserConfig(appConfig.GetUserConfig())

	return &Engine{
		log:       log,
		cmn:       cmn,
		appConfig: appConfig,
	}, nil
}

// RepoPath 返回当前打开的仓库工作区路径；没有打开仓库时返回空串。
func (e *Engine) RepoPath() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.repoPath
}

// OpenRepo 打开（或切换到）一个 git 仓库，并返回初始快照。
//
// 这一段的初始化顺序是照抄 lazygit 自己的启动逻辑（pkg/gui/gui.go 里
// onNewRepo 与 NewGui 对 osCommand / gitConfig / diffRenderer 的组装），
// 只是把最后的 GUI 换成了我们的引擎。
func (e *Engine) OpenRepo(path string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.openRepoLocked(path)
}

func (e *Engine) openRepoLocked(path string) (*RepoSnapshot, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("不是有效目录: %s", abs)
	}

	// lazygit 的命令层假设进程工作目录就在仓库里。
	if err := os.Chdir(abs); err != nil {
		return nil, err
	}

	osCommand := oscommands.NewOSCommand(
		e.cmn,
		e.appConfig,
		oscommands.GetPlatform(),
		oscommands.NewNullGuiIO(e.log),
	)

	gitVersion, err := git_commands.GetGitVersion(osCommand)
	if err != nil {
		return nil, fmt.Errorf("未找到可用的 git（lazygit 需要 2.32.0 以上）: %w", err)
	}

	diffRendererConfig := config.NewDiffRendererConfigManager(func() *config.UserConfig {
		return e.cmn.UserConfig()
	})

	// 复用同一个 config 读取器，这样设置身份后能直接清缓存见效
	if e.gitConfig == nil {
		e.gitConfig = git_config.NewStdCachedGitConfig(e.log)
	}

	git, err := commands.NewGitCommand(
		e.cmn,
		gitVersion,
		osCommand,
		e.gitConfig,
		diffRendererConfig,
	)
	if err != nil {
		return nil, err
	}

	repoPaths := git.RepoPaths

	// git_commands 内部会用这个 builder 拼接 git 命令；
	// MainBranches 则用于判断提交是否已推送。
	e.cmdBuilder = commands.NewGitCmdObjBuilder(
		e.log,
		osCommand.Cmd,
		repoPaths.WorktreePath(),
		repoPaths.GitLocationEnvVars(),
	)
	e.mainBranches = git_commands.NewMainBranches(e.cmn, e.cmdBuilder)

	e.os = osCommand
	e.git = git
	e.repoPath = repoPaths.WorktreePath()

	// 仓库换了就重新挂监听（旧的要先摘掉，否则会同时监听两个目录）
	e.restartWatcherLocked(e.repoPath)

	return e.snapshotLocked()
}

// SetRepoChangeHandler 注册「仓库有变化」的回调。
//
// app 层用它把变化转成前端事件；引擎本身不关心事件怎么送达。
// 必须在 OpenRepo 之前设置才会生效 —— 启动时先注册、再打开仓库是固定的顺序。
func (e *Engine) SetRepoChangeHandler(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onRepoChange = fn
}

// restartWatcherLocked 换掉当前的文件监听器。调用方需持有 e.mu。
func (e *Engine) restartWatcherLocked(repoPath string) {
	if e.watcher != nil {
		e.watcher.Stop()
		e.watcher = nil
	}
	if e.onRepoChange == nil || repoPath == "" {
		return
	}
	w, err := startRepoWatcher(repoPath, e.onRepoChange)
	if err != nil {
		// 监听不上不影响使用，只是没有自动刷新
		e.log.Warnf("无法监听仓库变化，自动刷新已关闭: %v", err)
		return
	}
	e.watcher = w
}

// Snapshot 读取当前仓库的完整状态快照。
func (e *Engine) Snapshot() (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshotLocked()
}

func (e *Engine) snapshotLocked() (*RepoSnapshot, error) {
	if e.git == nil {
		return nil, fmt.Errorf("尚未打开仓库")
	}

	snap := &RepoSnapshot{
		RepoPath:   e.repoPath,
		RepoName:   filepath.Base(e.repoPath),
		Files:      []FileDTO{},
		Commits:    []CommitDTO{},
		Branches:   []BranchDTO{},
		IsDetached: e.git.Branch.IsHeadDetached(),
	}

	if branch, err := e.git.Branch.CurrentBranchName(); err == nil && branch != "" {
		snap.Branch = branch
	}
	// 游离 HEAD 时 CurrentBranchName 返回空串，顶栏会什么都不显示。
	// 这里退化成显示提交号，界面再配合 IsDetached 标注「游离 HEAD」。
	if snap.IsDetached && snap.Branch == "" {
		if head, err := e.gitOutput("rev-parse", "--short", "HEAD"); err == nil {
			snap.Branch = strings.TrimSpace(head)
		}
	}
	snap.State = stateLabel(e.git.Status.WorkingTreeState())

	// 工作区 / 暂存区文件
	for _, f := range e.git.Loaders.FileLoader.GetStatusFiles(git_commands.GetStatusFileOptions{}) {
		if f.IsWorktree {
			continue
		}
		snap.Files = append(snap.Files, toFileDTO(f))
	}

	// 提交身份：没配置的话 git 会拒绝提交，界面需要据此给出引导
	snap.IdentityName, snap.IdentityEmail = e.identityLocked()

	snap.Commits = e.loadCommitsLocked()
	snap.Branches = e.loadBranchesLocked()
	snap.Tags = e.loadTagsLocked()
	snap.Remotes = e.loadRemotesLocked()
	snap.RemoteBranches = e.loadRemoteBranchesLocked()
	snap.CanUndo, snap.UndoHint = e.undoInfoLocked()

	return snap, nil
}

// undoInfoLocked 判断「撤销上一步」当前是否可用，以及撤销会撤掉什么。
//
// 判断依据是 reflog：它记录了这个仓库上每一次 HEAD 的移动。
// 只有两条以上记录、且不处于变基/合并中间状态时，撤销才有意义。
func (e *Engine) undoInfoLocked() (bool, string) {
	state := e.git.Status.WorkingTreeState()
	if state.Rebasing || state.Merging || state.CherryPicking || state.Reverting {
		return false, ""
	}
	entries, err := e.reflogLocked(2)
	if err != nil || len(entries) < 2 {
		return false, ""
	}
	return true, "撤销「" + entries[0].Subject + "」"
}

// loadCommitsLocked 读取提交历史。
//
// 注意 RefForPushedStatus 传 nil 是允许的（CommitLoader 会跳过未推送标记），
// 但 MainBranches 不能为 nil，否则内部会直接解引用 panic。
func (e *Engine) loadCommitsLocked() []CommitDTO {
	commits, err := e.git.Loaders.CommitLoader.GetCommits(git_commands.GetCommitsOptions{
		Limit:        true,
		RefName:      "HEAD",
		All:          false,
		MainBranches: e.mainBranches,
		HashPool:     &utils.StringPool{},
	})
	if err != nil {
		// 仓库还没有任何提交时 `git log HEAD` 会失败，这里按“暂无历史”处理。
		return []CommitDTO{}
	}

	result := make([]CommitDTO, 0, len(commits))
	for _, c := range commits {
		result = append(result, toCommitDTO(c))
	}
	return result
}

func (e *Engine) loadBranchesLocked() []BranchDTO {
	// onWorker / renderFunc 是 lazygit 用来把加载放到后台并触发重绘的回调。
	// 在引擎里我们同步执行、不重绘。
	branches, err := e.git.Loaders.BranchLoader.Load(
		nil,
		e.mainBranches,
		nil,
		false,
		func(f func() error) { _ = f() },
		func() {},
	)
	if err != nil {
		return []BranchDTO{}
	}

	result := make([]BranchDTO, 0, len(branches))
	for _, b := range branches {
		result = append(result, toBranchDTO(b))
	}
	return result
}

// Close 释放资源：停掉文件监听并断开当前仓库。
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.watcher != nil {
		e.watcher.Stop()
		e.watcher = nil
	}
	e.git = nil
	e.os = nil
	return nil
}
