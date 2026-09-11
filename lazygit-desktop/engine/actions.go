package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

// 本文件是引擎的“动作层”：UI 发来的每一个意图都在这里落成一次 git 操作，
// 操作完成后统一返回一份新快照，前端据此重绘。
//
// 之所以每次动作都回一份完整快照，是为了让 UI 保持无状态 —— 前端永远不需要
// 自己推算 git 的下一步状态，这也正好对应 lazygit 内部“动作结束就 refresh”的模式。

func (e *Engine) requireRepoLocked() error {
	if e.git == nil {
		return fmt.Errorf("尚未打开仓库")
	}
	return nil
}

func (e *Engine) findFileLocked(path string) (*models.File, error) {
	for _, f := range e.git.Loaders.FileLoader.GetStatusFiles(git_commands.GetStatusFileOptions{}) {
		if f.Path == path {
			return f, nil
		}
	}
	return nil, fmt.Errorf("文件不在当前改动列表中: %s", path)
}

// ---------------------------------------------------------------------------
// 暂存区
// ---------------------------------------------------------------------------

// StageFile 暂存单个文件（等价于 `git add -- <path>`）。
func (e *Engine) StageFile(path string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := e.git.WorkingTree.StageFile(path); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// UnstageFile 取消暂存单个文件。
// 已跟踪文件走 `git reset HEAD --`，未跟踪文件走 `git rm --cached`，
// lazygit 的 UnStageFile 已经帮我们分好了这两种情况。
func (e *Engine) UnstageFile(path string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	file, err := e.findFileLocked(path)
	if err != nil {
		return nil, err
	}
	// 重命名要同时带上新旧两个路径，git 才认得出。
	if err := e.git.WorkingTree.UnStageFile(file.Names(), file.Tracked); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// StageAll 暂存全部改动（含未跟踪文件，对应 `git add -A`）。
func (e *Engine) StageAll() (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := e.git.WorkingTree.StageAll(false); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// UnstageAll 取消全部暂存（对应 `git reset`）。
func (e *Engine) UnstageAll() (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := e.git.WorkingTree.UnstageAll(); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// DiscardFile 丢弃工作区改动。
//   - 已跟踪文件：`git checkout -- <path>`
//   - 未跟踪文件：直接删除磁盘上的文件
//
// 这是个不可逆操作，前端务必先弹确认框。
func (e *Engine) DiscardFile(path string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	file, err := e.findFileLocked(path)
	if err != nil {
		return nil, err
	}

	if file.Tracked {
		if err := e.git.WorkingTree.DiscardUnstagedFileChanges(file); err != nil {
			return nil, err
		}
	} else {
		if err := e.removeUntrackedFile(path); err != nil {
			return nil, err
		}
	}

	return e.snapshotLocked()
}

// removeUntrackedFile 删除未跟踪文件，并防止路径穿越到仓库之外。
func (e *Engine) removeUntrackedFile(path string) error {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return fmt.Errorf("非法文件路径: %s", path)
	}
	full := filepath.Join(e.repoPath, clean)
	return os.Remove(full)
}

// ---------------------------------------------------------------------------
// 提交
// ---------------------------------------------------------------------------

// Commit 用给定的标题 / 描述直接提交。
//
// 这里就是重写 UI 后最舒服的一处：桌面端有真正的输入框，所以直接把消息
// 通过 `-m` 传给 git 即可，完全不需要 lazygit 那套“把自己伪装成 GIT_EDITOR
// 的 daemon 进程”来绕过终端编辑器的限制。
func (e *Engine) Commit(summary, description string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(summary) == "" {
		return nil, fmt.Errorf("提交信息不能为空")
	}
	// 兜底：身份没配好时 git 只会丢一句英文 fatal，这里换成可操作的提示
	if name, email := e.identityLocked(); name == "" || email == "" {
		return nil, fmt.Errorf("还没有配置提交身份（user.name / user.email），请在提交面板里填写后重试")
	}
	if err := e.git.Commit.CommitCmdObj(summary, description, false).Run(); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// ---------------------------------------------------------------------------
// 分支
// ---------------------------------------------------------------------------

// CheckoutBranch 切换分支。注意：工作区有冲突或会覆盖改动时 git 会拒绝，属正常行为。
func (e *Engine) CheckoutBranch(name string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := e.git.Branch.Checkout(name, git_commands.CheckoutOptions{}); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// CreateBranch 基于当前 HEAD 建一个新分支并切过去。
func (e *Engine) CreateBranch(name string) (*RepoSnapshot, error) {
	return e.CreateBranchFrom(name, "", true)
}

// CreateBranchFrom 从指定起点创建分支。
//
//	start 为空      -> 以当前 HEAD 为起点
//	checkout = true -> 创建后立即切过去（等价 git checkout -b）
//	checkout = false-> 只创建不切换（等价 git branch）
//
// 注意：不能直接调 lazygit 的 Branch.New(name, start) —— 它内部用的是
// Arg()，而 Arg 不会跳过空串，start 为空时会生成 `git checkout -b 名字 ""`，
// git 会报 "empty string is not a valid pathspec"。所以这里自己拼参数。
func (e *Engine) CreateBranchFrom(name, start string, checkout bool) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	name = strings.TrimSpace(name)
	start = strings.TrimSpace(start)
	if name == "" {
		return nil, fmt.Errorf("分支名不能为空")
	}
	if strings.ContainsAny(name, " \t~^:?*[\\") {
		return nil, fmt.Errorf("分支名里有非法字符（不能包含空格、~^:?*[ 等）")
	}

	var args []string
	if checkout {
		args = append(args, "checkout", "-b", name)
	} else {
		args = append(args, "branch", name)
	}
	if start != "" {
		args = append(args, start)
	}

	if out, err := e.gitRun(args...); err != nil {
		return nil, fmt.Errorf("%s", firstErrorLine(out))
	}
	return e.snapshotLocked()
}

// DeleteBranch 删除本地分支。
func (e *Engine) DeleteBranch(name string, force bool) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("分支名不能为空")
	}

	flag := "-d"
	if force {
		flag = "-D"
	}
	if out, err := e.gitRun("branch", flag, name); err != nil {
		return nil, fmt.Errorf("%s", firstErrorLine(out))
	}
	return e.snapshotLocked()
}

// ---------------------------------------------------------------------------
// 同步（远端）
// ---------------------------------------------------------------------------

// syncOp 把一次远端操作包起来：加锁 → 执行 → 刷新快照。
//
// 为什么不用 lazygit 的 SyncCommands？
//
// 它的 Fetch/Pull/Push 都调用了 PromptOnCredentialRequest(task)，那会把命令
// **放进 PTY 执行**。git 因此认为自己在真终端里，遇到需要账号密码时会弹出
// 提示并无限等待 —— 在没有终端的桌面应用里就是永久挂起（界面卡在"正在…"
// 且按钮全部禁用）。
//
// 这里用自己的 gitRun：不开 PTY，并显式设 GIT_TERMINAL_PROMPT=0，
// 让需要凭据的远端立刻失败并给出可读的错误，而不是把界面拖死。
func (e *Engine) syncOp(args []string, env ...string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	if out, err := e.gitRunWith(append(env, "GIT_TERMINAL_PROMPT=0"), args...); err != nil {
		msg := firstErrorLine(out)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return e.snapshotLocked()
}

// Fetch 拉取远端引用（不合并到本地分支）。
func (e *Engine) Fetch() (*RepoSnapshot, error) {
	args := []string{"fetch"}
	if e.cmn.UserConfig().Git.FetchAll {
		args = append(args, "--all")
	}
	// 不写 FETCH_HEAD，避免和并行的 pull 打架
	args = append(args, "--no-write-fetch-head")
	return e.syncOp(args)
}

// Pull 拉取并合并当前分支。
//
// GIT_SEQUENCE_EDITOR=: 用来兜底：万一用户配了 pull.rebase=interactive，
// 也能跳过交互式编辑。
func (e *Engine) Pull() (*RepoSnapshot, error) {
	return e.syncOp([]string{"pull", "--no-edit"}, "GIT_SEQUENCE_EDITOR=:")
}

// Push 推送当前分支到它的上游。
func (e *Engine) Push() (*RepoSnapshot, error) {
	return e.syncOp([]string{"push"})
}

// PushSetUpstream 首次推送：把当前分支推上去并设置上游。
func (e *Engine) PushSetUpstream(remote string) (*RepoSnapshot, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = "origin"
	}
	return e.syncOp([]string{"push", "--set-upstream", remote, "HEAD"})
}

// ---------------------------------------------------------------------------
// 只读：取 diff
// ---------------------------------------------------------------------------

// FileDiff 取某个文件的 diff 文本。
//
//   - staged=true  → 暂存区 vs HEAD（`git diff --cached`）
//   - staged=false → 工作区 vs 暂存区（`git diff`）
//
// plain=true 表示不要 git 的 ANSI 颜色，由前端自己按行着色。
func (e *Engine) FileDiff(path string, staged bool) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return "", err
	}
	file, err := e.findFileLocked(path)
	if err != nil {
		return "", err
	}

	// 注意：WorktreeFileDiff 内部把错误当作“文件已删除”处理，不会返回 error。
	return e.git.WorkingTree.WorktreeFileDiff(file, true, staged), nil
}

// CommitDiff 取某次提交的完整 diff。
func (e *Engine) CommitDiff(hash string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return "", err
	}
	return e.git.Commit.GetCommitDiff(hash)
}
