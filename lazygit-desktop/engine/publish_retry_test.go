package engine

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// 本文件验证「上传遇到瞬时故障会自己重试」这条链路是通的。
//
// 背景：用户的实际观感是「有时成功有时失败」——同一个仓库、同样的操作，
// 一会儿成一会儿败。排查下来是本机到 GitHub 的链路本来就在丢包
// （代理节点不稳 / TLS 握手中途被掐）。既然故障是瞬时的，就该重试。

// makeRepo 建一个「一个提交的本地仓库 + 一个裸仓库当远端」，返回两边路径。
func makeRepo(t *testing.T) (work, bare string) {
	t.Helper()

	root := t.TempDir()
	work = filepath.Join(root, "work")
	bare = filepath.Join(root, "remote.git")

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

	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	git(bare, "init", "-q", "--bare")

	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	git(work, "init", "-q", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(work, "add", "-A")
	git(work, "commit", "-qm", "init")

	return work, bare
}

func openEngine(t *testing.T, work string) *Engine {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.OpenRepo(work); err != nil {
		t.Fatal(err)
	}
	return e
}

// 创建仓库的 API 连吃两个 503 之后成功 —— 整条链路（API 重试 + 推送）都要跑通。
func TestPublishRetriesAPICreate(t *testing.T) {
	work, bare := makeRepo(t)

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"Server Error"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		// clone_url 指向本地裸仓库，这样推送是真的会发生，但不用出网
		_, _ = fmt.Fprintf(w, `{"html_url":"https://example.com/u/demo","clone_url":%q,"name":"demo","owner":{"login":"u"}}`, bare)
	}))
	defer srv.Close()

	old := platformAPIBase[PlatformGitHub]
	platformAPIBase[PlatformGitHub] = srv.URL
	defer func() { platformAPIBase[PlatformGitHub] = old }()

	e := openEngine(t, work)

	var steps []string
	res, err := e.Publish(PublishRequest{
		Platform: PlatformGitHub,
		Mode:     "create",
		Token:    "t",
		Name:     "demo",
	}, func(s string) { steps = append(steps, s) })

	if err != nil {
		t.Fatalf("硬错误: %v", err)
	}
	if !res.OK {
		t.Fatalf("API 重试之后应该成功，却失败了: %s（建议: %s）", res.Error, res.Suggestion)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("应该请求平台 API 3 次（两次 503 后成功），实际 %d 次", got)
	}
	if res.Suggestion != "" {
		t.Errorf("成功了不该带失败建议: %s", res.Suggestion)
	}

	// 推送要真的落到了远端
	out, _ := exec.Command("git", "-C", bare, "log", "--oneline", "-1", "main").CombinedOutput()
	if !strings.Contains(string(out), "init") {
		t.Errorf("远端没有收到提交: %s", out)
	}

	// 重试过程要让用户看得见
	joined := strings.Join(steps, " | ")
	if !strings.Contains(joined, "重试") {
		t.Errorf("重试过程没有反馈给界面: %s", joined)
	}
}

// 推送连吃两次瞬时故障之后成功。
func TestPublishRetriesTransientPushFailure(t *testing.T) {
	work, bare := makeRepo(t)
	e := openEngine(t, work)

	var calls int32
	e.pushRunner = func(args ...string) (string, error) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			// 用户机器上真实出现过的报错原文
			return "致命错误：无法访问 'https://github.com/u/r.git/'：GnuTLS, handshake failed: TLS 链接非正常地终止了。",
				errors.New("exit status 128")
		}
		return e.gitRun(args...)
	}

	res, err := e.Publish(PublishRequest{
		Platform: PlatformGitee,
		Mode:     "existing",
		RepoURL:  bare,
	}, nil)
	if err != nil {
		t.Fatalf("硬错误: %v", err)
	}
	if !res.OK {
		t.Fatalf("瞬时故障应该被重试掉，实际失败: %s（建议: %s）", res.Error, res.Suggestion)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("应该推送 3 次（两次失败后成功），实际 %d 次", got)
	}

	out, _ := exec.Command("git", "-C", bare, "log", "--oneline", "-1", "main").CombinedOutput()
	if !strings.Contains(string(out), "init") {
		t.Errorf("远端没有收到提交: %s", out)
	}
}

// 确定性失败（远端有本地没有的提交）不许重试，也不许瞎猜成网络问题。
// 这条是防「好心办坏事」：把一个「该先 pull」的问题说成「链路抖动」，
// 用户会一直重试下去。
func TestPublishDoesNotRetryPermanentFailure(t *testing.T) {
	work, bare := makeRepo(t)
	e := openEngine(t, work)

	const rejected = "! [被拒绝]        main -> main (fetch first)\n" +
		"error: 无法推送一些引用到 'https://example.com/u/r.git'"

	var calls int32
	e.pushRunner = func(args ...string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return rejected, errors.New("exit status 1")
	}

	res, err := e.Publish(PublishRequest{
		Platform: PlatformGitee,
		Mode:     "existing",
		RepoURL:  bare,
	}, nil)
	if err != nil {
		t.Fatalf("硬错误: %v", err)
	}
	if res.OK {
		t.Fatal("这不该被当成成功")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("确定性失败只该推 1 次，实际 %d 次", got)
	}
	if res.Suggestion != "" {
		t.Errorf("不该把 non-fast-forward 说成网络问题: %s", res.Suggestion)
	}
	if !strings.Contains(res.Error, "被拒绝") {
		t.Errorf("错误信息应保留 git 的原文，实际: %s", res.Error)
	}
}

// 网络类失败重试用尽后，要给出「为什么 + 怎么办」，而不是只丢一句 exit status。
func TestPublishNetworkFailureExplainsItself(t *testing.T) {
	work, bare := makeRepo(t)
	e := openEngine(t, work)

	var calls int32
	e.pushRunner = func(args ...string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "fatal: unable to access 'https://gitee.com/u/r.git/': Recv failure: Connection reset by peer",
			errors.New("exit status 128")
	}

	res, err := e.Publish(PublishRequest{
		Platform:   "gitee",
		Mode:       "existing",
		RepoURL:    bare,
		StoreToken: true,
	}, nil)
	if err != nil {
		t.Fatalf("硬错误: %v", err)
	}
	if res.OK {
		t.Fatal("一直失败就不该报成功")
	}
	if got := atomic.LoadInt32(&calls); got != pushAttempts {
		t.Errorf("应该用尽 %d 次重试，实际 %d 次", pushAttempts, got)
	}
	if res.Suggestion == "" {
		t.Error("网络类失败必须给出可照做的原因")
	}
	if !strings.Contains(res.Error, "Connection reset") {
		t.Errorf("错误信息应是 git 的原文，实际: %s", res.Error)
	}

	// 失败后远端地址必须还原干净，不能留下带 token 的配置
	url, _ := e.gitOutput("remote", "get-url", "origin")
	if strings.Contains(url, "@") && !strings.Contains(url, "git@") {
		t.Errorf("失败后远端地址被 token 污染了: %s", url)
	}
}
