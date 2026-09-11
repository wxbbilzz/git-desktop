package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/git_commands"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
)

// 本文件实现「行级暂存」：只把文件里的某几行放进暂存区。
//
// 核心能力全部复用 lazygit 的 pkg/commands/patch：
//   - patch.Parse     把 diff 解析成结构化的 Patch
//   - Patch.Transform 挑出选中的行，重新生成一个可 apply 的子 patch
//   - PatchCommands.ApplyPatch  用 `git apply --cached` 写进索引
//
// 这个包几乎没有 UI 耦合（theme/style 只用在 TUI 的彩色输出里，
// 我们只调 FormatPlain 不调 FormatView），所以可以直接用。

// PatchLineDTO 是 patch 里的一行，带稳定的下标供前端勾选。
type PatchLineDTO struct {
	Index int    `json:"index"`
	Kind  string `json:"kind"` // header / hunk / addition / deletion / context / other
	Text  string `json:"text"` // 去掉首字符标记（+/-/空格）后的内容
	// 原始首字符，前端用它着色
	Marker string `json:"marker"`
	OldNo  int    `json:"oldNo"`
	NewNo  int    `json:"newNo"`
	// 是否是可勾选的变更行（只有增/删行能单独选）
	Selectable bool `json:"selectable"`
}

// FilePatch 是某个文件 diff 的结构化表示。
type FilePatch struct {
	Path   string         `json:"path"`
	Staged bool           `json:"staged"`
	Lines  []PatchLineDTO `json:"lines"`
	// 没有任何可用于暂存的变更行时，前端应禁用行级暂存
	HasChanges bool `json:"hasChanges"`
}

// FilePatchLines 取某个文件 diff 的结构化行，供界面渲染和勾选。
func (e *Engine) FilePatchLines(path string, staged bool) (*FilePatch, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	file, err := e.findFileLocked(path)
	if err != nil {
		return nil, err
	}

	diffStr := e.git.WorkingTree.WorktreeFileDiff(file, true, staged)
	return buildFilePatch(path, staged, diffStr), nil
}

// StageLines 把选中的行暂存或取消暂存。
//
// staged=true 表示这些行当前在暂存区里，要「取消暂存」（反向应用）。
func (e *Engine) StageLines(path string, staged bool, lineIndices []int) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if len(lineIndices) == 0 {
		return nil, fmt.Errorf("没有选中任何行")
	}

	file, err := e.findFileLocked(path)
	if err != nil {
		return nil, err
	}

	// 关键：diff 的方向必须和操作对应
	//  - 暂存未暂存的改动：看工作区 diff（cached=false），正向应用
	//  - 取消已暂存的改动：看暂存区 diff（cached=true），反向应用
	diffStr := e.git.WorkingTree.WorktreeFileDiff(file, true, staged)

	p := patch.Parse(diffStr)
	if len(p.Lines()) == 0 {
		return nil, fmt.Errorf("这个文件没有可操作的改动")
	}

	sub := p.Transform(patch.TransformOpts{
		IncludedLineIndices: lineIndices,
		Reverse:             staged,
	})

	patchText := sub.FormatPlain()
	if strings.TrimSpace(patchText) == "" {
		return nil, fmt.Errorf("选中的行无法构成有效的补丁")
	}

	// Cached=true 让 git 把结果写进索引（暂存区）而不是工作区
	err = e.git.Patch.ApplyPatch(patchText, git_commands.ApplyPatchOpts{
		Cached:  true,
		Reverse: staged,
	})
	if err != nil {
		return nil, fmt.Errorf("应用补丁失败: %w", err)
	}

	return e.snapshotLocked()
}

// buildFilePatch 把 diff 文本转成带行号的结构化行。
func buildFilePatch(path string, staged bool, diffStr string) *FilePatch {
	p := patch.Parse(diffStr)
	raw := p.Lines()

	fp := &FilePatch{Path: path, Staged: staged, Lines: make([]PatchLineDTO, 0, len(raw))}

	oldNo, newNo := 0, 0
	for i, l := range raw {
		dto := PatchLineDTO{Index: i, Text: l.Content}

		switch l.Kind {
		case patch.PATCH_HEADER:
			dto.Kind = "header"
		case patch.HUNK_HEADER:
			dto.Kind = "hunk"
			// @@ -旧起点,旧行数 +新起点,新行数 @@
			oldNo, newNo = parseHunkHeader(l.Content)
		case patch.ADDITION:
			dto.Kind = "addition"
			dto.Marker = "+"
			dto.Text = strings.TrimPrefix(l.Content, "+")
			dto.NewNo = newNo
			newNo++
			dto.Selectable = true
			fp.HasChanges = true
		case patch.DELETION:
			dto.Kind = "deletion"
			dto.Marker = "-"
			dto.Text = strings.TrimPrefix(l.Content, "-")
			dto.OldNo = oldNo
			oldNo++
			dto.Selectable = true
			fp.HasChanges = true
		case patch.CONTEXT:
			dto.Kind = "context"
			dto.Marker = " "
			dto.Text = strings.TrimPrefix(l.Content, " ")
			dto.OldNo = oldNo
			dto.NewNo = newNo
			oldNo++
			newNo++
		case patch.NEWLINE_MESSAGE:
			dto.Kind = "meta"
			dto.Text = l.Content
		default:
			dto.Kind = "other"
		}

		fp.Lines = append(fp.Lines, dto)
	}

	return fp
}

// parseHunkHeader 从 "@@ -1,3 +1,5 @@" 里取出新旧起始行号。
func parseHunkHeader(header string) (int, int) {
	// 去掉前后的 @@
	s := strings.TrimSpace(header)
	s = strings.TrimPrefix(s, "@@")
	if i := strings.Index(s, "@@"); i != -1 {
		s = s[:i]
	}
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return 0, 0
	}
	oldStart := parseStart(fields[0])
	newStart := parseStart(fields[1])
	return oldStart, newStart
}

func parseStart(field string) int {
	// "-12,3" / "+12" / "12"
	field = strings.TrimLeft(field, "-+")
	if i := strings.Index(field, ","); i != -1 {
		field = field[:i]
	}
	n, err := strconv.Atoi(field)
	if err != nil {
		return 0
	}
	return n
}
