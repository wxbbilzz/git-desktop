package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 本文件实现合并冲突的解析与解决。
//
// lazygit 的 pkg/gui/mergeconflicts 有一份现成的实现，但它的 State
// 是为 TUI 交互设计的（内部维护选择下标、撤销栈，且枚举可选分支的方法是
// 未导出的）。对 GUI 来说，把冲突解析成结构化数据、由界面直接操作更合适，
// 所以这里自己实现解析（算法思路参考了 lazygit）：
//   - 支持自定义 conflict-marker-size（git 属性）
//   - 支持 diff3 风格的 ||||||| 基准段
//   - 保留 <<<<<<< 后面的标签（通常是分支名）

// ConflictBlock 是文件里的一处冲突。
type ConflictBlock struct {
	Index int `json:"index"`
	// StartLine / EndLine 是 0 基的行号，分别是 <<<<<<< 和 >>>>>>> 所在行
	StartLine int `json:"startLine"`
	EndLine   int `json:"endLine"`

	Ours   []string `json:"ours"`
	Theirs []string `json:"theirs"`
	Base   []string `json:"base"`
	// 是否是 diff3 风格（有基准段）
	HasBase bool `json:"hasBase"`

	LabelOurs   string `json:"labelOurs"`
	LabelTheirs string `json:"labelTheirs"`
}

// ConflictFile 是一个有待解决冲突的文件。
type ConflictFile struct {
	Path       string          `json:"path"`
	Lines      []string        `json:"lines"`
	Blocks     []ConflictBlock `json:"blocks"`
	MarkerSize int             `json:"markerSize"`
}

// ConflictChoice 是界面为某一处冲突做出的选择。
type ConflictChoice struct {
	BlockIndex int    `json:"blockIndex"`
	Choice     string `json:"choice"` // ours / theirs / both / base
}

// ReadConflictFile 读取并解析一个冲突文件。
func (e *Engine) ReadConflictFile(path string) (*ConflictFile, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	file, err := e.findFileLocked(path)
	if err != nil {
		return nil, err
	}

	markerSize := file.ConflictMarkerSize
	if markerSize <= 0 {
		markerSize = 7 // git 默认
	}

	content, err := e.readRepoFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(content, "\n")
	blocks := parseConflictBlocks(lines, markerSize)

	return &ConflictFile{
		Path:       path,
		Lines:      lines,
		Blocks:     blocks,
		MarkerSize: markerSize,
	}, nil
}

// ResolveConflicts 按界面的选择解决冲突，并把结果写回文件。
//
// 只处理 choices 里出现过的冲突块；没选的块保持原样（仍然冲突）。
// 全部解决完后文件里就没有冲突标记了，界面可以据此提示「标记为已解决」。
func (e *Engine) ResolveConflicts(path string, choices []ConflictChoice) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	file, err := e.findFileLocked(path)
	if err != nil {
		return nil, err
	}
	markerSize := file.ConflictMarkerSize
	if markerSize <= 0 {
		markerSize = 7
	}

	content, err := e.readRepoFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(content, "\n")
	blocks := parseConflictBlocks(lines, markerSize)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("这个文件里没有找到冲突标记")
	}

	// 把界面选择整理成 map
	want := map[int]string{}
	for _, c := range choices {
		want[c.BlockIndex] = c.Choice
	}

	out := []string{}
	cursor := 0
	for _, b := range blocks {
		// 冲突块之前的原样内容
		out = append(out, lines[cursor:b.StartLine]...)

		choice, ok := want[b.Index]
		if !ok {
			// 没选：原样保留
			out = append(out, lines[b.StartLine:b.EndLine+1]...)
		} else {
			switch choice {
			case "ours":
				out = append(out, b.Ours...)
			case "theirs":
				out = append(out, b.Theirs...)
			case "base":
				out = append(out, b.Base...)
			case "both":
				out = append(out, b.Ours...)
				out = append(out, b.Theirs...)
			default:
				return nil, fmt.Errorf("未知的选择：%s", choice)
			}
		}
		cursor = b.EndLine + 1
	}
	out = append(out, lines[cursor:]...)

	if err := e.writeRepoFile(path, strings.Join(out, "\n")); err != nil {
		return nil, err
	}

	return e.snapshotLocked()
}

// RefreshConflictFile 重新读取冲突文件（界面在解决一处后刷新用）。
func (e *Engine) RefreshConflictFile(path string) (*ConflictFile, error) {
	return e.ReadConflictFile(path)
}

// ---------------------------------------------------------------- 内部

// parseConflictBlocks 扫描文件内容，找出所有冲突块。
func parseConflictBlocks(lines []string, markerSize int) []ConflictBlock {
	blocks := []ConflictBlock{}

	for i := 0; i < len(lines); i++ {
		if !hasMarker(lines[i], '<', markerSize) {
			continue
		}

		start := i
		labelOurs := markerLabel(lines[i], '<')

		var ours, base, theirs []string

		j := i + 1
		// 收集「我方」内容，直到 ======= 或 |||||||
		for j < len(lines) && !hasMarker(lines[j], '=', markerSize) && !hasMarker(lines[j], '|', markerSize) {
			ours = append(ours, lines[j])
			j++
		}

		hasBase := false
		// diff3 风格：多一段基准内容
		if j < len(lines) && hasMarker(lines[j], '|', markerSize) {
			hasBase = true
			j++
			for j < len(lines) && !hasMarker(lines[j], '=', markerSize) {
				base = append(base, lines[j])
				j++
			}
		}

		if j >= len(lines) || !hasMarker(lines[j], '=', markerSize) {
			continue // 标记不完整，不是有效冲突块
		}
		j++ // 跳过 =======

		for j < len(lines) && !hasMarker(lines[j], '>', markerSize) {
			theirs = append(theirs, lines[j])
			j++
		}
		if j >= len(lines) {
			continue
		}

		blocks = append(blocks, ConflictBlock{
			Index:       len(blocks),
			StartLine:   start,
			EndLine:     j,
			Ours:        ours,
			Theirs:      theirs,
			Base:        base,
			HasBase:     hasBase,
			LabelOurs:   labelOurs,
			LabelTheirs: markerLabel(lines[j], '>'),
		})
		i = j
	}

	return blocks
}

// hasMarker 判断一行是否以至少 size 个 ch 开头（也就是冲突标记）。
func hasMarker(line string, ch byte, size int) bool {
	if len(line) < size {
		return false
	}
	for i := 0; i < size; i++ {
		if line[i] != ch {
			return false
		}
	}
	return true
}

// markerLabel 取出标记后面的文字，例如 "HEAD" / "feature-x"。
func markerLabel(line string, ch byte) string {
	i := 0
	for i < len(line) && line[i] == ch {
		i++
	}
	return strings.TrimSpace(line[i:])
}

// readRepoFile 读取仓库里的文件，并防止路径穿越。
func (e *Engine) readRepoFile(path string) (string, error) {
	full, err := e.safeRepoPath(path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}
	return string(b), nil
}

// writeRepoFile 写回仓库里的文件。
func (e *Engine) writeRepoFile(path string, content string) error {
	full, err := e.safeRepoPath(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

// safeRepoPath 把仓库内相对路径转成绝对路径，并拒绝越界路径。
func (e *Engine) safeRepoPath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法文件路径: %s", path)
	}
	full := filepath.Join(e.repoPath, clean)
	// 再确认一次没有跑到仓库外面
	rel, err := filepath.Rel(e.repoPath, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("非法文件路径: %s", path)
	}
	return full, nil
}
