package engine

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

// 本文件负责「失败了，为什么，怎么办」。
//
// 有一类上传失败，光看 git 的报错根本猜不到原因，而这台机器上真的发生过：
// 域名被 /etc/hosts 指到 127.0.0.1（加速器卸载后留下的残留），于是所有请求
// 都在本机打转；另一类是环境里配了代理、但代理没在跑。这两种情况下
// 「连接被拒绝」「TLS 握手失败」对用户没有任何指导意义，所以这里主动探一下，
// 翻译成一句能照着做的话。
//
// 只在失败路径上调用，所以这里允许做 DNS 查询和 1.5 秒的拨号探测。

const (
	diagResolveTimeout = 2 * time.Second
	diagDialTimeout    = 1500 * time.Millisecond
)

// suggestForNetworkFailure 在重试用尽后给出「可能的原因 + 怎么办」。
// host 是这次要访问的主机名（api.github.com / github.com …）。
func suggestForNetworkFailure(host string, attempts int) string {
	ctx, cancel := context.WithTimeout(context.Background(), diagResolveTimeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)

	switch {
	case err != nil:
		return fmt.Sprintf("域名 %s 解析失败，检查一下网络和 DNS。", host)
	case len(addrs) > 0 && allLoopback(addrs):
		return fmt.Sprintf("域名 %s 被解析到了 127.0.0.1 —— 请求根本没离开这台机器。"+
			"通常是加速器（Steam++ / Watt Toolkit 之类）改过 /etc/hosts，而加速器已经不在运行。"+
			"把 /etc/hosts 里指向 127.0.0.1 的那几行删掉即可恢复。", host)
	}

	if proxy := firstProxyFromEnv(os.Getenv); proxy != "" {
		addr, perr := proxyHostPort(proxy)
		if perr == nil {
			conn, derr := net.DialTimeout("tcp", addr, diagDialTimeout)
			if derr != nil {
				return fmt.Sprintf("环境变量里配了代理 %s，但它连不上（代理软件没启动？）。"+
					"启动代理，或者把 http_proxy / https_proxy 去掉后重试。", proxy)
			}
			_ = conn.Close()
			return fmt.Sprintf("代理 %s 本身是通的，但这次请求没能完成 —— 多半是代理的上游节点不稳。"+
				"在代理软件里换一个节点、做一次延迟测试再试。", proxy)
		}
	}

	return fmt.Sprintf("看起来是链路抖动（已经自动重试 %d 次）。隔一会儿再试，"+
		"或者换一个网络 / 代理节点。", attempts)
}

// allLoopback 判断解析结果是否全部指向本机。
func allLoopback(addrs []string) bool {
	if len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil || !ip.IsLoopback() {
			return false
		}
	}
	return true
}

// firstProxyFromEnv 找出环境里配的代理地址。
//
// 大小写都要认（两种写法在实际环境里都常见），而且 Go 的 http 客户端
// 只认 HTTP(S)_PROXY、不认 ALL_PROXY —— 诊断时都要看，才能说清为什么
// 「git 能推、创建仓库却失败」。顺序按「最可能生效」排。
func firstProxyFromEnv(getenv func(string) string) string {
	for _, k := range []string{
		"HTTPS_PROXY", "https_proxy",
		"ALL_PROXY", "all_proxy",
		"HTTP_PROXY", "http_proxy",
	} {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// proxyHostPort 从代理地址里取出可拨号的 host:port。
func proxyHostPort(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("代理地址为空")
	}
	// 允许只写 127.0.0.1:7890 这种没有协议的写法
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("代理地址里没有主机名")
	}
	if port := u.Port(); port != "" {
		return net.JoinHostPort(host, port), nil
	}

	switch strings.ToLower(u.Scheme) {
	case "https":
		return net.JoinHostPort(host, "443"), nil
	case "socks", "socks5", "socks5h", "socks4", "socks4a":
		return net.JoinHostPort(host, "1080"), nil
	default:
		return net.JoinHostPort(host, "80"), nil
	}
}
