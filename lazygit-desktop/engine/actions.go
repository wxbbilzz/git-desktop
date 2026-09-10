package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gocui"
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
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("分支名不能为空")
	}
	// New() 默认会 checkout；base 传空串表示以当前 HEAD 为起点。
	if err := e.git.Branch.New(name, ""); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// ---------------------------------------------------------------------------
// 同步（远端）
// ---------------------------------------------------------------------------

// syncOp 收敛了引擎里唯一一处对 lazygit TUI 的依赖：长任务需要 gocui.Task。
//
// 引擎没有 UI 任务系统，而 gocui.Task 又带未导出方法、外部无法自己实现，
// 于是这里借 gocui.NewFakeTask() 顶替 —— 它只在 lazygit 内部用于判断
// “程序是否正忙”，对我们的用途没有副作用。
//
// 想彻底去掉这个依赖，只要把 git_commands 中 Push/Fetch/Pull 的
// task 参数改成一个自定义的窄接口即可（这是抽引擎阶段顺手就能做的清理）。
func (e *Engine) syncOp(fn func(task gocui.Task) error) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := fn(gocui.NewFakeTask()); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// Fetch 拉取远端引用（对应 `git fetch [--all] --no-write-fetch-head`）。
func (e *Engine) Fetch() (*RepoSnapshot, error) {
	return e.syncOp(func(task gocui.Task) error {
		return e.git.Sync.Fetch(task)
	})
}

// Pull 拉取并合并当前分支（`git pull --no-edit`）。
func (e *Engine) Pull() (*RepoSnapshot, error) {
	return e.syncOp(func(task gocui.Task) error {
		return e.git.Sync.Pull(task, git_commands.PullOptions{})
	})
}

// Push 推送当前分支到它的上游（`git push`）。
// 若分支还没有上游，git 会报错，前端应给出「首次推送」的提示。
func (e *Engine) Push() (*RepoSnapshot, error) {
	return e.syncOp(func(task gocui.Task) error {
		return e.git.Sync.Push(task, git_commands.PushOpts{})
	})
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
