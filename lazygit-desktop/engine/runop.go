package engine

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// 本文件是「通用执行器」：把目录里描述的操作真正跑成 git 命令。
//
// 有了它，新增一个 git 命令只需要往 catalog.go 加一条数据，
// 不需要写任何执行代码，前端也会自动出现对应界面。

// RunResult 是一次操作的执行结果，返回给界面展示。
type RunResult struct {
	OperationID string `json:"operationId"`
	// Command 是实际执行的命令，显示给用户以便学习和复制
	Command string `json:"command"`
	// Output 是 git 的合并输出（stdout + stderr）
	Output string `json:"output"`
	OK     bool   `json:"ok"`
	Error  string `json:"error"`
	// Snapshot 是执行完后的仓库状态，界面直接用它刷新
	Snapshot *RepoSnapshot `json:"snapshot"`
}

// RunOperation 执行目录中的一个操作。
func (e *Engine) RunOperation(id string, args map[string]string) (*RunResult, error) {
	op, ok := OperationByID(id)
	if !ok {
		return nil, fmt.Errorf("未知操作: %s", id)
	}

	argv, err := op.BuildArgs(args)
	if err != nil {
		return nil, err
	}

	return e.runGitArgv(op.ID, argv)
}

// RunRawGit 直接执行任意 git 命令。
//
// 这是「覆盖所有命令」的兜底入口：目录里收录的是常用、需要表单的操作，
// 而 git 的 159 个命令（尤其是 plumbing 底层命令）不可能全部手写表单，
// 用这个入口可以执行任何 git 子命令及其参数。
func (e *Engine) RunRawGit(args []string) (*RunResult, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("命令不能为空")
	}
	return e.runGitArgv("raw", args)
}

// runGitArgv 是所有 git 执行的唯一出口。
//
// 设计要点：
//   - 用 argv 形式调用，不经过 shell，所以参数里的特殊字符不会被解释（无注入风险）
//   - 显式设置工作目录，不依赖进程 cwd
//   - GIT_TERMINAL_PROMPT=0：没有终端，绝不能让 git 卡在凭据提示上
//   - 无论成功失败都返回 RunResult，让界面能把 git 的原始输出完整展示出来
func (e *Engine) runGitArgv(opID string, argv []string) (*RunResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	cmd := exec.Command("git", argv...)
	cmd.Dir = e.repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	out, runErr := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))

	res := &RunResult{
		OperationID: opID,
		Command:     "git " + strings.Join(argv, " "),
		Output:      text,
		OK:          runErr == nil,
	}
	if runErr != nil {
		if text != "" {
			// git 的报错信息比 "exit status 1" 有用得多
			res.Error = firstErrorLine(text)
		} else {
			res.Error = runErr.Error()
		}
	}

	// 命令执行完刷新一次快照。只读命令也刷新，这样界面状态永远和仓库一致。
	if snap, err := e.snapshotLocked(); err == nil {
		res.Snapshot = snap
	}

	return res, nil
}

// firstErrorLine 从 git 输出里挑一行最有信息量的作为错误摘要。
func firstErrorLine(text string) string {
	lines := strings.Split(text, "\n")

	// 先找被拒绝的引用那一行，例如：
	//	! [被拒绝]  main -> main (fetch first)
	// 它说清了「哪个分支、为什么被拒」；紧跟其后的
	// "error: 无法推送一些引用" 只是套话，拿它当摘要把原因丢了。
	for _, l := range lines {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "! [") {
			return l
		}
	}

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// 其次返回带 fatal/error 的行
		if strings.HasPrefix(l, "fatal:") || strings.HasPrefix(l, "error:") {
			return l
		}
	}
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return text
}

// ---------------------------------------------------------------- 界面用的查询

// OperationSummary 是发给前端的操作概要（带分类，便于分组渲染）。
type OperationSummary struct {
	ID          string  `json:"id"`
	Category    string  `json:"category"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Params      []Param `json:"params"`
	Dangerous   bool    `json:"dangerous"`
	ReadOnly    bool    `json:"readOnly"`
}

// Operations 返回全部操作，供前端渲染操作面板。
func (e *Engine) Operations() []OperationSummary {
	ops := Catalog()
	out := make([]OperationSummary, 0, len(ops))
	for _, op := range ops {
		params := op.Params
		if params == nil {
			params = []Param{}
		}
		out = append(out, OperationSummary{
			ID:          op.ID,
			Category:    op.Category,
			Name:        op.Name,
			Description: op.Description,
			Params:      params,
			Dangerous:   op.Dangerous,
			ReadOnly:    op.ReadOnly,
		})
	}
	return out
}

// SplitCommandLine 把一行命令切成参数切片，支持单引号和双引号。
//
// 用于「原始命令」输入框，例如：
//
//	log --oneline -n 20 "src/my file.go"
//
// 会切成 ["log","--oneline","-n","20","src/my file.go"]。
func SplitCommandLine(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	inToken := false

	flush := func() {
		if inToken {
			out = append(out, cur.String())
			cur.Reset()
			inToken = false
		}
	}

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
			inToken = true
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	flush()
	return out
}

// ---------------------------------------------------------------- 下拉候选值

// RefOption 是下拉里的一个选项。
type RefOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// OperationChoices 一次性返回所有下拉需要的候选值。
//
// 放在一个方法里返回，是为了避免「每个参数发一次 IPC」——
// 打开操作面板时只调一次就够。
type OperationChoices struct {
	Branches []RefOption `json:"branches"`
	Refs     []RefOption `json:"refs"`
	Commits  []RefOption `json:"commits"`
	Files    []RefOption `json:"files"`
	Remotes  []RefOption `json:"remotes"`
	Tags     []RefOption `json:"tags"`
	Stashes  []RefOption `json:"stashes"`
}

// OperationChoices 返回所有下拉参数的候选值。
func (e *Engine) OperationChoices() (*OperationChoices, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	out := &OperationChoices{
		Branches: []RefOption{}, Refs: []RefOption{}, Commits: []RefOption{},
		Files: []RefOption{}, Remotes: []RefOption{}, Tags: []RefOption{},
		Stashes: []RefOption{},
	}

	// 本地分支
	if s, err := e.gitOutput("branch", "--format=%(refname:short)"); err == nil {
		for _, name := range splitNonEmptyLines(s) {
			out.Branches = append(out.Branches, RefOption{Value: name, Label: name})
		}
	}

	// 标签（最近的在前）
	if s, err := e.gitOutput("tag", "-l", "--sort=-creatordate"); err == nil {
		for _, name := range splitNonEmptyLines(s) {
			out.Tags = append(out.Tags, RefOption{Value: name, Label: name})
		}
	}

	// 远端
	if s, err := e.gitOutput("remote"); err == nil {
		for _, name := range splitNonEmptyLines(s) {
			out.Remotes = append(out.Remotes, RefOption{Value: name, Label: name})
		}
	}

	// 最近提交：值用哈希，标签显示「短哈希 + 提交说明」
	if s, err := e.gitOutput("log", "--oneline", "-n", "100", "--no-decorate"); err == nil {
		for _, line := range splitNonEmptyLines(s) {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 0 {
				continue
			}
			hash := parts[0]
			subject := ""
			if len(parts) > 1 {
				subject = parts[1]
			}
			out.Commits = append(out.Commits, RefOption{Value: hash, Label: line})
			_ = subject
		}
	}

	// 仓库文件（下拉里最多放前 500 个，太多反而难选）
	if s, err := e.gitOutput("ls-files", "--cached", "--others", "--exclude-standard"); err == nil {
		for i, name := range splitNonEmptyLines(s) {
			if i >= 500 {
				break
			}
			out.Files = append(out.Files, RefOption{Value: name, Label: name})
		}
	}

	// 储藏记录
	if s, err := e.gitOutput("stash", "list"); err == nil {
		for _, e2 := range parseStashList(s) {
			out.Stashes = append(out.Stashes, RefOption{
				Value: strconv.Itoa(e2.Index),
				Label: fmt.Sprintf("%s  %s", e2.Ref, e2.Message),
			})
		}
	}

	// 「分支 / 标签 / 提交都行」的场景：分支 + 常用特殊写法 + 标签
	out.Refs = append(out.Refs, RefOption{Value: "HEAD", Label: "HEAD（当前提交）"})
	out.Refs = append(out.Refs, RefOption{Value: "HEAD~1", Label: "HEAD~1（上一个提交）"})
	out.Refs = append(out.Refs, out.Branches...)
	for _, t := range out.Tags {
		out.Refs = append(out.Refs, RefOption{Value: t.Value, Label: "标签 " + t.Label})
	}
	if len(out.Commits) > 20 {
		out.Refs = append(out.Refs, out.Commits[:20]...)
	} else {
		out.Refs = append(out.Refs, out.Commits...)
	}

	return out, nil
}

// splitNonEmptyLines 按行切分并去掉空行与首尾空白。
func splitNonEmptyLines(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
