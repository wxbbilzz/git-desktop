package engine

import (
	"os"
	"os/exec"
	"strings"
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
	runWithTimeout("Push(无上游)", func() (*RepoSnapshot, error) { return e.Push() })

	t.Log("2) 首次推送并设置上游")
	snap, err := e.PushSetUpstream("origin")
	if err != nil {
		t.Fatalf("PushSetUpstream 失败: %v", err)
	}
	_ = snap

	t.Log("3) Fetch")
	runWithTimeout("Fetch", func() (*RepoSnapshot, error) { return e.Fetch() })

	t.Log("4) Pull")
	runWithTimeout("Pull", func() (*RepoSnapshot, error) { return e.Pull() })

	// 验证真的推上去了（裸仓库的 HEAD 可能指向 master，所以直接看 main 这个 ref）
	out := git(bare, "log", "--oneline", "-1", "main")
	if !strings.Contains(out, "init") {
		t.Errorf("远端 main 应该已经有提交，实际: %s", out)
	}
	t.Log("✅ 远端确实收到了提交")
}
