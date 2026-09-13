package engine

import (
	"fmt"
	"strings"
)

// 本文件把「高频 git 操作」从通用命令行目录里提出来，做成引擎的一等方法。
//
// 为什么不让前端直接去调 RunOperation？因为那些是「填表单执行命令」的入口，
// 参数是用户手填的字符串，界面拿不到上下文（这是哪个分支、点的是哪个提交）。
// 而界面上真正高频的动作 —— 把某个分支合并进来、拣选这次提交、回到这里 ——
// 参数都来自用户点的那一行，应该由界面直接传结构化参数过来。
//
// 所有方法都遵守同一条约定：加锁 → 校验 → 执行 → 返回新快照。
// 唯一例外是涉及网络的（推送标签），走 syncOp 的「网络期间不持锁」路径。

// ---------------------------------------------------------------------------
// 合并 / 变基
// ---------------------------------------------------------------------------

// MergeBranch 把 name 合并进当前分支。
//
// noFF=true 强制生成合并提交（--no-ff），squash=true 只把改动放进工作区不提交。
func (e *Engine) MergeBranch(name string, noFF, squash bool) (*RepoSnapshot, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("请选择要合并的分支")
	}

	args := []string{"merge", "--no-edit"}
	if noFF {
		args = append(args, "--no-ff")
	}
	if squash {
		args = append(args, "--squash")
	}
	args = append(args, name)

	return e.runOpLocked(args, nil,
		fmt.Sprintf("把 %s 合并进当前分支", name))
}

// RebaseOnto 把当前分支变基到 onto 之上。
//
// 冲突时 git 会停下来并保留 rebase 状态，快照里 State 会变成「rebase 中」，
// 界面据此显示「继续 / 中止」按钮。
func (e *Engine) RebaseOnto(onto string) (*RepoSnapshot, error) {
	onto = strings.TrimSpace(onto)
	if onto == "" {
		return nil, fmt.Errorf("请选择变基的目标分支")
	}
	// GIT_SEQUENCE_EDITOR / GIT_EDITOR 兜底：用户配过 pull.rebase=interactive
	// 之类的话，不能让 git 停在那里等编辑器
	env := []string{"GIT_SEQUENCE_EDITOR=:", "GIT_EDITOR=true"}
	return e.runOpLocked([]string{"rebase", onto}, env,
		fmt.Sprintf("变基到 %s", onto))
}

// ---------------------------------------------------------------------------
// 拣选 / 撤销某次提交
// ---------------------------------------------------------------------------

// CherryPick 把某次提交应用到当前分支。
func (e *Engine) CherryPick(hash string) (*RepoSnapshot, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, fmt.Errorf("请选择要拣选的提交")
	}
	return e.runOpLocked([]string{"cherry-pick", "--no-edit", hash}, nil,
		fmt.Sprintf("拣选提交 %s", shortHash(hash)))
}

// RevertCommit 生成一个「反向提交」来抵消某次提交的改动。
//
// 和 reset 的区别：revert 不改历史，适合已经推送出去的提交。
func (e *Engine) RevertCommit(hash string) (*RepoSnapshot, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, fmt.Errorf("请选择要撤销的提交")
	}
	return e.runOpLocked([]string{"revert", "--no-edit", hash}, nil,
		fmt.Sprintf("撤销提交 %s（生成反向提交）", shortHash(hash)))
}

// ---------------------------------------------------------------------------
// 回退到某个提交
// ---------------------------------------------------------------------------

// ResetTo 把当前分支回退到 commit。
//
//	mode=soft   改动留在暂存区（最安全，只移动分支指针）
//	mode=mixed  改动留在工作区（取消暂存）
//	mode=hard   连工作区一起丢弃 —— 会真的丢文件，所以先记一个回收点
//
// 无论哪种模式，git 都会把原来的 HEAD 写进 ORIG_HEAD，所以「撤销上一步」
// 都能把这次回退撤回来。
func (e *Engine) ResetTo(commit, mode string) (*RepoSnapshot, error) {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return nil, fmt.Errorf("请选择要回退到的提交")
	}

	switch mode {
	case "soft", "mixed", "hard":
	default:
		return nil, fmt.Errorf("不支持的回退模式: %s", mode)
	}

	return e.runOpLocked([]string{"reset", "--" + mode, commit}, nil,
		fmt.Sprintf("回退到 %s（--%s）", shortHash(commit), mode))
}

// ---------------------------------------------------------------------------
// 修补最后一次提交
// ---------------------------------------------------------------------------

// AmendCommit 修补最后一次提交。
//
// summary 为空时表示「只改内容，不动提交信息」（--amend --no-edit）——
// 这是最常见的用法：忘了加一个文件，补进去。
//
// 不用 lazygit 的 CommitCmdObj：它的第三个参数是 forceSkipHooks 而不是
// amend，很容易传错（传错的表现是 git 报「无文件要提交」）。这里自己拼参数，
// 意图写在脸上。
func (e *Engine) AmendCommit(summary, description string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	sum := strings.TrimSpace(summary)
	desc := strings.TrimSpace(description)
	if sum == "" && desc != "" {
		return nil, fmt.Errorf("只填了详细描述，请把标题也填上")
	}

	// --allow-empty：只改提交信息（不改内容）也是合法用法
	args := []string{"commit", "--amend", "--allow-empty"}
	if sum == "" {
		args = append(args, "--no-edit")
	} else {
		if name, email := e.identityLocked(); name == "" || email == "" {
			return nil, fmt.Errorf("还没有配置提交身份（user.name / user.email）")
		}
		args = append(args, "-m", sum)
		if desc != "" {
			args = append(args, "-m", desc)
		}
	}

	if out, err := e.gitRun(args...); err != nil {
		msg := firstErrorLine(out)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("修补提交失败：%s", msg)
	}
	return e.snapshotLocked()
}

// ---------------------------------------------------------------------------
// 中断的操作：继续 / 中止
// ---------------------------------------------------------------------------

// ContinueOperation 继续被冲突中断的操作（变基 / 拣选 / revert）。
// 合并没有 continue —— 解决冲突后直接提交即可。
func (e *Engine) ContinueOperation() (*RepoSnapshot, error) {
	e.mu.Lock()
	state := e.git.Status.WorkingTreeState()
	e.mu.Unlock()

	var args []string
	switch {
	case state.Rebasing:
		args = []string{"rebase", "--continue"}
	case state.CherryPicking:
		args = []string{"cherry-pick", "--continue"}
	case state.Reverting:
		args = []string{"revert", "--continue"}
	default:
		return nil, fmt.Errorf("当前没有需要继续的操作")
	}

	return e.runOpLocked(args, []string{"GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=:"}, "继续操作")
}

// AbortOperation 中止当前被中断的操作，恢复到它开始之前的状态。
func (e *Engine) AbortOperation() (*RepoSnapshot, error) {
	e.mu.Lock()
	state := e.git.Status.WorkingTreeState()
	e.mu.Unlock()

	var args []string
	switch {
	case state.Rebasing:
		args = []string{"rebase", "--abort"}
	case state.Merging:
		args = []string{"merge", "--abort"}
	case state.CherryPicking:
		args = []string{"cherry-pick", "--abort"}
	case state.Reverting:
		args = []string{"revert", "--abort"}
	default:
		return nil, fmt.Errorf("当前没有可中止的操作")
	}

	return e.runOpLocked(args, nil, "中止操作")
}

// ---------------------------------------------------------------------------
// 标签
// ---------------------------------------------------------------------------

// CreateTag 创建标签。
//
//	message 非空 -> 附注标签（git tag -a），带说明和创建者信息
//	message 为空 -> 轻量标签（git tag），只是一个指向提交的名字
//	ref 为空     -> 打在 HEAD 上
func (e *Engine) CreateTag(name, ref, message string) (*RepoSnapshot, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("标签名不能为空")
	}
	if strings.ContainsAny(name, " \t~^:?*[\\") {
		return nil, fmt.Errorf("标签名里有非法字符（不能包含空格、~^:?*[ 等）")
	}

	ref = strings.TrimSpace(ref)
	message = strings.TrimSpace(message)

	var args []string
	if message != "" {
		args = []string{"tag", "-a", name, "-m", message}
	} else {
		args = []string{"tag", name}
	}
	if ref != "" {
		args = append(args, ref)
	}

	return e.runOpLocked(args, nil, "创建标签 "+name)
}

// DeleteTag 删除本地标签。
func (e *Engine) DeleteTag(name string) (*RepoSnapshot, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("标签名不能为空")
	}
	return e.runOpLocked([]string{"tag", "-d", name}, nil, "删除标签 "+name)
}

// PushTag 把某个标签推到远端（网络操作，不持锁）。
func (e *Engine) PushTag(name, remote string, onProgress func(SyncProgress)) (*RepoSnapshot, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("标签名不能为空")
	}
	remote = defaultRemote(remote)
	return e.syncOp([]string{"push", "--progress", remote, "refs/tags/" + name}, nil, onProgress)
}

// PushAllTags 推送所有本地标签。
func (e *Engine) PushAllTags(remote string, onProgress func(SyncProgress)) (*RepoSnapshot, error) {
	return e.syncOp([]string{"push", "--progress", defaultRemote(remote), "--tags"}, nil, onProgress)
}

func defaultRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "origin"
	}
	return remote
}

// ---------------------------------------------------------------------------
// 远端
// ---------------------------------------------------------------------------

// AddRemote 添加一个远端。
func (e *Engine) AddRemote(name, url string) (*RepoSnapshot, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return nil, fmt.Errorf("远端名不能为空")
	}
	if url == "" {
		return nil, fmt.Errorf("远端地址不能为空")
	}
	return e.runOpLocked([]string{"remote", "add", name, url}, nil, "添加远端 "+name)
}

// RemoveRemote 删除一个远端（同时删掉它在本地的远端分支引用）。
func (e *Engine) RemoveRemote(name string) (*RepoSnapshot, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("远端名不能为空")
	}
	return e.runOpLocked([]string{"remote", "remove", name}, nil, "删除远端 "+name)
}

// SetRemoteURL 修改远端地址。
//
// 传进来的地址里如果带着凭据会被原样写入 config —— 这是用户自己的选择，
// 但界面上显示的时候会剥掉，见 loadRemotesLocked。
func (e *Engine) SetRemoteURL(name, url string) (*RepoSnapshot, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" || url == "" {
		return nil, fmt.Errorf("远端名和地址都不能为空")
	}
	return e.runOpLocked([]string{"remote", "set-url", name, url}, nil, "修改远端地址")
}

// ---------------------------------------------------------------------------
// 远端分支
// ---------------------------------------------------------------------------

// CheckoutRemoteBranch 以某个远端分支为起点创建本地分支并切过去。
//
// local 为空时用远端分支的短名（origin/feature -> feature）。
func (e *Engine) CheckoutRemoteBranch(remoteBranch, local string) (*RepoSnapshot, error) {
	remoteBranch = strings.TrimSpace(remoteBranch)
	if remoteBranch == "" {
		return nil, fmt.Errorf("请选择远端分支")
	}
	local = strings.TrimSpace(local)
	if local == "" {
		_, local = splitRemoteBranch(remoteBranch)
	}
	if local == "" {
		return nil, fmt.Errorf("无法从 %s 推断本地分支名", remoteBranch)
	}

	return e.runOpLocked(
		[]string{"checkout", "-b", local, "--track", remoteBranch}, nil,
		fmt.Sprintf("从 %s 检出分支 %s", remoteBranch, local))
}

// UpdateRemoteBranch 把一个远端分支的更新拉到本地（快进）。
// 用于「本地分支落后于它的上游」的场景。
func (e *Engine) FastForwardBranch(remoteBranch string) (*RepoSnapshot, error) {
	remoteBranch = strings.TrimSpace(remoteBranch)
	if remoteBranch == "" {
		return nil, fmt.Errorf("请选择远端分支")
	}
	return e.runOpLocked([]string{"merge", "--ff-only", remoteBranch}, nil,
		fmt.Sprintf("快进到 %s", remoteBranch))
}

// ---------------------------------------------------------------------------
// 撤销上一步
// ---------------------------------------------------------------------------

// UndoLast 撤销最近一次改动 HEAD 的操作。
//
// 实现方式是 `git reset --soft HEAD@{1}`：只把分支指针挪回上一个位置，
// 完全不动工作区和暂存区 —— 也就是说**任何文件内容都不会丢**。
// 提交被撤销后改动会回到暂存区，合并/变基被撤销后提交回到原来的位置。
//
// 两个刻意的限制：
//
//   - 处于 rebase / merge 中断状态时拒绝执行，让用户先去「继续 / 中止」，
//     避免在中间状态上叠一次 reset 把事情搞乱。
//   - 只覆盖「移动过 HEAD 的操作」。丢弃文件改动不在此列 —— 那种情况下
//     内容已经不在 git 对象库里了，reset 救不回来；它由回收站（restore.go）负责。
func (e *Engine) UndoLast() (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	state := e.git.Status.WorkingTreeState()
	if state.Rebasing || state.Merging || state.CherryPicking || state.Reverting {
		return nil, fmt.Errorf("当前有未完成的 %s，请先「继续」或「中止」后再撤销", stateLabel(state))
	}

	entries, err := e.reflogLocked(2)
	if err != nil || len(entries) < 2 {
		return nil, fmt.Errorf("没有可撤销的操作")
	}

	if _, err := e.gitRun("reset", "--soft", "HEAD@{1}"); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// ReflogEntryDTO 是 reflog 里的一条记录，用来告诉用户「撤销会撤掉什么」。
type ReflogEntryDTO struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
}

// reflogLocked 读最近 n 条 reflog。
func (e *Engine) reflogLocked(n int) ([]ReflogEntryDTO, error) {
	out, err := e.gitOutput("reflog", "--format=%H%x00%gs", "-n", fmt.Sprintf("%d", n))
	if err != nil {
		return nil, err
	}

	entries := []ReflogEntryDTO{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.SplitN(line, "\x00", 2)
		subject := ""
		if len(f) > 1 {
			subject = f[1]
		}
		entries = append(entries, ReflogEntryDTO{Hash: f[0], Subject: subject})
	}
	return entries, nil
}

// ---------------------------------------------------------------------------
// 统一执行入口
// ---------------------------------------------------------------------------

// runOpLocked 是上面这些操作的共同骨架：
// 加锁 → 确认仓库可用 → 跑 git → 回一份新快照。
func (e *Engine) runOpLocked(args []string, env []string, what string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	out, err := e.gitRunWith(env, args...)
	if err != nil {
		msg := firstErrorLine(out)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s失败：%s", what, msg)
	}
	return e.snapshotLocked()
}
