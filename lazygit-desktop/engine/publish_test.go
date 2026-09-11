package engine

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseRepoURL(t *testing.T) {
	cases := []struct{ in, owner, name string }{
		{"https://gitee.com/user/repo.git", "user", "repo"},
		{"https://gitee.com/user/repo", "user", "repo"},
		{"https://github.com/me/my-project.git", "me", "my-project"},
		{"git@gitee.com:user/repo.git", "user", "repo"},
		{"https://gitee.com/group/sub/repo.git", "sub", "repo"},
	}
	for _, c := range cases {
		r, err := parseRepoURL(c.in)
		if err != nil {
			t.Errorf("解析 %s 失败: %v", c.in, err)
			continue
		}
		if r.owner != c.owner || r.name != c.name {
			t.Errorf("%s -> %s/%s，期望 %s/%s", c.in, r.owner, r.name, c.owner, c.name)
		}
	}
	for _, bad := range []string{"", "https://gitee.com/onlyone", "随便写的"} {
		if _, err := parseRepoURL(bad); err == nil {
			t.Errorf("%q 应该被拒绝", bad)
		}
	}
}

// 回归测试：以前只能「新建仓库」，仓库已存在时平台 API 报错，
// 用户就没法把代码推上去了。现在必须支持「推到已有仓库」。
func TestPublishToExistingRepo(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	root := t.TempDir()
	bare := root + "/existing.git"
	work := root + "/work"

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
	if err := os.WriteFile(work+"/a.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(work, "add", "-A")
	git(work, "commit", "-qm", "init")

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(work); err != nil {
		t.Fatal(err)
	}

	// mode=existing：跳过平台 API，直接添加远端并推送。
	// 这里用本地裸仓库当远端 —— 顺便验证「非 http(s) 地址不注入 token、
	// 也不要求 token」这条逻辑。
	res, err := e.Publish(PublishRequest{
		Platform:   "gitee",
		Mode:       "existing",
		RepoURL:    bare,
		StoreToken: true, // 即使勾了「记住凭据」，也不该污染本地路径
	}, nil)
	if err != nil {
		t.Fatalf("硬错误: %v", err)
	}
	if !res.OK {
		t.Fatalf("推到已有仓库失败: %s", res.Error)
	}

	out, _ := exec.Command("git", "-C", bare, "log", "--oneline", "-1", "main").CombinedOutput()
	if !strings.Contains(string(out), "init") {
		t.Errorf("远端没有收到提交: %s", out)
	}

	url, _ := e.gitOutput("remote", "get-url", "origin")
	if strings.Contains(url, "@") && !strings.Contains(url, "git@") {
		t.Errorf("远端地址被 token 污染了: %s", url)
	}
}

// 回归测试：上传对话框要能预填「上次用过的仓库地址」，
// 并且绝不能把地址里的 token 显示到界面上。
func TestPublishDefaults(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	dir := t.TempDir()
	git := func(args ...string) {
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

	git("init", "-q", "--initial-branch=main")
	if err := os.WriteFile(dir+"/a.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "init")

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(dir); err != nil {
		t.Fatal(err)
	}

	// 没有远端时不应该报错，只是返回空的地址
	d, err := e.PublishDefaults()
	if err != nil {
		t.Fatalf("没有远端时不该报错: %v", err)
	}
	if d.RemoteURL != "" {
		t.Errorf("没有远端时地址应为空，实际 %q", d.RemoteURL)
	}

	// 配一个带 token 的地址
	git("remote", "add", "origin",
		"https://oauth2:SECRET_TOKEN@example.com/user/repo.git")

	d, err = e.PublishDefaults()
	if err != nil {
		t.Fatal(err)
	}
	if d.RemoteName != "origin" {
		t.Errorf("远端名应为 origin，实际 %q", d.RemoteName)
	}
	if strings.Contains(d.RemoteURL, "SECRET_TOKEN") {
		t.Errorf("❌ 地址里的 token 泄露到界面了: %s", d.RemoteURL)
	}
	if d.RemoteURL != "https://example.com/user/repo.git" {
		t.Errorf("剥离后的地址不对: %s", d.RemoteURL)
	}
}
