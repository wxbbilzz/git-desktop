package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 丢弃文件改动的「回收站」。
//
// 「丢弃改动」是整个软件里唯一会真的让内容消失的操作 —— reset / merge / rebase
// 都能靠 reflog 救回来，但工作区里没提交过的改动一旦扔掉，git 对象库里根本没有它。
//
// 所以这里在丢弃之前，先把文件当前内容写成一个 git blob。
// 妙处在于 `git hash-object -w` **完全不碰仓库状态**：不建引用、不动 index，
// 只是往对象库里放一份内容并返回它的 SHA。之后想恢复，
// `git cat-file blob <SHA>` 写回原路径即可。
// 副作用只有对象库里多了一个不可达对象，会被 gc 按正常规则回收。
const (
	// 最多保留多少条回收记录（内存里，重启即失效 —— 够用了，
	// 需要长期保留的话用户会用 stash）
	maxDiscardRecords = 50
	// 超过这个大小的文件不做备份，避免把大文件整个读进内存
	maxDiscardBackupSize = 8 << 20 // 8 MiB
)

// DiscardRecord 是一条可恢复的丢弃记录。
type DiscardRecord struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	BlobSHA string `json:"blobSha"`
	Size    int    `json:"size"`
	When    string `json:"when"`
	// Truncated 为真表示文件太大没备份，只能看到记录但恢复不了
	Truncated bool `json:"truncated"`
}

// recordDiscardLocked 在丢弃之前把内容存起来。
//
// 任何一步失败都不阻断丢弃本身：这是「尽力而为」的保险，不是前置条件。
func (e *Engine) recordDiscardLocked(path string) {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return
	}

	full := filepath.Join(e.repoPath, clean)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		// 文件已经不在了（比如是「工作区里被删除」的状态），没什么可备份的
		return
	}

	if info.Size() > maxDiscardBackupSize {
		e.pushDiscardLocked(DiscardRecord{
			Path:      clean,
			Size:      int(info.Size()),
			When:      time.Now().Format("15:04:05"),
			Truncated: true,
		})
		return
	}

	// hash-object -w 会把内容写进对象库并回显 SHA，仓库状态一点不变
	out, err := e.gitOutput("hash-object", "-w", "--", clean)
	if err != nil {
		return
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		return
	}

	e.pushDiscardLocked(DiscardRecord{
		Path:    clean,
		BlobSHA: sha,
		Size:    int(info.Size()),
		When:    time.Now().Format("15:04:05"),
	})
}

func (e *Engine) pushDiscardLocked(rec DiscardRecord) {
	e.discardSeq++
	rec.ID = fmt.Sprintf("d%d", e.discardSeq)
	e.discards = append(e.discards, rec)
	if len(e.discards) > maxDiscardRecords {
		e.discards = e.discards[len(e.discards)-maxDiscardRecords:]
	}
}

// DiscardedFiles 返回本次运行里丢弃过的文件（最近的在前）。
func (e *Engine) DiscardedFiles() []DiscardRecord {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]DiscardRecord, 0, len(e.discards))
	for i := len(e.discards) - 1; i >= 0; i-- {
		out = append(out, e.discards[i])
	}
	return out
}

// RestoreDiscarded 把某条丢弃记录的内容写回原路径。
//
// 覆盖规则：只有当目标文件「当前没有改动」时才允许恢复。
// 丢弃一个已跟踪文件后，它的内容会被退回 HEAD 那个版本、文件本身还在 ——
// 这时它是干净的，恢复正好该覆盖它。但如果你后来又在编辑器里改过它，
// 恢复就会把你现在的改动冲掉，所以那种情况直接拒绝。
func (e *Engine) RestoreDiscarded(id string) (*RepoSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.requireRepoLocked(); err != nil {
		return nil, err
	}

	var rec *DiscardRecord
	for i := range e.discards {
		if e.discards[i].ID == id {
			rec = &e.discards[i]
			break
		}
	}
	if rec == nil {
		return nil, fmt.Errorf("找不到这条恢复记录（重启后记录会清空）")
	}
	if rec.Truncated {
		return nil, fmt.Errorf("这个文件太大，当时没有备份内容，无法恢复")
	}

	clean := filepath.Clean(rec.Path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return nil, fmt.Errorf("非法文件路径: %s", rec.Path)
	}
	full := filepath.Join(e.repoPath, clean)

	if _, err := os.Stat(full); err == nil {
		// 文件还在：只有它当前没有任何改动时才能安全覆盖
		out, _ := e.gitOutput("status", "--porcelain", "--", clean)
		if strings.TrimSpace(out) != "" {
			return nil, fmt.Errorf("文件 %s 现在已经有改动了，为避免覆盖你现在的编辑，"+
				"请先把它移走或撤掉改动再恢复", rec.Path)
		}
	}

	content, err := e.gitOutput("cat-file", "blob", rec.BlobSHA)
	if err != nil {
		return nil, fmt.Errorf("读不回备份内容: %w", err)
	}

	// 原来的目录可能已经被删掉了
	if dir := filepath.Dir(full); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return nil, err
	}

	// 恢复成功后这条记录就没用了，移掉避免重复点
	e.discards = removeRecord(e.discards, id)

	return e.snapshotLocked()
}

func removeRecord(list []DiscardRecord, id string) []DiscardRecord {
	out := list[:0]
	for _, r := range list {
		if r.ID != id {
			out = append(out, r)
		}
	}
	return out
}
