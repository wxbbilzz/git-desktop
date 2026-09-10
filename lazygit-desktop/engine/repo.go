package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// 本文件负责「仓库的诞生」：新建 与 克隆。
//
// 这两个操作有一个共同点：它们都发生在仓库还不存在的时候，
// 所以没法走 lazygit 的 git_commands（那套东西的前提是已经在一个仓库里、
// repoPaths 已经就绪）。
//
// lazygit 自己也是这么处理的：pkg/app/app.go 里遇到「不在仓库中」时，
// 直接用 os/exec 跑一条 `git init`。这里沿用同样的思路。
//
// 好处是我们可以完全掌控 clone 的 stderr —— git 的进度输出走的是 stderr，
// 而 lazygit 的 RunAndProcessLines 只读 stdout，拿不到进度。

// CloneProgress 是克隆过程中实时推送给前端的一条进度。
type CloneProgress struct {
	Phase   string `json:"phase"`   // 中文阶段名，例如「接收对象」
	Percent int    `json:"percent"` // 0-100；未知时为 -1
	Detail  string `json:"detail"`  // git 的原始输出行，便于排查问题
}

// git 的进度行长这样：
//
//	Receiving objects:  45% (123/273), 1.2 MiB | 3.4 MiB/s
//	Resolving deltas:  80% (100/125)
var progressRe = regexp.MustCompile(`^([A-Za-z][A-Za-z ]+?):\s+(\d+)%`)

// git 的阶段名是英文，这里转成中文让界面更友好。
var phaseLabels = map[string]string{
	"Enumerating objects": "枚举对象",
	"Counting objects":    "统计对象",
	"Compressing objects": "压缩对象",
	"Receiving objects":   "接收对象",
	"Resolving deltas":    "解析差异",
	"Updating files":      "更新文件",
	"Checking out files":  "检出文件",
	"remote":              "远端",
}

// ---------------------------------------------------------------------------
// 新建仓库
// ---------------------------------------------------------------------------

// InitRepo 在 parentDir 下新建名为 name 的仓库，并立刻打开它。
func (e *Engine) InitRepo(parentDir, name, initialBranch string) (*RepoSnapshot, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("仓库名不能为空")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return nil, fmt.Errorf("仓库名不能包含路径分隔符")
	}

	parentDir = strings.TrimSpace(parentDir)
	if parentDir == "" {
		return nil, fmt.Errorf("请选择存放位置")
	}

	dir := filepath.Join(parentDir, name)

	// 目录可以不存在（我们建），也可以存在但必须为空。
	// 不允许对着一个已有内容的目录执行 init，避免把用户的东西弄得莫名其妙。
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("目录已存在且不为空：%s", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	args := []string{"init"}
	if b := strings.TrimSpace(initialBranch); b != "" {
		args = append(args, "--initial-branch="+b)
	}
	args = append(args, dir)

	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git init 失败: %v\n%s", err, strings.TrimSpace(string(out)))
	}

	return e.OpenRepo(dir)
}

// ---------------------------------------------------------------------------
// 克隆仓库
// ---------------------------------------------------------------------------

// CloneRepo 把 url 克隆到 dest，并立刻打开它。
//
// depth > 0 时做浅克隆（只取最近 depth 次提交），大仓库用它可以快很多。
//
// onProgress 会在克隆过程中被反复调用（可能来自其他 goroutine），
// 用来把进度推给界面。传 nil 表示不关心进度。
func (e *Engine) CloneRepo(url, dest string, depth int, onProgress func(CloneProgress)) (*RepoSnapshot, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("仓库地址不能为空")
	}
	// 防止地址被当成命令行选项
	if strings.HasPrefix(url, "-") {
		return nil, fmt.Errorf("仓库地址不合法")
	}

	dest = strings.TrimSpace(dest)
	if dest == "" {
		return nil, fmt.Errorf("请选择保存位置")
	}

	if entries, err := os.ReadDir(dest); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("目标目录已存在且不为空：%s", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, fmt.Errorf("创建上级目录失败: %w", err)
	}

	args := []string{"clone", "--progress"}
	if depth > 0 {
		args = append(args, fmt.Sprintf("--depth=%d", depth))
	}
	args = append(args, "--", url, dest)

	cmd := exec.Command("git", args...)
	// 没有终端，绝不能让 git 卡在凭据提示上等输入；
	// 配了 credential helper 或走 SSH key 的情况不受影响。
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("无法启动 git: %w", err)
	}

	// git 的进度是用 \r 原地刷新同一行的，所以不能只按 \n 切分。
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(splitOnCRLF)

	var lastLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lastLine = line
		if onProgress != nil {
			onProgress(parseCloneProgress(line))
		}
	}

	if err := cmd.Wait(); err != nil {
		if lastLine != "" {
			return nil, fmt.Errorf("克隆失败: %s", lastLine)
		}
		return nil, fmt.Errorf("克隆失败: %w", err)
	}

	return e.OpenRepo(dest)
}

// splitOnCRLF 让 Scanner 同时以 \n 和 \r 作为分隔符。
func splitOnCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' || data[i] == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func parseCloneProgress(line string) CloneProgress {
	p := CloneProgress{Percent: -1, Detail: line}

	if m := progressRe.FindStringSubmatch(line); m != nil {
		raw := strings.TrimSpace(m[1])
		if label, ok := phaseLabels[raw]; ok {
			p.Phase = label
		} else {
			p.Phase = raw
		}
		if n, err := strconv.Atoi(m[2]); err == nil {
			p.Percent = n
		}
		return p
	}

	// 一些没有百分比的提示行
	switch {
	case strings.HasPrefix(line, "Cloning into"):
		p.Phase = "初始化"
	case strings.Contains(line, "done."):
		p.Phase = "完成"
	}
	return p
}

// ---------------------------------------------------------------------------
// 工具
// ---------------------------------------------------------------------------

// DefaultBaseDir 返回一个适合作为「存放位置」默认值的目录。
func DefaultBaseDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// DeriveRepoName 从仓库地址推断出目录名，供界面预填。
//
//	https://github.com/user/repo.git   -> repo
//	git@github.com:user/repo.git       -> repo
//	https://gitee.com/user/repo        -> repo
func DeriveRepoName(url string) string {
	s := strings.TrimSpace(url)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	if s == "" {
		return ""
	}
	// ssh 形式 git@host:user/repo
	if i := strings.LastIndex(s, ":"); i != -1 && !strings.Contains(s[i:], "/") {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "/"); i != -1 {
		s = s[i+1:]
	}
	return s
}
