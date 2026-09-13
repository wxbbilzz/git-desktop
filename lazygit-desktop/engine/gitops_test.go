package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 本文件验证「从命令行目录提出来的一等方法」和自动刷新确实能工作。
//
// 这些能力此前只能靠用户在「Git 操作」面板里填表单执行命令，
// 现在界面上的每一行都能直接触发，所以每一条都要有测试兜住。

// commitFile 在仓库里写一个文件并提交。
//
// 用 exec 直接提交而不是走 engine.Commit：后者要求 git 里配好身份，
// 而测试环境把全局配置指向了 /dev/null。
func commitFile(t *testing.T, dir, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-qm", message)
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.com")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("%v 失败: %s", args, out)
	}
	return string(out)
}

// openEngineWithIdentity 打开仓库，并把提交身份喂给 git。
//
// 两条路都要走：
//   - 环境变量：给 merge / cherry-pick / revert 这些「git 自己造提交」的操作用
//   - 仓库本地 config：引擎在提交前会检查身份是否配好，那一步读的是 git config
//
// 必须在 OpenRepo 之前写好 —— 引擎会缓存 config 读取器。
func openEngineWithIdentity(t *testing.T, work string) *Engine {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@e.com")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@e.com")

	gitCmd(t, work, "config", "user.name", "t")
	gitCmd(t, work, "config", "user.email", "t@e.com")

	return openEngine(t, work)
}

// ---------------------------------------------------------------------------
// 标签 / 远端 / 远端分支
// ---------------------------------------------------------------------------

func TestSnapshotCarriesTagsRemotesAndRemoteBranches(t *testing.T) {
	work, bare := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	// 配一个远端并推上去，这样才有远端分支
	gitCmd(t, work, "remote", "add", "origin", bare)
	gitCmd(t, work, "push", "-q", "-u", "origin", "main")

	// 一个轻量标签 + 一个附注标签
	if _, err := e.CreateTag("v1.0", "", ""); err != nil {
		t.Fatalf("创建轻量标签失败: %v", err)
	}
	if _, err := e.CreateTag("v2.0", "", "第二个版本"); err != nil {
		t.Fatalf("创建附注标签失败: %v", err)
	}

	snap, err := e.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if len(snap.Tags) != 2 {
		t.Fatalf("应该有 2 个标签，实际 %d 个", len(snap.Tags))
	}
	var annotated, lightweight int
	for _, tag := range snap.Tags {
		if tag.IsAnnotated {
			annotated++
		} else {
			lightweight++
		}
		if tag.Hash == "" || tag.ShortHash == "" {
			t.Errorf("标签 %s 缺少提交哈希", tag.Name)
		}
	}
	if annotated != 1 || lightweight != 1 {
		t.Errorf("附注/轻量标签应该各 1 个，实际附注 %d 轻量 %d", annotated, lightweight)
	}

	if len(snap.Remotes) != 1 || snap.Remotes[0].Name != "origin" {
		t.Fatalf("远端读错了: %+v", snap.Remotes)
	}
	if snap.Remotes[0].URL != bare {
		t.Errorf("远端地址应该是 %s，实际 %s", bare, snap.Remotes[0].URL)
	}

	// 远端分支里要有 origin/main，并且它就是当前分支的上游
	var found bool
	for _, rb := range snap.RemoteBranches {
		if rb.Name == "origin/main" {
			found = true
			if rb.Remote != "origin" || rb.Short != "main" {
				t.Errorf("远端分支拆分错了: %+v", rb)
			}
			if !rb.IsCurrentUpstream {
				t.Error("origin/main 应该是当前分支的上游")
			}
		}
		if strings.HasSuffix(rb.Name, "/HEAD") {
			t.Errorf("origin/HEAD 这种符号引用不该出现在列表里: %s", rb.Name)
		}
	}
	if !found {
		t.Errorf("没读到 origin/main: %+v", snap.RemoteBranches)
	}
}

// 远端地址里的 token 绝不能出现在界面上。
func TestRemotesHideCredentials(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	gitCmd(t, work, "remote", "add", "origin",
		"https://oauth2:SECRET_TOKEN@example.com/user/repo.git")

	snap, err := e.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Remotes) != 1 {
		t.Fatalf("应该有 1 个远端，实际 %d", len(snap.Remotes))
	}
	if strings.Contains(snap.Remotes[0].URL, "SECRET_TOKEN") {
		t.Errorf("❌ 远端地址里的 token 泄露到界面了: %s", snap.Remotes[0].URL)
	}
	if snap.Remotes[0].URL != "https://example.com/user/repo.git" {
		t.Errorf("剥离凭据后的地址不对: %s", snap.Remotes[0].URL)
	}
}

func TestDeleteTag(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	if _, err := e.CreateTag("temp", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.DeleteTag("temp"); err != nil {
		t.Fatal(err)
	}
	snap, _ := e.Snapshot()
	if len(snap.Tags) != 0 {
		t.Errorf("标签应该已经被删掉，实际还剩 %d 个", len(snap.Tags))
	}
}

func TestRemoteManagement(t *testing.T) {
	work, bare := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	if _, err := e.AddRemote("upstream", bare); err != nil {
		t.Fatalf("添加远端失败: %v", err)
	}
	snap, _ := e.Snapshot()
	if len(snap.Remotes) != 1 || snap.Remotes[0].Name != "upstream" {
		t.Fatalf("添加远端后快照不对: %+v", snap.Remotes)
	}

	if _, err := e.SetRemoteURL("upstream", bare+"2"); err != nil {
		t.Fatalf("改地址失败: %v", err)
	}
	snap, _ = e.Snapshot()
	if snap.Remotes[0].URL != bare+"2" {
		t.Errorf("地址没改成功: %s", snap.Remotes[0].URL)
	}

	if _, err := e.RemoveRemote("upstream"); err != nil {
		t.Fatalf("删除远端失败: %v", err)
	}
	snap, _ = e.Snapshot()
	if len(snap.Remotes) != 0 {
		t.Errorf("远端应该已经删掉，实际还剩 %d 个", len(snap.Remotes))
	}
}

// ---------------------------------------------------------------------------
// 合并 / 拣选 / 回退 / 修补
// ---------------------------------------------------------------------------

func TestMergeBranch(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	// 起一个分支并在上面提交
	gitCmd(t, work, "checkout", "-q", "-b", "feature")
	commitFile(t, work, "feature.txt", "hello", "feature work")
	gitCmd(t, work, "checkout", "-q", "main")

	if _, err := e.MergeBranch("feature", false, false); err != nil {
		t.Fatalf("合并失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "feature.txt")); err != nil {
		t.Errorf("合并后文件应该出现在工作区: %v", err)
	}
}

// 合并冲突时不能抛异常，而要留下 rebase/merge 状态让界面引导用户处理。
func TestMergeConflictLeavesState(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	gitCmd(t, work, "checkout", "-q", "-b", "other")
	commitFile(t, work, "a.txt", "来自 other 的内容", "other change")
	gitCmd(t, work, "checkout", "-q", "main")
	commitFile(t, work, "a.txt", "来自 main 的内容", "main change")

	if _, err := e.MergeBranch("other", false, false); err == nil {
		t.Fatal("冲突时应该返回错误")
	}

	snap, err := e.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.State == "" {
		t.Error("冲突后快照里应该标注「合并中」，界面要据此给出继续/中止")
	}

	// 而且此时必须能中止，回到合并前的样子
	if _, err := e.AbortOperation(); err != nil {
		t.Fatalf("中止合并失败: %v", err)
	}
	snap, _ = e.Snapshot()
	if snap.State != "" {
		t.Errorf("中止后不该还有特殊状态: %s", snap.State)
	}
	content, _ := os.ReadFile(filepath.Join(work, "a.txt"))
	if !strings.Contains(string(content), "main") {
		t.Errorf("中止后应该恢复到合并前的内容，实际: %s", content)
	}
}

func TestCherryPickCopiesCommit(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	gitCmd(t, work, "checkout", "-q", "-b", "feature")
	commitFile(t, work, "pick.txt", "内容", "要拣选的提交")
	hash := strings.TrimSpace(gitCmd(t, work, "rev-parse", "HEAD"))
	gitCmd(t, work, "checkout", "-q", "main")

	// 现在把那次提交拣选到 main 上
	if _, err := e.CherryPick(hash); err != nil {
		t.Fatalf("拣选失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "pick.txt")); err != nil {
		t.Errorf("拣选后文件应该出现在 main 上: %v", err)
	}
}

func TestRevertCommitCreatesCounterCommit(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	commitFile(t, work, "temp.txt", "内容", "临时提交")
	hash := strings.TrimSpace(gitCmd(t, work, "rev-parse", "HEAD"))

	if _, err := e.RevertCommit(hash); err != nil {
		t.Fatalf("revert 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "temp.txt")); !os.IsNotExist(err) {
		t.Error("revert 之后文件应该被删掉")
	}

	// 而且确实是「多了一次提交」而不是改历史
	snap, _ := e.Snapshot()
	if len(snap.Commits) != 3 {
		t.Errorf("应该有 3 次提交（初始 + 临时 + 反向），实际 %d", len(snap.Commits))
	}
}

func TestResetToSoftKeepsChangesStaged(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	commitFile(t, work, "second.txt", "内容", "第二次提交")

	if _, err := e.ResetTo("HEAD~1", "soft"); err != nil {
		t.Fatalf("soft 回退失败: %v", err)
	}

	snap, _ := e.Snapshot()
	if len(snap.Commits) != 1 {
		t.Errorf("应该退回 1 次提交，实际 %d", len(snap.Commits))
	}
	// soft 的关键：改动还在，而且是已暂存状态
	var staged bool
	for _, f := range snap.Files {
		if f.Path == "second.txt" && f.IsStaged {
			staged = true
		}
	}
	if !staged {
		t.Errorf("soft 回退后改动应该在暂存区: %+v", snap.Files)
	}
}

func TestResetToRejectsBadMode(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	if _, err := e.ResetTo("HEAD", "yolo"); err == nil {
		t.Error("非法的回退模式应该被拒绝")
	}
}

// 修补最后一次提交：不带信息时只换内容，提交信息保持不变。
func TestAmendCommitKeepsMessage(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	// makeRepo 已经有一次「init」提交，这里再加一次，然后往这次里补文件
	commitFile(t, work, "a.txt", "v2", "原始提交信息")
	if err := os.WriteFile(filepath.Join(work, "补进来的.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, work, "add", "-A")

	if _, err := e.AmendCommit("", ""); err != nil {
		t.Fatalf("修补提交失败: %v", err)
	}

	snap, _ := e.Snapshot()
	if len(snap.Commits) != 2 {
		t.Fatalf("修补不该增加提交数，实际 %d", len(snap.Commits))
	}
	if snap.Commits[0].Subject != "原始提交信息" {
		t.Errorf("不带信息修补时提交信息应保持原样，实际 %q", snap.Commits[0].Subject)
	}
	// 文件要真的进了这次提交：工作区应该不再是「有改动」的状态
	for _, f := range snap.Files {
		if f.Path == "补进来的.txt" {
			t.Errorf("文件应该已经被补进上次提交，却还在改动列表里: %+v", f)
		}
	}
}

// 带上新信息修补时，提交信息要被替换掉。
func TestAmendCommitReplacesMessage(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	commitFile(t, work, "a.txt", "v2", "写错了的信息")

	if _, err := e.AmendCommit("改好的信息", "补充说明"); err != nil {
		t.Fatalf("修补提交失败: %v", err)
	}

	snap, _ := e.Snapshot()
	if len(snap.Commits) != 2 {
		t.Fatalf("修补不该增加提交数，实际 %d", len(snap.Commits))
	}
	if snap.Commits[0].Subject != "改好的信息" {
		t.Errorf("提交信息应该是新的，实际 %q", snap.Commits[0].Subject)
	}
}

// ---------------------------------------------------------------------------
// 撤销上一步
// ---------------------------------------------------------------------------

func TestUndoLastRevertsCommitAndKeepsWork(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	commitFile(t, work, "undo.txt", "内容", "待撤销的提交")

	snap, _ := e.Snapshot()
	if !snap.CanUndo {
		t.Fatal("刚提交完应该可以撤销")
	}
	if !strings.Contains(snap.UndoHint, "待撤销的提交") {
		t.Errorf("撤销提示应该说明会撤掉什么，实际 %q", snap.UndoHint)
	}

	if _, err := e.UndoLast(); err != nil {
		t.Fatalf("撤销失败: %v", err)
	}

	snap, _ = e.Snapshot()
	if len(snap.Commits) != 1 {
		t.Errorf("撤销后应该回到 1 次提交，实际 %d", len(snap.Commits))
	}
	// 关键：改动不能丢，要回到暂存区
	var staged bool
	for _, f := range snap.Files {
		if f.Path == "undo.txt" && f.IsStaged {
			staged = true
		}
	}
	if !staged {
		t.Errorf("撤销后改动应该保留在暂存区，不能丢内容: %+v", snap.Files)
	}
}

// 没有任何历史时撤销按钮应该是不可用的，而不是点了报错。
func TestUndoNotAvailableOnFreshRepo(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	snap, _ := e.Snapshot()
	if snap.CanUndo {
		t.Error("全新仓库不该显示可撤销")
	}
	if _, err := e.UndoLast(); err == nil {
		t.Error("没有可撤销的操作时应该返回错误")
	}
}

// 变基/合并中途不许撤销 —— 那会叠在中间状态上把事情搞乱。
func TestUndoRefusesDuringConflict(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	gitCmd(t, work, "checkout", "-q", "-b", "other")
	commitFile(t, work, "a.txt", "来自 other", "other")
	gitCmd(t, work, "checkout", "-q", "main")
	commitFile(t, work, "a.txt", "来自 main", "main")

	if _, err := e.MergeBranch("other", false, false); err == nil {
		t.Fatal("这一步应该冲突")
	}

	if _, err := e.UndoLast(); err == nil {
		t.Error("处于合并冲突中时不该允许撤销")
	}

	snap, _ := e.Snapshot()
	if snap.CanUndo {
		t.Error("冲突期间撤销按钮应该是灰的")
	}
	// 但中止操作必须能用
	if _, err := e.AbortOperation(); err != nil {
		t.Errorf("中止应该可用: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 丢弃文件的回收站
// ---------------------------------------------------------------------------

func TestDiscardIsRecoverable(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	// 把 a.txt 改成有实质内容的样子，再丢弃
	original := "这是即将被丢弃的内容\n第二行\n"
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := e.DiscardFile("a.txt"); err != nil {
		t.Fatalf("丢弃失败: %v", err)
	}

	// 丢弃后内容应该回到 HEAD 版本
	after, _ := os.ReadFile(filepath.Join(work, "a.txt"))
	if string(after) == original {
		t.Fatal("丢弃之后内容应该被还原成 HEAD 版本")
	}

	// 但回收站里应该留着刚才的内容
	records := e.DiscardedFiles()
	if len(records) != 1 {
		t.Fatalf("回收站应该有 1 条记录，实际 %d 条", len(records))
	}
	if records[0].Path != "a.txt" || records[0].Truncated {
		t.Fatalf("回收记录不对: %+v", records[0])
	}

	if _, err := e.RestoreDiscarded(records[0].ID); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	back, _ := os.ReadFile(filepath.Join(work, "a.txt"))
	if string(back) != original {
		t.Errorf("恢复的内容不对:\n期望 %q\n实际 %q", original, back)
	}

	// 恢复过的记录不该还留在列表里
	if len(e.DiscardedFiles()) != 0 {
		t.Error("恢复成功后记录应该被移除")
	}
}

// 未跟踪文件被丢弃后文件真的没了，恢复要能把它整个写回来。
func TestDiscardUntrackedFileIsRecoverable(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	content := "未跟踪文件的内容\n"
	if err := os.WriteFile(filepath.Join(work, "scratch.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := e.DiscardFile("scratch.txt"); err != nil {
		t.Fatalf("丢弃失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "scratch.txt")); !os.IsNotExist(err) {
		t.Fatal("未跟踪文件丢弃后应该从磁盘上消失")
	}

	records := e.DiscardedFiles()
	if len(records) != 1 {
		t.Fatalf("回收站应该有 1 条记录，实际 %d", len(records))
	}
	if _, err := e.RestoreDiscarded(records[0].ID); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	back, err := os.ReadFile(filepath.Join(work, "scratch.txt"))
	if err != nil {
		t.Fatalf("文件应该被写回来: %v", err)
	}
	if string(back) != content {
		t.Errorf("内容不对: %q", back)
	}
}

// 用户恢复前又改了那个文件的话，必须拒绝恢复，不能把新改动冲掉。
func TestRestoreRefusesToClobberNewEdits(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)

	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("要被丢弃的"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := e.DiscardFile("a.txt"); err != nil {
		t.Fatal(err)
	}
	records := e.DiscardedFiles()
	if len(records) != 1 {
		t.Fatal("应该有回收记录")
	}

	// 用户又动手改了它
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("我新写的内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := e.RestoreDiscarded(records[0].ID); err == nil {
		t.Error("文件有新改动时不该允许恢复覆盖")
	}
	now, _ := os.ReadFile(filepath.Join(work, "a.txt"))
	if string(now) != "我新写的内容" {
		t.Errorf("用户的改动被覆盖了: %q", now)
	}
}

// ---------------------------------------------------------------------------
// 自动刷新（文件监听）
// ---------------------------------------------------------------------------

// 仓库被外部改动时要通知上层 —— 这正是「在编辑器里改完文件切回来，
// 列表还是旧的」那个问题的解法。
func TestWatcherNotifiesOnExternalChange(t *testing.T) {
	work, _ := makeRepo(t)
	openEngineWithIdentity(t, work)

	// 监听必须在打开仓库之前注册
	var hits int32
	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	e.SetRepoChangeHandler(func() { atomic.AddInt32(&hits, 1) })
	if _, err := e.OpenRepo(work); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = e.Close() }()

	if err := os.WriteFile(filepath.Join(work, "external.txt"), []byte("外部改动"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 监听有 400ms 的合并窗口，给足余量
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&hits) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("外部改动文件后没有收到刷新通知")
}

// 没注册回调时不该装监听（也就不会有任何开销）。
func TestWatcherDisabledWithoutHandler(t *testing.T) {
	work, _ := makeRepo(t)
	e := openEngineWithIdentity(t, work)
	defer func() { _ = e.Close() }()

	if e.watcher != nil {
		t.Error("没注册回调时不该启动监听")
	}
}
