package engine

import (
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 远端操作必须「要么很快成功，要么很快失败」——绝不能挂住。
// 曾经的 bug：lazygit 的 Sync 方法会把命令放进 PTY，git 因此会弹凭据提示
// 无限等待，界面卡在 busy 状态、按钮全部禁用。
func TestSyncOpsDoNotHang(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	root := t.TempDir()
	bare := root + "/origin.git"
	work := root + "/work"

	git := func(dir string, args ...string) string {
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

	os.MkdirAll(bare, 0o755)
	git(bare, "init", "-q", "--bare")
	os.MkdirAll(work, 0o755)
	git(work, "init", "-q", "--initial-branch=main")
	os.WriteFile(work+"/a.txt", []byte("x"), 0o644)
	git(work, "add", "-A")
	git(work, "commit", "-qm", "init")
	git(work, "remote", "add", "origin", bare)

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(work); err != nil {
		t.Fatal(err)
	}

	// 每次操作都必须在一秒内返回，否则就是挂住了
	runWithTimeout := func(name string, fn func() (*RepoSnapshot, error)) {
		done := make(chan struct{})
		var opErr error
		go func() { _, opErr = fn(); close(done) }()
		select {
		case <-done:
			t.Logf("  %-22s 返回 err=%v", name, opErr)
		case <-time.After(3 * time.Second):
			t.Fatalf("❌ %s 挂住了（超过 3 秒没返回）", name)
		}
	}

	t.Log("1) 没有上游时 push —— 应当快速失败而不是挂住")
	runWithTimeout("Push(无上游)", func() (*RepoSnapshot, error) { return e.Push(nil) })

	t.Log("2) 首次推送并设置上游")
	snap, err := e.PushSetUpstream("origin", nil)
	if err != nil {
		t.Fatalf("PushSetUpstream 失败: %v", err)
	}
	_ = snap

	t.Log("3) Fetch")
	runWithTimeout("Fetch", func() (*RepoSnapshot, error) { return e.Fetch(nil) })

	t.Log("4) Pull")
	runWithTimeout("Pull", func() (*RepoSnapshot, error) { return e.Pull(nil) })

	// 验证真的推上去了（裸仓库的 HEAD 可能指向 master，所以直接看 main 这个 ref）
	out := git(bare, "log", "--oneline", "-1", "main")
	if !strings.Contains(out, "init") {
		t.Errorf("远端 main 应该已经有提交，实际: %s", out)
	}
	t.Log("✅ 远端确实收到了提交")
}

// 回归测试：远端操作期间**不能持有引擎锁**。
//
// 以前 syncOp 从头 lock 到尾，推送几秒钟里界面所有请求都被阻塞，
// 整个应用看起来像死机。现在只有校验和最后刷新时加锁。
func TestSyncDoesNotBlockOtherCalls(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	root := t.TempDir()
	bare, work := root+"/origin.git", root+"/work"
	git := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v 失败: %s", args, out)
		}
	}

	os.MkdirAll(bare, 0o755)
	git(bare, "init", "-q", "--bare")
	os.MkdirAll(work, 0o755)
	git(work, "init", "-q", "--initial-branch=main")

	// 造足够多的文件，让推送有可观察的耗时
	for i := 0; i < 2000; i++ {
		name := work + "/f" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + ".txt"
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(work, "add", "-A")
	git(work, "commit", "-qm", "init")
	git(work, "remote", "add", "origin", bare)

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(work); err != nil {
		t.Fatal(err)
	}

	var progressCount int32
	pushDone := make(chan error, 1)
	go func() {
		_, err := e.PushSetUpstream("origin", func(SyncProgress) {
			atomic.AddInt32(&progressCount, 1)
		})
		pushDone <- err
	}()

	// 推送还在跑的时候，另一个请求应当立刻返回
	time.Sleep(20 * time.Millisecond)
	otherDone := make(chan time.Duration, 1)
	go func() {
		t0 := time.Now()
		_, _ = e.Stashes()
		otherDone <- time.Since(t0)
	}()

	select {
	case err := <-pushDone:
		if err != nil {
			t.Fatalf("推送失败: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("推送超时")
	}

	if atomic.LoadInt32(&progressCount) == 0 {
		t.Error("推送过程中应当收到进度回调（否则界面看不到进度）")
	} else {
		t.Logf("收到 %d 条推送进度", progressCount)
	}

	select {
	case d := <-otherDone:
		if d > 2*time.Second {
			t.Errorf("推送期间其他调用被阻塞了 %v", d)
		} else {
			t.Logf("推送期间只读操作耗时 %v —— 界面保持可用", d)
		}
	case <-time.After(3 * time.Second):
		t.Error("推送期间只读操作被卡住，界面会像死机")
	}
}
