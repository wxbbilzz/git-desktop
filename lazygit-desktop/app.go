package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"lazygit-desktop/engine"
)

// App 是暴露给前端的对象。
//
// Wails 会把这里所有导出方法生成成 window.go.main.App.* 供 TypeScript 调用，
// 所以我们不需要手写任何 IPC 协议 —— 这正是选 Wails 的主要原因。
//
// 这一层刻意做得很薄：它只负责「转发到引擎」和「需要窗口上下文的事（如目录选择框）」，
// 真正的 git 逻辑全部在 engine 包里。
type App struct {
	ctx    context.Context
	engine *engine.Engine
	// 启动目录如果不是 git 仓库，记在这里，交给前端问「要不要初始化」
	startupNotRepo string
}

func NewApp() *App {
	eng, err := engine.New()
	if err != nil {
		panic(err)
	}
	return &App{engine: eng}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 把文件夹（或文件）拖进窗口即可打开它所在的仓库。
	// Wails 会把拖进来的绝对路径交给我们，这里往上找 .git，
	// 所以拖仓库里的任意子目录也能正确打开整个仓库。
	runtime.OnFileDrop(ctx, func(_ int, _ int, paths []string) {
		if len(paths) == 0 {
			return
		}
		dir, err := resolveDroppedRepo(paths[0])
		if err != nil {
			runtime.EventsEmit(ctx, "repo:drop-failed", paths[0], err.Error())
			return
		}
		// 交给前端去调 OpenRepo，这样加载状态和错误提示都在一处
		runtime.EventsEmit(ctx, "repo:dropped", dir)
	})

	// 尽力打开启动目录。
	// 如果不是 git 仓库，记下来交给前端 —— 前端会问用户
	// 「要不要在这个文件夹里建一个仓库」，而不是干巴巴地显示欢迎页。
	if wd, err := os.Getwd(); err == nil {
		if _, err := a.engine.OpenRepo(wd); err != nil {
			a.startupNotRepo = wd
		}
	}
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.engine.Close()
}

// ---------------------------------------------------------------------------
// 读取
// ---------------------------------------------------------------------------

// Snapshot 返回当前仓库的完整状态快照。
func (a *App) Snapshot() (*engine.RepoSnapshot, error) {
	return a.engine.Snapshot()
}

// PickRepo 弹出系统目录选择框，返回用户选中的路径（取消则返回空串）。
func (a *App) PickRepo() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("应用尚未就绪")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 Git 仓库",
	})
}

// ChooseAndOpenRepo 弹出目录选择框并打开选中的仓库。
// 用户取消时返回 (nil, nil)，前端据此不做任何事。
func (a *App) ChooseAndOpenRepo() (*engine.RepoSnapshot, error) {
	dir, err := a.PickRepo()
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, nil
	}
	return a.engine.OpenRepo(dir)
}

// OpenRepo 直接按路径打开仓库。
func (a *App) OpenRepo(path string) (*engine.RepoSnapshot, error) {
	return a.engine.OpenRepo(path)
}

// FileDiff 取文件 diff；staged 为 true 时看暂存区。
func (a *App) FileDiff(path string, staged bool) (string, error) {
	return a.engine.FileDiff(path, staged)
}

// CommitDiff 取某次提交的 diff。
func (a *App) CommitDiff(hash string) (string, error) {
	return a.engine.CommitDiff(hash)
}

// ---------------------------------------------------------------------------
// 动作（都返回操作后的新快照，前端直接替换本地状态即可）
// ---------------------------------------------------------------------------

func (a *App) StageFile(path string) (*engine.RepoSnapshot, error) {
	return a.engine.StageFile(path)
}

func (a *App) UnstageFile(path string) (*engine.RepoSnapshot, error) {
	return a.engine.UnstageFile(path)
}

func (a *App) StageAll() (*engine.RepoSnapshot, error) {
	return a.engine.StageAll()
}

func (a *App) UnstageAll() (*engine.RepoSnapshot, error) {
	return a.engine.UnstageAll()
}

func (a *App) DiscardFile(path string) (*engine.RepoSnapshot, error) {
	return a.engine.DiscardFile(path)
}

func (a *App) Commit(summary string, description string) (*engine.RepoSnapshot, error) {
	return a.engine.Commit(summary, description)
}

func (a *App) CheckoutBranch(name string) (*engine.RepoSnapshot, error) {
	return a.engine.CheckoutBranch(name)
}

func (a *App) CreateBranch(name string) (*engine.RepoSnapshot, error) {
	return a.engine.CreateBranch(name)
}

func (a *App) Fetch() (*engine.RepoSnapshot, error) {
	return a.engine.Fetch(a.progressEmitter())
}

func (a *App) Pull() (*engine.RepoSnapshot, error) {
	return a.engine.Pull(a.progressEmitter())
}

func (a *App) Push() (*engine.RepoSnapshot, error) {
	return a.engine.Push(a.progressEmitter())
}

// ---------------------------------------------------------------------------
// 仓库的建立：新建 / 克隆
// ---------------------------------------------------------------------------

// PickDirectory 弹出系统目录选择框。title 用于区分不同用途的对话框。
func (a *App) PickDirectory(title string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("应用尚未就绪")
	}
	if title == "" {
		title = "选择目录"
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

// DefaultBaseDir 返回「存放位置」输入框的默认值（通常是用户主目录）。
func (a *App) DefaultBaseDir() string {
	return engine.DefaultBaseDir()
}

// DeriveRepoName 从仓库地址推断目录名，供「下载仓库」表单预填。
func (a *App) DeriveRepoName(url string) string {
	return engine.DeriveRepoName(url)
}

// CreateRepo 新建一个仓库并打开。
func (a *App) CreateRepo(parentDir string, name string, initialBranch string) (*engine.RepoSnapshot, error) {
	return a.engine.InitRepo(parentDir, name, initialBranch)
}

// CloneRepo 克隆远端仓库并打开。
//
// 克隆期间会通过 "clone:progress" 事件把进度推给前端，
// 前端用 window.runtime.EventsOn("clone:progress", ...) 接收。
func (a *App) CloneRepo(url string, dest string, depth int) (*engine.RepoSnapshot, error) {
	return a.engine.CloneRepo(url, dest, depth, func(p engine.CloneProgress) {
		if a.ctx != nil {
			// Wails 的事件发送是并发安全的，可以从 clone 的读取 goroutine 里调用
			runtime.EventsEmit(a.ctx, "clone:progress", p)
		}
	})
}

// JoinPath 让前端不必自己处理路径分隔符（Windows / Linux 差异）。
func (a *App) JoinPath(dir string, name string) string {
	return filepath.Join(dir, name)
}

// ---------------------------------------------------------------------------
// git 全命令：操作目录 + 通用执行器
// ---------------------------------------------------------------------------

// Operations 返回全部受支持的 git 操作，供前端渲染操作面板。
func (a *App) Operations() []engine.OperationSummary {
	return a.engine.Operations()
}

// RunOperation 执行目录中的一个操作。args 是表单里填的参数。
func (a *App) RunOperation(id string, args map[string]string) (*engine.RunResult, error) {
	return a.engine.RunOperation(id, args)
}

// RunRawGit 执行任意 git 命令（兜底入口）。
// command 是一行命令，可以带引号，也可以以 "git " 开头。
func (a *App) RunRawGit(command string) (*engine.RunResult, error) {
	args := engine.SplitCommandLine(command)
	if len(args) > 0 && args[0] == "git" {
		args = args[1:]
	}
	return a.engine.RunRawGit(args)
}

// ---------------------------------------------------------------------------
// 提交身份
// ---------------------------------------------------------------------------

// Identity 返回当前生效的 user.name / user.email。
func (a *App) Identity() (string, string) {
	return a.engine.Identity()
}

// SetIdentity 设置提交身份并返回新快照。
// global=true 写入全局配置（对所有仓库生效）。
func (a *App) SetIdentity(name string, email string, global bool) (*engine.RepoSnapshot, error) {
	return a.engine.SetIdentity(name, email, global)
}

// ---------------------------------------------------------------------------
// 按文件查看某个提交
// ---------------------------------------------------------------------------

// CommitFiles 列出某个提交改动的文件。
func (a *App) CommitFiles(hash string) ([]engine.CommitFileDTO, error) {
	return a.engine.CommitFiles(hash)
}

// CommitFileDiff 取某个提交里单个文件的 diff。
func (a *App) CommitFileDiff(hash string, path string) (string, error) {
	return a.engine.CommitFileDiff(hash, path)
}

// ---------------------------------------------------------------------------
// 上传到 GitHub / Gitee
// ---------------------------------------------------------------------------

// Publish 把当前仓库上传到托管平台（创建远端仓库 + 推送）。
//
// 过程中会通过 "publish:progress" 事件推送当前步骤，前端可以显示进度。
func (a *App) Publish(
	platform string,
	mode string,
	token string,
	name string,
	description string,
	repoURL string,
	private bool,
	storeToken bool,
) (*engine.PublishResult, error) {
	return a.engine.Publish(
		engine.PublishRequest{
			Platform:    platform,
			Mode:        mode,
			RepoURL:     repoURL,
			Token:       token,
			Name:        name,
			Description: description,
			Private:     private,
			StoreToken:  storeToken,
		},
		func(step string) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "publish:progress", step)
			}
		},
	)
}

// ---------------------------------------------------------------------------
// 行级暂存 / 冲突解决 / stash
// ---------------------------------------------------------------------------

// FilePatchLines 取某个文件 diff 的结构化行，供界面勾选暂存。
func (a *App) FilePatchLines(path string, staged bool) (*engine.FilePatch, error) {
	return a.engine.FilePatchLines(path, staged)
}

// StageLines 暂存或取消暂存选中的行。
func (a *App) StageLines(path string, staged bool, lineIndices []int) (*engine.RepoSnapshot, error) {
	return a.engine.StageLines(path, staged, lineIndices)
}

// ReadConflictFile 解析一个冲突文件。
func (a *App) ReadConflictFile(path string) (*engine.ConflictFile, error) {
	return a.engine.ReadConflictFile(path)
}

// ResolveConflicts 按选择解决冲突。
func (a *App) ResolveConflicts(path string, choices []engine.ConflictChoice) (*engine.RepoSnapshot, error) {
	return a.engine.ResolveConflicts(path, choices)
}

// Stashes 列出储藏记录。
func (a *App) Stashes() ([]engine.StashEntryDTO, error) {
	return a.engine.Stashes()
}

// StashShow 预览某条储藏。
func (a *App) StashShow(index int) (string, error) {
	return a.engine.StashShow(index)
}

// StashSave 储藏当前改动。
func (a *App) StashSave(message string, includeUntracked bool) (*engine.RepoSnapshot, error) {
	return a.engine.StashSave(message, includeUntracked)
}

// StashPop / StashApply / StashDrop 操作某条储藏。
func (a *App) StashPop(index int) (*engine.RepoSnapshot, error) {
	return a.engine.StashPop(index)
}

func (a *App) StashApply(index int) (*engine.RepoSnapshot, error) {
	return a.engine.StashApply(index)
}

func (a *App) StashDrop(index int) (*engine.RepoSnapshot, error) {
	return a.engine.StashDrop(index)
}

// ---------------------------------------------------------------------------
// 拖拽打开仓库
// ---------------------------------------------------------------------------

// resolveDroppedRepo 把一个拖进来的路径解析成仓库根目录。
//
// 拖进来的可能是：仓库目录本身、仓库里的某个子目录、或者某个文件。
// 这三种情况都应该打开同一个仓库，所以这里一路往上找 .git。
func resolveDroppedRepo(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("路径不存在")
	}

	dir := abs
	if !info.IsDir() {
		dir = filepath.Dir(abs)
	}

	// 往上找，最多找到文件系统根
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 没找到 .git：如果拖进来的本身就是个目录，交给上层去报错
	if info.IsDir() {
		return abs, nil
	}
	return "", fmt.Errorf("这里不是 git 仓库")
}

// ---------------------------------------------------------------------------
// 仓库文件树
// ---------------------------------------------------------------------------

// RepoFiles 列出仓库里的所有文件（已跟踪 + 未忽略的未跟踪文件）。
func (a *App) RepoFiles() ([]engine.RepoFileDTO, error) {
	return a.engine.RepoFiles()
}

// FileContent 读取文件内容，用于界面上的只读浏览。
func (a *App) FileContent(path string) (*engine.FileContentDTO, error) {
	return a.engine.FileContent(path)
}

// PathInfo 返回拖拽提示用不到的额外信息（保留给以后扩展）。
func (a *App) IsGitRepo(path string) bool {
	p, err := resolveDroppedRepo(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(p, ".git"))
	return err == nil
}

// ---------------------------------------------------------------------------
// 分支
// ---------------------------------------------------------------------------

// CreateBranchFrom 创建分支。
// start 为空表示以当前 HEAD 为起点；checkout 为真时创建后立即切换。
func (a *App) CreateBranchFrom(name string, start string, checkout bool) (*engine.RepoSnapshot, error) {
	return a.engine.CreateBranchFrom(name, start, checkout)
}

// DeleteBranch 删除本地分支。
func (a *App) DeleteBranch(name string, force bool) (*engine.RepoSnapshot, error) {
	return a.engine.DeleteBranch(name, force)
}

// PushSetUpstream 首次推送：把当前分支推上去并设置上游。
func (a *App) PushSetUpstream(remote string) (*engine.RepoSnapshot, error) {
	return a.engine.PushSetUpstream(remote, a.progressEmitter())
}

// progressEmitter 把引擎的进度回调转成 Wails 事件推给前端。
//
// 事件名沿用克隆那套（同为「远端操作进度」），前端订阅一次即可。
func (a *App) progressEmitter() func(engine.SyncProgress) {
	return func(p engine.SyncProgress) {
		if a.ctx == nil {
			return
		}
		runtime.EventsEmit(a.ctx, "sync:progress", p)
	}
}

// OperationChoices 返回操作面板里所有下拉的候选值。
func (a *App) OperationChoices() (*engine.OperationChoices, error) {
	return a.engine.OperationChoices()
}

// PublishDefaults 返回上传对话框可以预填的信息（已有远端地址等）。
func (a *App) PublishDefaults() (*engine.PublishDefaults, error) {
	return a.engine.PublishDefaults()
}

// ---------------------------------------------------------------------------
// 窗口控制（自绘标题栏用）
// ---------------------------------------------------------------------------

// MinimiseWindow 最小化窗口。
func (a *App) MinimiseWindow() {
	if a.ctx != nil {
		runtime.WindowMinimise(a.ctx)
	}
}

// ToggleMaximiseWindow 在最大化和还原之间切换。
func (a *App) ToggleMaximiseWindow() {
	if a.ctx != nil {
		runtime.WindowToggleMaximise(a.ctx)
	}
}

// IsWindowMaximised 返回窗口当前是否最大化（自绘标题栏用它切换按钮图标）。
func (a *App) IsWindowMaximised() bool {
	if a.ctx == nil {
		return false
	}
	return runtime.WindowIsMaximised(a.ctx)
}

// CloseWindow 关闭应用。
func (a *App) CloseWindow() {
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// ---------------------------------------------------------------------------
// 打开一个「可能还不是仓库」的文件夹
// ---------------------------------------------------------------------------

// InspectFolder 检查文件夹状态，供前端决定下一步：
// 直接打开 / 打开上级仓库 / 询问是否初始化。
func (a *App) InspectFolder(path string) (*engine.FolderInfo, error) {
	return a.engine.InspectFolder(path)
}

// InitRepoHere 在指定目录里 git init 并打开。
func (a *App) InitRepoHere(path string, initialBranch string) (*engine.RepoSnapshot, error) {
	return a.engine.InitRepoAt(path, initialBranch)
}

// PendingStartupFolder 返回「启动目录不是 git 仓库」时的目录信息。
// 启动目录本来就是仓库时返回 nil。
//
// 用途：用户 `cd 我的项目 && bingit` 时，可以直接问他要不要初始化，
// 不用他再去菜单里点「打开仓库」。
func (a *App) PendingStartupFolder() (*engine.FolderInfo, error) {
	if a.startupNotRepo == "" {
		return nil, nil
	}
	return a.engine.InspectFolder(a.startupNotRepo)
}
