package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 本文件实现「把本地仓库一键上传到 GitHub / Gitee」。
//
// 流程：
//   1. 调用托管平台的 API 创建远端仓库
//   2. 给本地仓库添加（或更新）名为 origin 的远端
//   3. 推送当前分支并设置上游
//
// 为什么需要 token：创建仓库是平台 API 的写操作，必须鉴权。
// token 由用户自己到平台上生成（GitHub: repo 权限；Gitee: projects 权限），
// 软件不保存、不上传，只用于这一次以及后续推送。

const (
	PlatformGitHub = "github"
	PlatformGitee  = "gitee"
)

// platformAPIBase 是各平台 API 的基地址。
//
// 抽成变量而不是写死在请求里，是为了测试能把它指向本地测试服务器，
// 从而端到端验证「API 重试」这条路径（不用真的去建仓库）。
var platformAPIBase = map[string]string{
	PlatformGitHub: "https://api.github.com",
	PlatformGitee:  "https://gitee.com/api/v5",
}

// PublishRequest 是一次上传请求。
type PublishRequest struct {
	Platform string `json:"platform"` // github / gitee
	// Mode 决定这次上传是「新建仓库」还是「推到已有仓库」
	//   create   （默认）调平台 API 建一个新仓库
	//   existing 跳过 API，直接推到 RepoURL 指定的仓库
	Mode string `json:"mode"`
	// RepoURL 只在 Mode=existing 时使用，例如
	//   https://gitee.com/user/repo.git
	//   git@gitee.com:user/repo.git
	RepoURL     string `json:"repoUrl"`
	Token       string `json:"token"`
	Name        string `json:"name"` // 远端仓库名
	Description string `json:"description"`
	Private     bool   `json:"private"`
	RemoteName  string `json:"remoteName"` // 留空用 origin
	Branch      string `json:"branch"`     // 留空用当前分支
	// StoreToken 为真时把 token 写进 remote URL，
	// 这样以后在软件里点「Push」也能直接用（代价是 token 明文存在 .git/config）
	StoreToken bool `json:"storeToken"`
}

// PublishResult 是上传结果。
type PublishResult struct {
	RepoURL  string        `json:"repoUrl"`  // 网页地址
	CloneURL string        `json:"cloneUrl"` // 用于 git 的地址
	Command  string        `json:"command"`
	Output   string        `json:"output"`
	OK       bool          `json:"ok"`
	Error    string        `json:"error"`
	Snapshot *RepoSnapshot `json:"snapshot"`
	// Suggestion 是失败时的「可能原因 + 怎么办」。
	// Error 说的是发生了什么（git/平台的原话），Suggestion 说的是接下来做什么。
	// 只在这类失败确实是网络问题时才填，确定性的失败（重名、token 没权限、
	// non-fast-forward）留空 —— 那时候报错原文比猜测有用。
	Suggestion string `json:"suggestion"`
}

// Publish 把当前仓库上传到托管平台。
// onProgress 会分阶段回调，用来在界面上显示进度。
func (e *Engine) Publish(req PublishRequest, onProgress func(step string)) (*PublishResult, error) {
	if onProgress == nil {
		onProgress = func(string) {}
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	req.Token = strings.TrimSpace(req.Token)
	req.Name = strings.TrimSpace(req.Name)
	req.Branch = strings.TrimSpace(req.Branch)
	req.RemoteName = strings.TrimSpace(req.RemoteName)

	if platform != PlatformGitHub && platform != PlatformGitee {
		return nil, fmt.Errorf("暂不支持该平台：%s", req.Platform)
	}
	// 只有 HTTP(S) 远端才需要 token：
	// SSH 走密钥，本地路径 / file:// 根本不需要认证。
	needsToken := true
	if req.Mode == "existing" {
		u := strings.TrimSpace(req.RepoURL)
		needsToken = strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
	}
	if needsToken && req.Token == "" {
		return nil, fmt.Errorf("请填写访问令牌（token）")
	}
	// 「已有仓库」模式下仓库名是从地址里解析的，不需要单独填
	if req.Mode != "existing" {
		if req.Name == "" {
			return nil, fmt.Errorf("请填写仓库名")
		}
		if !validRepoName(req.Name) {
			return nil, fmt.Errorf("仓库名只能包含字母、数字、点、下划线和连字符")
		}
	}
	if req.RemoteName == "" {
		req.RemoteName = "origin"
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	// 分支留空就用当前分支
	if req.Branch == "" {
		out, err := e.gitOutput("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return nil, err
		}
		req.Branch = strings.TrimSpace(out)
	}
	if req.Branch == "" || req.Branch == "HEAD" {
		return nil, fmt.Errorf("当前处于游离 HEAD 状态，请先切换到一个分支再上传")
	}

	apiHost, gitHost := platformHosts(platform)

	res := &PublishResult{}

	// ---- 1) 拿到远端仓库的信息 ----
	var repo *createdRepo
	var err error
	apiTries := 0
	if req.Mode == "existing" {
		// 推到已有仓库：不调 API，直接从地址里解析出用户名和仓库名
		onProgress("正在连接到已有仓库")
		repo, err = parseRepoURL(req.RepoURL)
		if err != nil {
			res.Error = err.Error()
			return res, nil
		}
	} else {
		onProgress("正在创建远端仓库")
		repo, apiTries, err = createRemoteRepo(platform, req, onProgress)
		if err != nil {
			// 仓库已存在是常见情况，给出可操作的提示而不是干巴巴的报错
			if isAlreadyExists(err) {
				res.Error = "远端已经有同名仓库了。如果那就是你要用的仓库，" +
					"请把上传方式切换成「上传到已有仓库」并填写它的地址。"
				return res, nil
			}
			res.Error = err.Error()
			// 只有确实是网络问题时才解释原因；
			// 4xx（token 没权限之类）报错原文已经说清楚了，不必再猜。
			if httpRetryable(err) {
				res.Suggestion = suggestForNetworkFailure(apiHost, apiTries)
			}
			return res, nil
		}
	}
	res.RepoURL = repo.htmlURL
	res.CloneURL = repo.cloneURL

	// ---- 2) 配置远端 ----
	onProgress("正在配置远端地址")
	remoteURL := repo.cloneURL
	if req.StoreToken {
		remoteURL = authenticatedURL(platform, repo, req.Token)
	}

	existing, _ := e.gitOutput("remote", "get-url", req.RemoteName)
	if strings.TrimSpace(existing) != "" {
		if _, err := e.gitOutput("remote", "set-url", req.RemoteName, remoteURL); err != nil {
			res.Error = err.Error()
			return res, nil
		}
	} else {
		if _, err := e.gitOutput("remote", "add", req.RemoteName, remoteURL); err != nil {
			res.Error = err.Error()
			return res, nil
		}
	}

	// ---- 3) 推送 ----
	onProgress("正在推送代码")

	// 关键：-u 后面必须跟**远程名**（origin），不能跟 URL。
	//
	// 如果跟 URL，git 会把 branch.<name>.remote 直接写成那个 URL，
	// 于是这个分支就再也关联不上 refs/remotes/origin/<branch>：
	//   · git rev-parse @{u} 报「not stored as a remote-tracking branch」
	//   · origin/main 永远不会更新，界面上「领先/落后几个提交」全是错的
	//
	// 认证靠的是上面已经把 remote 的 URL 设成了带 token 的形式，
	// 所以这里直接用名字推送即可。
	argv := []string{"push", "-u", req.RemoteName, req.Branch + ":" + req.Branch}
	res.Command = "git " + strings.Join(redact(argv), " ")

	// 推送可以放心重试：万一第一次其实成功了、只是响应丢在回程，
	// 再推一次只会得到「Everything up-to-date」，不会推重。
	runner := e.pushRunner
	if runner == nil {
		runner = e.gitRun
	}

	var pushOut string
	policy := newRetryPolicy(pushAttempts, retryBase)
	policy.onRetry = func(attempt int, err error) {
		onProgress(fmt.Sprintf("推送被中断，正在重试（第 %d/%d 次）", attempt, pushAttempts))
	}
	pushTries, pushErr := policy.run(
		// 刚建好的仓库，个别平台存在短暂的「仓库不存在」窗口，这种也值得再试
		func(err error) bool { return pushRetryable(err, req.Mode == "create") },
		func() error {
			out, err := runner(argv...)
			pushOut = out
			if err != nil {
				return &gitAttemptError{output: out, err: err}
			}
			return nil
		},
	)

	res.Output = strings.TrimSpace(pushOut)
	if pushErr != nil {
		// git 的原文比 "exit status 128" 有用得多
		if line := strings.TrimSpace(firstErrorLine(res.Output)); line != "" {
			res.Error = line
		} else {
			res.Error = pushErr.Error()
		}
		// 推送失败时把远端地址改回干净地址，避免留下带 token 的配置
		if req.StoreToken {
			_, _ = e.gitOutput("remote", "set-url", req.RemoteName, repo.cloneURL)
		}
		// 只对网络类失败解释原因。non-fast-forward、认证失败这些
		// git 自己说得比我们清楚，再套一句「链路抖动」只会误导。
		if pushRetryable(pushErr, req.Mode == "create") {
			res.Suggestion = suggestForNetworkFailure(gitHost, pushTries)
		}
		return res, nil
	}

	// 推送成功后，如果不需要记住 token，把远端地址恢复成干净形式
	if req.StoreToken {
		// 保留带 token 的地址，方便下次直接推送
	} else {
		_, _ = e.gitOutput("remote", "set-url", req.RemoteName, repo.cloneURL)
	}

	res.OK = true
	onProgress("完成")

	if snap, err := e.snapshotLocked(); err == nil {
		res.Snapshot = snap
	}
	return res, nil
}

// ---------------------------------------------------------------- 平台 API

type createdRepo struct {
	htmlURL  string
	cloneURL string
	owner    string
	name     string
}

// createRemoteRepo 调用平台 API 创建仓库，瞬时失败会自动重试。
//
// 返回实际尝试次数，失败诊断要用它来告诉用户「已经试过几次了」。
func createRemoteRepo(platform string, req PublishRequest, onProgress func(string)) (*createdRepo, int, error) {
	var repo *createdRepo
	policy := newRetryPolicy(apiAttempts, retryBase)
	policy.onRetry = func(attempt int, err error) {
		onProgress(fmt.Sprintf("创建仓库请求被中断，正在重试（第 %d/%d 次）", attempt, apiAttempts))
	}
	tries, err := policy.run(httpRetryable, func() error {
		r, e := createRemoteRepoOnce(platform, req)
		if e == nil {
			repo = r
		}
		return e
	})
	if err != nil {
		return nil, tries, err
	}
	return repo, tries, nil
}

// platformHosts 返回这次上传要连的两个主机名，供失败诊断时探测。
// apiHost 走平台 API（创建仓库），gitHost 走 git 传输（推送）。
func platformHosts(platform string) (apiHost, gitHost string) {
	if platform == PlatformGitHub {
		return "api.github.com", "github.com"
	}
	return "gitee.com", "gitee.com"
}

// createRemoteRepoOnce 是单次尝试：调一次平台 API 建仓库。
//
// 失败时返回带类型的错误，方便上层判断该不该重试：
//   - 没拿到响应（DNS/连接/TLS/超时）→ transportError，可重试
//   - 拿到 5xx / 429 响应 → apiAttemptError，可重试
//   - 4xx（token 没权限、仓库重名）→ apiAttemptError，不重试
//
// 重试一个 POST 是安全的：万一第一次其实建成功了只是响应丢了，
// 第二次会得到「已存在」，上层正好把用户引到「上传到已有仓库」，
// 不会出现两个仓库，也不会把代码灌进别人的仓库。
func createRemoteRepoOnce(platform string, req PublishRequest) (*createdRepo, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	if platform == PlatformGitHub {
		body, _ := json.Marshal(map[string]interface{}{
			"name":        req.Name,
			"description": req.Description,
			"private":     req.Private,
			"auto_init":   false, // 不要帮我们建 README，否则推送会冲突
		})
		httpReq, err := http.NewRequest("POST", platformAPIBase[PlatformGitHub]+"/user/repos", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+req.Token)
		httpReq.Header.Set("Accept", "application/vnd.github+json")
		httpReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, &transportError{err: fmt.Errorf("连接 GitHub 失败: %w", err)}
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &apiAttemptError{
				status: resp.StatusCode,
				msg:    fmt.Sprintf("GitHub 创建仓库失败（%d）：%s", resp.StatusCode, apiError(raw)),
			}
		}

		var out struct {
			HTMLURL  string `json:"html_url"`
			CloneURL string `json:"clone_url"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("解析 GitHub 返回失败: %w", err)
		}
		return &createdRepo{htmlURL: out.HTMLURL, cloneURL: out.CloneURL, owner: out.Owner.Login, name: out.Name}, nil
	}

	// Gitee
	form := map[string]string{
		"access_token": req.Token,
		"name":         req.Name,
		"description":  req.Description,
		"private":      fmt.Sprintf("%t", req.Private),
		// 不要自动初始化，避免远端已有提交导致推送冲突
		"auto_init": "false",
	}
	vals := make([]string, 0, len(form))
	for k, v := range form {
		vals = append(vals, k+"="+urlQueryEscape(v))
	}
	httpReq, err := http.NewRequest(
		"POST",
		platformAPIBase[PlatformGitee]+"/user/repos",
		strings.NewReader(strings.Join(vals, "&")),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, &transportError{err: fmt.Errorf("连接 Gitee 失败: %w", err)}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiAttemptError{
			status: resp.StatusCode,
			msg:    fmt.Sprintf("Gitee 创建仓库失败（%d）：%s", resp.StatusCode, apiError(raw)),
		}
	}

	var out struct {
		HTMLURL  string `json:"html_url"`
		FullName string `json:"full_name"`
		Name     string `json:"name"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
		Namespace struct {
			Path string `json:"path"`
		} `json:"namespace"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析 Gitee 返回失败: %w", err)
	}

	owner := out.Namespace.Path
	if owner == "" {
		owner = out.Owner.Login
	}
	cloneURL := ""
	if out.HTMLURL != "" {
		cloneURL = strings.TrimSuffix(out.HTMLURL, "/") + ".git"
	}
	return &createdRepo{htmlURL: out.HTMLURL, cloneURL: cloneURL, owner: owner, name: out.Name}, nil
}

// authenticatedURL 拼出带凭据的 HTTPS 地址，仅用于推送，不会提交到仓库里。
func authenticatedURL(platform string, repo *createdRepo, token string) string {
	// 只有 HTTP(S) 才需要把 token 拼进地址：
	//   SSH 走密钥，本地路径 / file:// 压根不需要认证。
	// 往这些地址里塞 token 会得到一个根本连不上的 URL。
	if !strings.HasPrefix(repo.cloneURL, "http://") && !strings.HasPrefix(repo.cloneURL, "https://") {
		return repo.cloneURL
	}

	switch platform {
	case PlatformGitHub:
		// GitHub 用任意用户名 + token 作为密码即可
		return fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", token, repo.owner, repo.name)
	default:
		// Gitee 用 oauth2 作为用户名
		return fmt.Sprintf("https://oauth2:%s@gitee.com/%s/%s.git", token, repo.owner, repo.name)
	}
}

// apiError 从平台返回的 JSON 里提取可读的错误信息。
func apiError(raw []byte) string {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) == nil {
		for _, k := range []string{"message", "error_description", "error"} {
			if v, ok := m[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 300 {
		s = s[:300]
	}
	if s == "" {
		return "（平台没有返回错误详情）"
	}
	return s
}

// redact 把命令里的 token 换成 ***，避免显示或记录到日志里。
func redact(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		if strings.Contains(a, "@") && (strings.HasPrefix(a, "https://") || strings.Contains(a, "token")) {
			// https://user:token@host/... -> https://***@host/...
			if at := strings.LastIndex(a, "@"); at != -1 {
				scheme := "https://"
				if i := strings.Index(a, "://"); i != -1 {
					scheme = a[:i+3]
				}
				out[i] = scheme + "***@" + a[at+1:]
				continue
			}
		}
		out[i] = a
	}
	return out
}

func validRepoName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			return false
		}
	}
	return true
}

func urlQueryEscape(s string) string {
	// 只处理 token / 描述里常见的字符，避免引入额外依赖
	replacer := strings.NewReplacer(
		"%", "%25", "&", "%26", "+", "%2B", "=", "%3D",
		"#", "%23", " ", "%20", "?", "%3F", "/", "%2F",
	)
	return replacer.Replace(s)
}

// gitRun 执行 git 命令并返回合并输出。调用方需持有 e.mu。
func (e *Engine) gitRun(args ...string) (string, error) {
	return e.gitRunWith(nil, args...)
}

// gitRunWith 同上，但可以附加环境变量。
//
// 关键点：不开 PTY，并且默认设 GIT_TERMINAL_PROMPT=0 ——
// 没有终端的桌面应用绝不能让 git 停下来等输入。
func (e *Engine) gitRunWith(extraEnv []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = e.repoPath
	cmd.Env = append(cmd.Environ(), extraEnv...)
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// parseRepoURL 从用户填的仓库地址里解析出用户名和仓库名。
//
// 支持：
//
//	https://gitee.com/user/repo.git
//	https://gitee.com/user/repo
//	git@gitee.com:user/repo.git
func parseRepoURL(raw string) (*createdRepo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("请填写仓库地址")
	}

	owner, name := "", ""

	if strings.HasPrefix(raw, "git@") {
		// git@host:owner/repo.git
		i := strings.Index(raw, ":")
		if i == -1 {
			return nil, fmt.Errorf("SSH 地址格式不对，应该像 git@host:用户名/仓库名.git")
		}
		path := strings.TrimSuffix(strings.Trim(raw[i+1:], "/"), ".git")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("地址里看不出用户名和仓库名")
		}
		owner = parts[len(parts)-2]
		name = parts[len(parts)-1]
	} else {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("地址解析失败: %w", err)
		}
		path := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("地址里看不出用户名和仓库名，应该像 https://gitee.com/用户名/仓库名.git")
		}
		owner = parts[len(parts)-2]
		name = parts[len(parts)-1]
	}

	if owner == "" || name == "" {
		return nil, fmt.Errorf("地址里看不出用户名和仓库名")
	}

	html := ""
	if !strings.HasPrefix(raw, "git@") {
		html = strings.TrimSuffix(strings.TrimSuffix(raw, "/"), ".git")
	}
	return &createdRepo{htmlURL: html, cloneURL: raw, owner: owner, name: name}, nil
}

// isAlreadyExists 判断平台返回的错误是不是「仓库已存在」。
func isAlreadyExists(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"already exist", "already exists", "已存在", "已被使用", "已被占用"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 上次用过的信息（避免每次都重新输入）
// ---------------------------------------------------------------------------

// PublishDefaults 是上传对话框的预填值。
type PublishDefaults struct {
	// RemoteURL 是当前仓库已经配置的远端地址（通常是 origin）。
	// 已经推过一次的仓库，地址直接从这里读，不用用户再填。
	RemoteURL string `json:"remoteUrl"`
	// RemoteName 是读到的远端名（origin / upstream …）
	RemoteName string `json:"remoteName"`
	// RepoName 是仓库目录名，给「新建仓库」时的仓库名做默认值
	RepoName string `json:"repoName"`
}

// PublishDefaults 返回上传对话框可以预填的信息。
func (e *Engine) PublishDefaults() (*PublishDefaults, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	out := &PublishDefaults{RepoName: filepath.Base(e.repoPath)}

	// 先看 origin，没有就取第一个远端
	for _, name := range []string{"origin", ""} {
		args := []string{"remote", "get-url", name}
		if name == "" {
			// remote get-url 不带名字会报错，这里换成列出第一个
			list, err := e.gitOutput("remote")
			if err != nil {
				continue
			}
			names := splitNonEmptyLines(list)
			if len(names) == 0 {
				continue
			}
			args = []string{"remote", "get-url", names[0]}
		}
		url, err := e.gitOutput(args...)
		if err != nil {
			continue
		}
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		// 去掉可能存在的凭据部分，避免把 token 显示到界面上
		out.RemoteURL = stripCredentials(url)
		if name == "" {
			names, _ := e.gitOutput("remote")
			if list := splitNonEmptyLines(names); len(list) > 0 {
				out.RemoteName = list[0]
			}
		} else {
			out.RemoteName = name
		}
		return out, nil
	}

	return out, nil
}

// stripCredentials 去掉 URL 里的 用户名:密码@ 部分。
func stripCredentials(raw string) string {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return raw
	}
	schemeEnd := strings.Index(raw, "://") + 3
	rest := raw[schemeEnd:]
	at := strings.LastIndex(rest, "@")
	if at == -1 {
		return raw
	}
	return raw[:schemeEnd] + rest[at+1:]
}
