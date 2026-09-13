package engine

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// 重试策略：瞬时失败要退避重试，成功了就停。
func TestRetryPolicyRetriesTransientFailure(t *testing.T) {
	var delays []time.Duration
	p := newRetryPolicy(3, 100*time.Millisecond)
	p.sleep = func(d time.Duration) { delays = append(delays, d) }

	calls := 0
	tries, err := p.run(
		func(error) bool { return true },
		func() error {
			calls++
			if calls < 3 {
				return fmt.Errorf("连接被重置")
			}
			return nil
		},
	)

	if err != nil {
		t.Fatalf("第 3 次应该成功，却报错: %v", err)
	}
	if tries != 3 || calls != 3 {
		t.Errorf("应该尝试 3 次，实际 tries=%d calls=%d", tries, calls)
	}
	if len(delays) != 2 || delays[0] != 100*time.Millisecond || delays[1] != 200*time.Millisecond {
		t.Errorf("退避节奏不对: %v（期望 100ms 然后 200ms）", delays)
	}
}

// 确定性失败必须立刻结束，不能重试——否则只是让用户多等两秒再看到同样的错误。
func TestRetryPolicyStopsOnPermanentFailure(t *testing.T) {
	p := newRetryPolicy(3, time.Millisecond)
	slept := 0
	p.sleep = func(time.Duration) { slept++ }

	calls := 0
	tries, err := p.run(
		func(error) bool { return false },
		func() error {
			calls++
			return fmt.Errorf("! [被拒绝] main -> main (fetch first)")
		},
	)

	if err == nil {
		t.Fatal("应该返回错误")
	}
	if tries != 1 || calls != 1 {
		t.Errorf("确定性失败只该尝试 1 次，实际 tries=%d calls=%d", tries, calls)
	}
	if slept != 0 {
		t.Errorf("不该有任何退避等待，实际等了 %d 次", slept)
	}
}

func TestRetryPolicyGivesUpAfterMaxAttempts(t *testing.T) {
	p := newRetryPolicy(2, time.Millisecond)
	p.sleep = func(time.Duration) {}

	calls := 0
	tries, err := p.run(
		func(error) bool { return true },
		func() error { calls++; return fmt.Errorf("一直失败") },
	)

	if err == nil {
		t.Fatal("用尽次数后应该返回最后一次错误")
	}
	if tries != 2 || calls != 2 {
		t.Errorf("应该尝试 2 次，实际 tries=%d calls=%d", tries, calls)
	}
}

func TestHTTPRetryable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"没拿到响应（TLS/DNS/超时）", &transportError{err: errors.New("EOF")}, true},
		{"5xx", &apiAttemptError{status: 500, msg: "boom"}, true},
		{"502", &apiAttemptError{status: 502, msg: "bad gateway"}, true},
		{"503", &apiAttemptError{status: 503, msg: "unavailable"}, true},
		{"504", &apiAttemptError{status: 504, msg: "timeout"}, true},
		{"429 限流", &apiAttemptError{status: 429, msg: "rate limited"}, true},
		{"401 token 不对", &apiAttemptError{status: 401, msg: "bad credentials"}, false},
		{"403 没权限", &apiAttemptError{status: 403, msg: "forbidden"}, false},
		{"404", &apiAttemptError{status: 404, msg: "not found"}, false},
		{"422 仓库重名", &apiAttemptError{status: 422, msg: "name already exists"}, false},
		{"解析失败这类本地错误", errors.New("解析 GitHub 返回失败"), false},
	}
	for _, c := range cases {
		if got := httpRetryable(c.err); got != c.want {
			t.Errorf("%s: 期望 %v，实际 %v", c.name, c.want, got)
		}
	}
}

// 推送失败分类用的都是 git 真实吐出来的文案——这份机器上 git 是中文的，
// 所以中英文两种文案都必须认。
func TestPushRetryable(t *testing.T) {
	cases := []struct {
		name           string
		output         string
		includeMissing bool
		want           bool
	}{
		{
			name:   "TLS 握手中途被掐（中文 git 的原文）",
			output: "致命错误：无法访问 'https://github.com/x/y.git/'：GnuTLS, handshake failed: TLS 链接非正常地终止了。",
			want:   true,
		},
		{
			name:   "连接被重置",
			output: "fatal: unable to access 'https://github.com/x/y.git/': Recv failure: Connection reset by peer",
			want:   true,
		},
		{
			name:   "代理端口没在监听",
			output: "致命错误：无法访问 'https://github.com/x/y.git/'：Failed to connect to 127.0.0.1 port 7890 after 0 ms: Could not connect to server",
			want:   true,
		},
		{
			name:   "域名解析不了",
			output: "fatal: unable to access 'https://github.com/x/y.git/': Could not resolve host: github.com",
			want:   true,
		},
		{
			name:   "远端 5xx",
			output: "error: RPC failed; HTTP 502 curl 22 The requested URL returned error: 502",
			want:   true,
		},
		{
			name:   "non-fast-forward：远端有本地没有的提交",
			output: "! [rejected]        main -> main (fetch first)\nerror: failed to push some refs to 'https://github.com/x/y.git'",
			want:   false,
		},
		{
			name:   "non-fast-forward（中文 git）",
			output: "! [被拒绝]        main -> main (fetch first)",
			want:   false,
		},
		{
			name:   "认证失败",
			output: "fatal: Authentication failed for 'https://github.com/x/y.git/'",
			want:   false,
		},
		{
			name:   "token 没权限",
			output: "fatal: unable to access 'https://github.com/x/y.git/': The requested URL returned error: 403",
			want:   false,
		},
		{
			name:   "仓库不可写",
			output: "remote: Write access to repository not granted.\nfatal: unable to access 'https://github.com/x/y.git/'",
			want:   false,
		},
		{
			name:   "本地路径不是仓库",
			output: "fatal: '/tmp/nope.git' does not appear to be a git repository",
			want:   false,
		},
		{
			name:           "刚建好的仓库短暂报不存在：该重试",
			output:         "致命错误：仓库 'https://gitee.com/u/r.git/' 未找到",
			includeMissing: true,
			want:           true,
		},
		{
			name:           "同一个「不存在」，在「推到已有仓库」模式下不重试",
			output:         "致命错误：仓库 'https://gitee.com/u/r.git/' 未找到",
			includeMissing: false,
			want:           false,
		},
	}

	for _, c := range cases {
		err := &gitAttemptError{output: c.output, err: errors.New("exit status 128")}
		if got := pushRetryable(err, c.includeMissing); got != c.want {
			t.Errorf("%s: 期望重试=%v，实际 %v", c.name, c.want, got)
		}
	}

	if pushRetryable(nil, true) {
		t.Error("nil 不该被判为可重试")
	}
}

func TestAllLoopback(t *testing.T) {
	cases := []struct {
		addrs []string
		want  bool
	}{
		{[]string{"127.0.0.1"}, true},
		{[]string{"127.0.0.1", "::1"}, true},
		{[]string{"140.82.121.4"}, false},
		{[]string{"127.0.0.1", "140.82.121.4"}, false},
		{[]string{}, false},
		{[]string{"不是IP"}, false},
	}
	for _, c := range cases {
		if got := allLoopback(c.addrs); got != c.want {
			t.Errorf("allLoopback(%v) 期望 %v，实际 %v", c.addrs, c.want, got)
		}
	}
}

func TestFirstProxyFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	if got := firstProxyFromEnv(env(map[string]string{"https_proxy": "http://a:1"})); got != "http://a:1" {
		t.Errorf("小写 https_proxy 没认出来: %q", got)
	}
	if got := firstProxyFromEnv(env(map[string]string{"ALL_PROXY": "socks5://b:2"})); got != "socks5://b:2" {
		t.Errorf("ALL_PROXY 没认出来: %q", got)
	}
	// https_proxy 优先于 all_proxy：Go 的 http 客户端只认前者，诊断要按最可能生效的来
	if got := firstProxyFromEnv(env(map[string]string{
		"HTTPS_PROXY": "http://first:1", "ALL_PROXY": "socks5://second:2",
	})); got != "http://first:1" {
		t.Errorf("优先级不对: %q", got)
	}
	if got := firstProxyFromEnv(env(nil)); got != "" {
		t.Errorf("没有代理时应返回空串，实际 %q", got)
	}
}

func TestProxyHostPort(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"http://127.0.0.1:7890", "127.0.0.1:7890"},
		{"127.0.0.1:7890", "127.0.0.1:7890"},
		{"socks5://127.0.0.1:10808", "127.0.0.1:10808"},
		{"http://proxy.local", "proxy.local:80"},
		{"https://proxy.local", "proxy.local:443"},
		{"socks5h://proxy.local", "proxy.local:1080"},
	}
	for _, c := range cases {
		got, err := proxyHostPort(c.in)
		if err != nil {
			t.Errorf("proxyHostPort(%q) 报错: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("proxyHostPort(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}

	for _, bad := range []string{"", "   ", "http://"} {
		if _, err := proxyHostPort(bad); err == nil {
			t.Errorf("proxyHostPort(%q) 应该报错", bad)
		}
	}
}
