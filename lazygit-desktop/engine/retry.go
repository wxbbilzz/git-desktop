package engine

import (
	"errors"
	"strings"
	"time"
)

// 本文件判断「一次失败是暂时的还是永久的」，以及瞬时失败怎么重试。
//
// 背景：上传到 GitHub/Gitee 时，链路抖一下、代理节点抽风、TLS 握手中途被掐，
// 都会让这一次请求失败，但重试一下往往就过了。以前是失败即终止，
// 表现出来就是「有时成功、有时失败」。现在瞬时故障自动重试，
// 确定性的失败（重名、token 不对、non-fast-forward）则立刻结束，不浪费用户时间。

// 重试节奏：最多 3 次尝试，退避 700ms、1.4s。
//
// 次数不取大：用户就在界面前等着，而且这类故障要么几秒内自愈，要么根本不是抖动。
const (
	apiAttempts  = 3
	pushAttempts = 3
	retryBase    = 700 * time.Millisecond
)

// retryPolicy 描述重试节奏：最多 attempts 次尝试，退避 base、2*base、4*base…
type retryPolicy struct {
	attempts int
	base     time.Duration
	sleep    func(time.Duration)
	onRetry  func(attempt int, err error)
}

func newRetryPolicy(attempts int, base time.Duration) retryPolicy {
	return retryPolicy{attempts: attempts, base: base, sleep: time.Sleep}
}

// run 反复调用 fn，直到成功、被判为不值得重试，或达到次数上限。
// 返回实际尝试次数和最后一次的错误。
func (p retryPolicy) run(isRetryable func(error) bool, fn func() error) (int, error) {
	if p.attempts < 1 {
		p.attempts = 1
	}
	delay := p.base

	var err error
	for attempt := 1; attempt <= p.attempts; attempt++ {
		if err = fn(); err == nil {
			return attempt, nil
		}
		if attempt == p.attempts || !isRetryable(err) {
			return attempt, err
		}
		if p.onRetry != nil {
			p.onRetry(attempt+1, err)
		}
		if p.sleep != nil && delay > 0 {
			p.sleep(delay)
		}
		delay *= 2
	}
	return p.attempts, err
}

// ---------------------------------------------------------------- 失败分类

// apiAttemptError 是一次 API 请求得到的「HTTP 错误响应」。
// 把状态码单独带出来，重试策略才能按状态码判断是否值得再试。
type apiAttemptError struct {
	status int
	msg    string
}

func (e *apiAttemptError) Error() string { return e.msg }

// transportError 表示请求根本没拿到响应：DNS、连接、TLS、超时。
// 这类错误全都可以重试。
type transportError struct{ err error }

func (e *transportError) Error() string { return e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

// gitAttemptError 带上 git 的原始输出，便于按输出内容判断是否值得重试。
type gitAttemptError struct {
	output string
	err    error
}

func (e *gitAttemptError) Error() string { return e.err.Error() }
func (e *gitAttemptError) Unwrap() error { return e.err }

// httpRetryable 判断一次 API 失败是否值得重试。
//
// 只有「没拿到响应」和「服务端自己出问题」才重试：
// 4xx 是确定性的（token 没权限、仓库重名），再试一百次也是同样的结果。
func httpRetryable(err error) bool {
	var ae *apiAttemptError
	if errors.As(err, &ae) {
		switch ae.status {
		case 429, 500, 502, 503, 504:
			return true
		}
		return false
	}

	var te *transportError
	return errors.As(err, &te)
}

// pushPermanentPatterns 是「再试也没用」的失败特征。
// 先查这张表，避免把确定性失败当成抖动反复推送。
var pushPermanentPatterns = []string{
	// 远端有自己的提交，本地不是快进
	"non-fast-forward", "fetch first", "updates were rejected", "被拒绝", "非快进",
	// 认证问题
	"authentication failed", "认证失败", "invalid username or password",
	"could not read username", "could not read password", "permission denied",
	"403 forbidden", "password authentication was removed", "终端提示已禁用",
	// git 把 HTTP 状态码嵌在一句 "无法访问 'URL'" 里，别被当成网络抖动
	"returned error: 401", "returned error: 403",
	// 目标不存在或不可写
	"does not appear to be a git repository", "not a git repository",
	// 被服务端规则挡下
	"pre-receive hook declined", "protected branch", "hook declined",
	// 本地参数问题
	"no such remote", "src refspec",
}

// pushTransientPatterns 是「多半是网络抖了一下」的失败特征。
//
// 中英文都要认：git 的报错跟随系统语言，中文环境下是「致命错误」「TLS 链接非正常地终止了」。
var pushTransientPatterns = []string{
	"connection reset", "connection was reset", "connection closed", "connection timed out",
	"连接被对方重置", "连接被重置", "连接被关闭", "连接超时",
	"unexpected eof", "early eof", "premature eof", "eof while reading",
	"意外的 eof", "非正常地终止",
	"handshake failed", "tls connect error", "ssl_read", "ssl_connect", "握手失败",
	"could not resolve host", "temporary failure in name resolution",
	"无法解析主机", "名称或服务未知",
	"failed to connect", "无法连接", "无法访问", "连接失败",
	"timed out", "timeout", "超时",
	"the remote end hung up", "remote end hung up", "远端意外挂断", "远程端意外挂断",
	"rpc failed", "rpc 失败",
	"internal server error", "服务端错误",
	"502", "503", "504", "500 bad gateway",
	"broken pipe", "network is unreachable", "网络不可达", "connection refused", "拒绝连接",
}

// pushRetryable 判断一次推送失败是否值得重试。
//
// includeNotFound 用于「刚建好仓库」的场景：远端仓库刚创建时，个别平台
// 会有短暂的「仓库不存在」窗口，这种情况值得再试一次。
func pushRetryable(err error, includeNotFound bool) bool {
	if err == nil {
		return false
	}

	text := strings.ToLower(err.Error())
	var ge *gitAttemptError
	if errors.As(err, &ge) && ge.output != "" {
		text = strings.ToLower(ge.output)
	}

	for _, p := range pushPermanentPatterns {
		if strings.Contains(text, p) {
			return false
		}
	}

	if includeNotFound {
		// 注意匹配「未找到」而不是「仓库未找到」：git 会把仓库地址夹在中间，
		// 实际文案是 `仓库 'https://…' 未找到` / `repository 'https://…' not found`。
		for _, p := range []string{"not found", "未找到", "仓库不存在"} {
			if strings.Contains(text, p) {
				return true
			}
		}
	}

	for _, p := range pushTransientPatterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}
