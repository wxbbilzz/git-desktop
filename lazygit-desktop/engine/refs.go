package engine

import (
	"strconv"
	"strings"
)

// 本文件读取「分支之外的那些引用」：标签、远端、远端分支。
//
// lazygit 的 BranchLoader 只读 refs/heads（本地分支），远端分支和标签它分别
// 由别的 loader 负责。这里直接用 for-each-ref 取，字段由我们自己定，
// 反而比拟合它的模型更省事。

// TagDTO 是一个标签。
type TagDTO struct {
	Name      string `json:"name"`
	Hash      string `json:"hash"`
	ShortHash string `json:"shortHash"`
	Subject   string `json:"subject"`
	When      string `json:"when"`
	// IsAnnotated 区分附注标签（git tag -a）和轻量标签
	IsAnnotated bool `json:"isAnnotated"`
}

// RemoteBranchDTO 是一个远端分支（refs/remotes/...）。
type RemoteBranchDTO struct {
	Name      string `json:"name"`  // origin/main
	Short     string `json:"short"` // main
	Remote    string `json:"remote"`
	Hash      string `json:"hash"`
	ShortHash string `json:"shortHash"`
	Subject   string `json:"subject"`
	When      string `json:"when"`
	// IsCurrentUpstream 表示这就是当前分支跟踪的那个远端分支
	IsCurrentUpstream bool `json:"isCurrentUpstream"`
	// HasLocal 表示本地已经有同名分支了（此时不需要再「检出为新分支」）
	HasLocal bool `json:"hasLocal"`
}

// RemoteDTO 是一个远端。
type RemoteDTO struct {
	Name string `json:"name"`
	// URL 已经剥掉凭据，可以安全地显示在界面上
	URL     string `json:"url"`
	PushURL string `json:"pushUrl"`
}

// sep 是 for-each-ref 的字段分隔符。
// 用 \x00 而不是空格，因为提交说明里什么字符都可能有。
const refFieldSep = "%00"

// loadTagsLocked 读取全部标签。
func (e *Engine) loadTagsLocked() []TagDTO {
	// creatordate 对轻量标签和附注标签都有效；*objectname 是附注标签指向的提交
	out, err := e.gitOutput("for-each-ref", "refs/tags",
		"--sort=-creatordate",
		"--format=%(refname:short)"+refFieldSep+
			"%(objectname)"+refFieldSep+
			"%(objecttype)"+refFieldSep+
			"%(*objectname)"+refFieldSep+
			"%(subject)"+refFieldSep+
			"%(creatordate:unix)",
	)
	if err != nil {
		return []TagDTO{}
	}

	tags := []TagDTO{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) < 6 {
			continue
		}
		name := f[0]
		hash := f[1]
		annotated := f[2] == "tag"
		// 附注标签的 objectname 是标签对象本身，指向的提交在 *objectname
		if f[3] != "" {
			hash = f[3]
		}
		ts, _ := strconv.ParseInt(f[5], 10, 64)

		tags = append(tags, TagDTO{
			Name:        name,
			Hash:        hash,
			ShortHash:   shortHash(hash),
			Subject:     f[4],
			When:        relativeTime(ts),
			IsAnnotated: annotated,
		})
	}
	return tags
}

// loadRemoteBranchesLocked 读取全部远端分支。
func (e *Engine) loadRemoteBranchesLocked() []RemoteBranchDTO {
	out, err := e.gitOutput("for-each-ref", "refs/remotes",
		"--sort=-committerdate",
		"--format=%(refname:short)"+refFieldSep+
			"%(objectname)"+refFieldSep+
			"%(subject)"+refFieldSep+
			"%(committerdate:unix)",
	)
	if err != nil {
		return []RemoteBranchDTO{}
	}

	// 本地已有分支的名字，用来标记「已经有同名本地分支」
	local := map[string]bool{}
	for _, b := range e.loadBranchesLocked() {
		local[b.Name] = true
	}

	upstream := ""
	if b, err := e.git.Branch.CurrentBranchName(); err == nil && b != "" {
		if s, err := e.gitOutput("rev-parse", "--abbrev-ref", "--symbolic-full-name", b+"@{u}"); err == nil {
			upstream = strings.TrimSpace(s)
		}
	}

	branches := []RemoteBranchDTO{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) < 4 {
			continue
		}
		name := f[0]
		// refs/remotes/origin/HEAD 是个符号引用，指向默认分支，
		// 显示出来只会让人困惑（它和 origin/main 是同一个提交）
		if strings.HasSuffix(name, "/HEAD") {
			continue
		}
		remote, short := splitRemoteBranch(name)
		if remote == "" {
			continue
		}
		ts, _ := strconv.ParseInt(f[3], 10, 64)

		branches = append(branches, RemoteBranchDTO{
			Name:              name,
			Short:             short,
			Remote:            remote,
			Hash:              f[1],
			ShortHash:         shortHash(f[1]),
			Subject:           f[2],
			When:              relativeTime(ts),
			IsCurrentUpstream: name == upstream,
			HasLocal:          local[short],
		})
	}
	return branches
}

// loadRemotesLocked 读取全部远端（名字 + 地址）。
func (e *Engine) loadRemotesLocked() []RemoteDTO {
	out, err := e.gitOutput("remote", "-v")
	if err != nil {
		return []RemoteDTO{}
	}

	type entry struct{ fetch, push string }
	byName := map[string]*entry{}
	order := []string{}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		// 格式：<name>\t<url> (fetch|push)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name, url := parts[0], parts[1]
		kind := parts[len(parts)-1]

		it, ok := byName[name]
		if !ok {
			it = &entry{}
			byName[name] = it
			order = append(order, name)
		}
		// 地址里可能带 token（用户勾过「记住凭据」），不能显示到界面上
		clean := stripCredentials(url)
		if strings.HasPrefix(kind, "(push)") {
			it.push = clean
		} else {
			it.fetch = clean
		}
	}

	remotes := []RemoteDTO{}
	for _, name := range order {
		it := byName[name]
		remotes = append(remotes, RemoteDTO{Name: name, URL: it.fetch, PushURL: it.push})
	}
	return remotes
}

// splitRemoteBranch 把 "origin/feature/x" 拆成 ("origin", "feature/x")。
//
// 不能简单按第一个 / 切然后当分支名完事 —— 远端名本身也可能不含 /，
// 但分支名里一定有；所以按第一个 / 切是对的。
func splitRemoteBranch(name string) (remote, branch string) {
	i := strings.Index(name, "/")
	if i <= 0 || i == len(name)-1 {
		return "", ""
	}
	return name[:i], name[i+1:]
}

// shortHash 取短哈希，保留给 DTO 用。
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
