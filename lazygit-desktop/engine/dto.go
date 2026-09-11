package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

// RepoSnapshot 是发往前端的完整状态快照。
// 前端不持有任何 git 逻辑，只渲染这份数据 —— 这是“引擎 / UI 分离”的关键。
type RepoSnapshot struct {
	RepoPath   string `json:"repoPath"`
	RepoName   string `json:"repoName"`
	Branch     string `json:"branch"`
	IsDetached bool   `json:"isDetached"`
	// 提交身份（user.name / user.email）。任一为空时 git 会拒绝提交
	IdentityName  string      `json:"identityName"`
	IdentityEmail string      `json:"identityEmail"`
	State         string      `json:"state"` // 空串表示不在 rebase / merge 等特殊状态
	Files         []FileDTO   `json:"files"`
	Commits       []CommitDTO `json:"commits"`
	Branches      []BranchDTO `json:"branches"`
}

// FileDTO 是工作区 / 暂存区里的一个文件。
//
// 注意：同一个文件可能同时有「已暂存」和「未暂存」的改动，
// 所以 IsStaged / IsUnstaged 是两个独立的布尔值，而不是一个枚举。
type FileDTO struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previousPath"`
	Status       string `json:"status"` // 类似 "M"、"??"、"AD"
	StatusLabel  string `json:"statusLabel"`
	Kind         string `json:"kind"` // new / modified / deleted / renamed / conflict / untracked
	IsStaged     bool   `json:"isStaged"`
	IsUnstaged   bool   `json:"isUnstaged"`
	IsTracked    bool   `json:"isTracked"`
	HasConflicts bool   `json:"hasConflicts"`
	LinesAdded   int    `json:"linesAdded"`
	LinesDeleted int    `json:"linesDeleted"`
}

// CommitDTO 是提交历史里的一条。
type CommitDTO struct {
	Hash      string   `json:"hash"`
	ShortHash string   `json:"shortHash"`
	Subject   string   `json:"subject"`
	Author    string   `json:"author"`
	When      string   `json:"when"`
	Tags      []string `json:"tags"`
	ExtraInfo string   `json:"extraInfo"` // 类似 "HEAD -> main, origin/main"
	// Parents 是父提交哈希，界面用它计算提交图的分支泳道
	Parents []string `json:"parents"`
}

// BranchDTO 是一个本地分支。
type BranchDTO struct {
	Name     string `json:"name"`
	IsHead   bool   `json:"isHead"`
	Ahead    string `json:"ahead"`
	Behind   string `json:"behind"`
	Upstream string `json:"upstream"`
	Subject  string `json:"subject"`
}

func toFileDTO(f *models.File) FileDTO {
	return FileDTO{
		Path:         f.Path,
		PreviousPath: f.PreviousPath,
		Status:       strings.TrimSpace(f.ShortStatus),
		StatusLabel:  fileStatusLabel(f),
		Kind:         fileStatusKind(f),
		IsStaged:     f.HasStagedChanges,
		IsUnstaged:   f.HasUnstagedChanges,
		IsTracked:    f.Tracked,
		HasConflicts: f.HasMergeConflicts,
		LinesAdded:   f.LinesAdded,
		LinesDeleted: f.LinesDeleted,
	}
}

func fileStatusLabel(f *models.File) string {
	s := strings.TrimSpace(f.ShortStatus)
	switch {
	case f.HasMergeConflicts:
		return "冲突"
	case s == "??":
		return "未跟踪"
	case strings.Contains(s, "R"):
		return "重命名"
	case strings.Contains(s, "A"):
		return "新增"
	case strings.Contains(s, "D"):
		return "删除"
	case strings.Contains(s, "M"):
		return "修改"
	default:
		return "改动"
	}
}

func fileStatusKind(f *models.File) string {
	s := strings.TrimSpace(f.ShortStatus)
	switch {
	case f.HasMergeConflicts:
		return "conflict"
	case s == "??":
		return "untracked"
	case strings.Contains(s, "R"):
		return "renamed"
	case strings.Contains(s, "A"):
		return "new"
	case strings.Contains(s, "D"):
		return "deleted"
	default:
		return "modified"
	}
}

func toCommitDTO(c *models.Commit) CommitDTO {
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	return CommitDTO{
		Hash:      c.Hash(),
		ShortHash: c.ShortHash(),
		Subject:   c.Name,
		Author:    c.AuthorName,
		When:      relativeTime(c.UnixTimestamp),
		Tags:      tags,
		ExtraInfo: c.ExtraInfo,
		Parents:   c.Parents(),
	}
}

func toBranchDTO(b *models.Branch) BranchDTO {
	upstream := ""
	if b.UpstreamRemote != "" && b.UpstreamBranch != "" {
		upstream = b.UpstreamRemote + "/" + b.UpstreamBranch
	}
	return BranchDTO{
		Name:     b.Name,
		IsHead:   b.Head,
		Ahead:    b.AheadForPull,
		Behind:   b.BehindForPull,
		Upstream: upstream,
		Subject:  b.Subject,
	}
}

func stateLabel(s models.WorkingTreeState) string {
	switch {
	case s.Rebasing:
		return "rebase 中"
	case s.Merging:
		return "合并中"
	case s.CherryPicking:
		return "cherry-pick 中"
	case s.Reverting:
		return "revert 中"
	default:
		return ""
	}
}

func relativeTime(ts int64) string {
	if ts == 0 {
		return ""
	}

	d := time.Since(time.Unix(ts, 0))
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	default:
		return time.Unix(ts, 0).Format("2006-01-02")
	}
}
