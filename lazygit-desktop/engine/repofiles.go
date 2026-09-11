package engine

import (
	"fmt"
	"os"
	"strings"
)

// 本文件提供「浏览整个仓库的文件」的能力。
//
// 侧边栏默认只列出**有改动**的文件，一旦提交/推送完就空了，
// 用户会以为「文件树不见了」。这里补一个完整仓库文件树：
//   - 已跟踪的文件
//   - 未跟踪但不被 .gitignore 忽略的文件
// 这样任何时候都能看到仓库长什么样。

// RepoFileDTO 是仓库里的一个文件（只带路径，树结构交给前端构建）。
type RepoFileDTO struct {
	Path string `json:"path"`
}

// RepoFiles 列出仓库里的所有文件。
func (e *Engine) RepoFiles() ([]RepoFileDTO, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	// --cached：已跟踪；--others + --exclude-standard：未跟踪但不被忽略
	out, err := e.gitOutput("ls-files", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	files := []RepoFileDTO{}
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		p := strings.TrimSpace(line)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		files = append(files, RepoFileDTO{Path: p})
	}
	return files, nil
}

// FileContentDTO 是一个文件的内容，用于界面上的只读浏览。
type FileContentDTO struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	// 二进制文件不返回内容，界面提示「无法预览」
	Binary bool `json:"binary"`
	// 文件过大时只返回前面一部分
	Truncated bool  `json:"truncated"`
	Lines     int   `json:"lines"`
	Size      int64 `json:"size"`
}

// 单个文件最多返回的内容（超出部分不读，避免把界面卡死）
const maxPreviewBytes = 512 * 1024

// FileContent 读取仓库里某个文件的内容。
func (e *Engine) FileContent(path string) (*FileContentDTO, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	full, err := e.safeRepoPath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s 是一个目录", path)
	}

	f, err := os.Open(full)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	limit := info.Size()
	truncated := false
	if limit > maxPreviewBytes {
		limit = maxPreviewBytes
		truncated = true
	}

	buf := make([]byte, limit)
	n, _ := f.Read(buf)
	buf = buf[:n]

	dto := &FileContentDTO{Path: path, Size: info.Size(), Truncated: truncated}

	// 含 NUL 字节基本就是二进制
	if strings.IndexByte(string(buf), 0) >= 0 {
		dto.Binary = true
		return dto, nil
	}

	content := string(buf)
	dto.Content = content
	dto.Lines = strings.Count(content, "\n") + 1
	return dto, nil
}
