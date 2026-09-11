package engine

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// 本文件实现「按文件查看某个提交」。
//
// 之前查看提交是把整个 diff 一次显示出来，改动多的时候很难看。
// 这里先取提交涉及的文件列表，再按需取单个文件的 diff，
// 界面就能做成「左边文件列表 + 右边该文件的改动」。

// CommitFileDTO 是某个提交里改动的一个文件。
type CommitFileDTO struct {
	Path        string `json:"path"`
	OldPath     string `json:"oldPath"` // 重命名时的原路径
	Status      string `json:"status"`  // A / M / D / R / C ...
	StatusLabel string `json:"statusLabel"`
	Kind        string `json:"kind"` // new / modified / deleted / renamed / copied
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
}

// CommitFiles 列出某个提交改动的文件（含增删行数）。
func (e *Engine) CommitFiles(hash string) ([]CommitFileDTO, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, fmt.Errorf("提交标识不能为空")
	}

	// 合并提交通常没有可展示的差异：git 对合并提交用的是 combined diff，
	// 一次干净的合并会输出空。这时改用「与第一个父提交的差异」，
	// 也就是「这次合并把什么带了进来」，这才是用户想看的。
	// 注意：这里必须把 hash 拼进去，否则 git 会默认用 HEAD
	// （曾经漏过一次，导致查任意提交都返回 HEAD 的文件）
	baseArgs := []string{"show", "--format=", hash}
	if e.isMergeCommit(hash) {
		baseArgs = []string{"diff", hash + "^1", hash}
	}

	statusArgs := append(append([]string{}, baseArgs...), "--name-status")
	statusOut, err := e.gitOutput(statusArgs...)
	if err != nil {
		return nil, err
	}
	files := parseNameStatus(statusOut)

	numArgs := append(append([]string{}, baseArgs...), "--numstat")
	if numOut, err := e.gitOutput(numArgs...); err == nil {
		applyNumstat(files, numOut)
	}

	return files, nil
}

// CommitFileDiff 取某个提交里单个文件的 diff 文本。
//
// 复用 lazygit 的 ShowCmdObj：它会带上用户配置的 diff renderer 参数，
// 并且支持按路径过滤（这样重命名也能正确显示）。
func (e *Engine) CommitFileDiff(hash string, path string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return "", err
	}
	hash = strings.TrimSpace(hash)
	path = strings.TrimSpace(path)
	if hash == "" || path == "" {
		return "", fmt.Errorf("提交标识和文件路径都不能为空")
	}

	var args []string
	if e.isMergeCommit(hash) {
		// 同 CommitFiles：合并提交用与第一个父提交的差异
		args = []string{"diff", hash + "^1", hash, "--", path}
	} else {
		// --format= 去掉提交头信息，只看这个文件的改动
		args = []string{"show", "--format=", "--patch", hash, "--", path}
	}
	out, err := e.gitOutput(args...)
	if err != nil {
		return "", err
	}
	return out, nil
}

// isMergeCommit 判断某个提交是不是合并提交（有多个父提交）。
// 调用方需持有 e.mu。
func (e *Engine) isMergeCommit(hash string) bool {
	out, err := e.gitOutput("rev-list", "--parents", "-n", "1", hash)
	if err != nil {
		return false
	}
	// 输出形如 "<hash> <parent1> <parent2> ..."，字段数 > 2 就是合并
	return len(strings.Fields(strings.TrimSpace(out))) > 2
}

// ---------------------------------------------------------------- 内部工具

// gitOutput 执行一条 git 命令并返回 stdout。调用方需持有 e.mu。
func (e *Engine) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = e.repoPath
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s 失败: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

// parseNameStatus 解析 `git show --name-status` 的输出。
func parseNameStatus(out string) []CommitFileDTO {
	files := []CommitFileDTO{}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		status := strings.TrimSpace(parts[0])
		f := CommitFileDTO{Status: status}

		// 重命名 / 复制是三段：状态、原路径、新路径
		if (strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C")) && len(parts) >= 3 {
			f.OldPath = parts[1]
			f.Path = parts[2]
		} else {
			f.Path = parts[1]
		}

		f.Kind, f.StatusLabel = describeStatus(status)
		files = append(files, f)
	}
	return files
}

// applyNumstat 把 `git show --numstat` 的增删行数填进文件列表。
func applyNumstat(files []CommitFileDTO, out string) {
	byPath := map[string]int{}
	for i := range files {
		byPath[files[i].Path] = i
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		add, errA := strconv.Atoi(strings.TrimSpace(parts[0]))
		del, errD := strconv.Atoi(strings.TrimSpace(parts[1]))

		path := strings.TrimSpace(parts[2])
		idx, ok := byPath[path]
		if !ok {
			for i := range files {
				if files[i].Path != "" && strings.Contains(path, files[i].Path) {
					idx, ok = i, true
					break
				}
			}
		}
		if !ok {
			continue
		}
		if errA == nil {
			files[idx].Additions = add
		}
		if errD == nil {
			files[idx].Deletions = del
		}
	}
}

// describeStatus 把 git 的状态字母翻译成界面用的分类和中文标签。
func describeStatus(status string) (string, string) {
	if status == "" {
		return "modified", "修改"
	}
	switch status[0] {
	case 'A':
		return "new", "新增"
	case 'D':
		return "deleted", "删除"
	case 'R':
		return "renamed", "重命名"
	case 'C':
		return "copied", "复制"
	case 'T':
		return "modified", "类型变更"
	case 'U':
		return "conflict", "冲突"
	default:
		return "modified", "修改"
	}
}
