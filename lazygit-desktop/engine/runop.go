package engine

import (
	"fmt"
	"os"
	"os/exec"
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
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// 优先返回带 fatal/error/hint 的行
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
