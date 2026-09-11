package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// 本文件提供 stash 面板需要的能力：列表、预览、以及各种操作。
// 操作本身复用 lazygit 的 StashCommands，只有列表解析是自己做的
// （`git stash list` 的输出格式得自己拆）。

// StashEntryDTO 是一条储藏记录。
type StashEntryDTO struct {
	Index   int    `json:"index"`
	Ref     string `json:"ref"` // stash@{0}
	Message string `json:"message"`
	Branch  string `json:"branch"`
}

// Stashes 列出所有储藏记录（最近的在前）。
func (e *Engine) Stashes() ([]StashEntryDTO, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	out, err := e.gitOutput("stash", "list")
	if err != nil {
		return nil, err
	}
	return parseStashList(out), nil
}

// StashShow 取某条储藏的内容预览。
func (e *Engine) StashShow(index int) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return "", err
	}
	return e.gitOutput("stash", "show", "-p", fmt.Sprintf("stash@{%d}", index))
}

// StashSave 把当前改动存进储藏。
func (e *Engine) StashSave(message string, includeUntracked bool) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	args := []string{"stash", "push"}
	if m := strings.TrimSpace(message); m != "" {
		args = append(args, "-m", m)
	}
	if includeUntracked {
		args = append(args, "--include-untracked")
	}

	if out, err := e.gitRun(args...); err != nil {
		return nil, fmt.Errorf("储藏失败: %s", firstErrorLine(out))
	}
	return e.snapshotLocked()
}

// StashPop 应用某条储藏并从列表移除。
func (e *Engine) StashPop(index int) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := e.git.Stash.Pop(index); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// StashApply 应用某条储藏但保留记录。
func (e *Engine) StashApply(index int) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := e.git.Stash.Apply(index); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// StashDrop 删除某条储藏。
func (e *Engine) StashDrop(index int) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}
	if err := e.git.Stash.Drop(index); err != nil {
		return nil, err
	}
	return e.snapshotLocked()
}

// parseStashList 解析 `git stash list` 的输出。
//
//	stash@{0}: WIP on main: abc1234 修复登录问题
//	stash@{1}: On feature-x: 临时保存
func parseStashList(out string) []StashEntryDTO {
	entries := []StashEntryDTO{}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 取出 stash@{N}
		open := strings.Index(line, "{")
		closeIdx := strings.Index(line, "}")
		if open == -1 || closeIdx == -1 || closeIdx < open {
			continue
		}
		idx, err := strconv.Atoi(line[open+1 : closeIdx])
		if err != nil {
			continue
		}

		rest := strings.TrimSpace(line[closeIdx+1:])
		rest = strings.TrimPrefix(rest, ":")
		rest = strings.TrimSpace(rest)

		entry := StashEntryDTO{
			Index:   idx,
			Ref:     fmt.Sprintf("stash@{%d}", idx),
			Message: rest,
		}

		// 解析分支名
		switch {
		case strings.HasPrefix(rest, "WIP on "):
			entry.Branch = cutBefore(strings.TrimPrefix(rest, "WIP on "), ":")
		case strings.HasPrefix(rest, "On "):
			entry.Branch = cutBefore(strings.TrimPrefix(rest, "On "), ":")
		}

		entries = append(entries, entry)
	}

	return entries
}

// cutBefore 返回 s 在第一个 sep 之前的部分；没有 sep 就返回原串。
func cutBefore(s, sep string) string {
	if i := strings.Index(s, sep); i != -1 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
