package engine

import (
	"fmt"
	"sort"
	"strings"
)

// 本文件是「git 全命令 GUI 化」的核心：一张数据表描述每个 git 操作。
//
// 为什么不给每个命令写一套界面：
// git 有 159 个顶级命令，其中约 90 个是用户级命令（porcelain）。
// 每个都手写 UI 会让代码量爆炸且难以维护。改成「目录 + 通用执行器」后：
//   - 前端只要一套「搜索 + 动态表单」渲染逻辑
//   - 新增命令 = 往表里加一条数据，前端自动就有了
//   - 危险操作（硬重置 / 强制推送 / 清理）能被明确标注
//
// 少数最高频的操作（提交、暂存、切换分支、同步）另有专属界面，
// 但它们底层也是往这张表的同一条命令传参。

// pathsParamName 是「路径类参数」的约定名字，统一放在 -- 之后。
const pathsParamName = "__paths"

// 参数类型
const (
	KindString = "string" // 单行文本
	KindText   = "text"   // 多行文本
	KindBool   = "bool"   // 开关
	KindChoice = "choice" // 固定枚举下拉
	// KindRef 是「从仓库数据里选」的下拉：分支、提交、文件、远端、标签、储藏。
	// 这样绝大多数操作点几下就能完成，不用手输名字。
	KindRef = "ref"
)

// 下拉选项的来源
const (
	SourceRef    = "ref"    // 分支 / 标签 / 提交都能用
	SourceBranch = "branch" // 本地分支
	SourceCommit = "commit" // 提交历史
	SourceFile   = "file"   // 仓库文件
	SourceRemote = "remote" // 远端名
	SourceTag    = "tag"    // 标签
	SourceStash  = "stash"  // 储藏记录
)

// Param 描述一个命令参数。
type Param struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind"`
	Required    bool     `json:"required"`
	Default     string   `json:"default"`
	Placeholder string   `json:"placeholder"`
	Choices     []string `json:"choices"`
	Help        string   `json:"help"`
	// Flag：拼 argv 时加在值前面，例如 "-m" 拼成 ["-m", 值]。
	// 对 KindBool，表示开关打开时要追加的标志。
	Flag string `json:"flag"`
	// Source 只在 Kind 为 KindRef 时有意义，指明下拉选项从哪来。
	Source string `json:"source"`
}

// Operation 描述一个 git 操作。
type Operation struct {
	ID          string  `json:"id"`
	Category    string  `json:"category"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Params      []Param `json:"params"`
	// Dangerous 为 true 时前端必须二次确认
	Dangerous bool `json:"dangerous"`
	// ReadOnly 表示只读查询，不会改动仓库
	ReadOnly bool `json:"readOnly"`
	// Base 是基础 argv，不发给前端
	Base []string `json:"-"`
}

// Catalog 返回全部受支持的 git 操作。
func Catalog() []Operation {
	var ops []Operation
	ops = append(ops, commitOps()...)
	ops = append(ops, branchOps()...)
	ops = append(ops, remoteOps()...)
	ops = append(ops, tagOps()...)
	ops = append(ops, stashOps()...)
	ops = append(ops, undoOps()...)
	ops = append(ops, worktreeOps()...)
	ops = append(ops, submoduleOps()...)
	ops = append(ops, patchOps()...)
	ops = append(ops, queryOps()...)
	ops = append(ops, maintenanceOps()...)
	ops = append(ops, configOps()...)
	ops = append(ops, exportOps()...)
	ops = append(ops, advancedOps()...)

	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Category != ops[j].Category {
			return ops[i].Category < ops[j].Category
		}
		return ops[i].Name < ops[j].Name
	})
	return ops
}

// OperationByID 按 id 查找操作。
func OperationByID(id string) (Operation, bool) {
	for _, op := range Catalog() {
		if op.ID == id {
			return op, true
		}
	}
	return Operation{}, false
}

// BuildArgs 把表单参数拼成 git 的 argv。
//
// 规则简单但覆盖绝大多数 git 命令：
//   - bool 为真时追加 Flag（不带值）
//   - 其他参数有 Flag 就先加 Flag 再加值（如 ["-m","提交信息"]）
//   - 未填且非必填则跳过
//   - 额外的 __paths 字段词分割后统一放在 "--" 之后
func (op Operation) BuildArgs(args map[string]string) ([]string, error) {
	argv := append([]string{}, op.Base...)

	for _, p := range op.Params {
		// __paths 统一放到 "--" 之后处理，不要在循环里再拼一次，
		// 否则会重复出现在命令里（历史上这里就出过这个 bug）。
		if p.Name == pathsParamName {
			continue
		}

		v := strings.TrimSpace(args[p.Name])

		if p.Kind == KindBool {
			if v == "true" || v == "1" || v == "on" {
				if p.Flag != "" {
					argv = append(argv, p.Flag)
				}
			}
			continue
		}

		if v == "" {
			if p.Required {
				return nil, fmt.Errorf("缺少必填参数：%s", p.Label)
			}
			continue
		}
		if p.Flag != "" {
			argv = append(argv, p.Flag)
		}
		argv = append(argv, v)
	}

	if paths := strings.TrimSpace(args[pathsParamName]); paths != "" {
		argv = append(argv, "--")
		// 用支持引号的切分，这样带空格的路径（"my file.txt"）不会被拆坏
		argv = append(argv, SplitCommandLine(paths)...)
	}

	return argv, nil
}

// ---------------------------------------------------------------- 参数快捷构造

func s(name, label string, required bool, placeholder string) Param {
	return Param{Name: name, Label: label, Kind: KindString, Required: required, Placeholder: placeholder}
}
func sf(name, label, flag string, required bool, placeholder string) Param {
	return Param{Name: name, Label: label, Kind: KindString, Flag: flag, Required: required, Placeholder: placeholder}
}

// ref 构造一个「从仓库数据里选」的下拉参数
func ref(name, label, source string, required bool) Param {
	return Param{
		Name:     name,
		Label:    label,
		Kind:     KindRef,
		Source:   source,
		Required: required,
	}
}

func bo(name, label, flag string, def bool) Param {
	d := "false"
	if def {
		d = "true"
	}
	return Param{Name: name, Label: label, Kind: KindBool, Flag: flag, Default: d}
}

// ---------------------------------------------------------------- 提交与历史

func commitOps() []Operation {
	return []Operation{
		{
			ID: "commit.amend", Category: "提交", Name: "修补最后一次提交",
			Description: "把当前暂存区的改动并入上一次提交，沿用原提交信息",
			Base:        []string{"commit", "--amend", "--no-edit"},
		},
		{
			ID: "commit.revert", Category: "提交", Name: "撤销某次提交",
			Description: "生成一个反向提交抵消指定提交（新建提交，安全）",
			Base:        []string{"revert", "--no-edit"},
			Params:      []Param{ref("ref", "要撤销的提交", SourceCommit, true)},
		},
		{
			ID: "commit.cherrypick", Category: "提交", Name: "拣选提交到当前分支",
			Description: "把其他分支的某次提交复制到当前分支",
			Base:        []string{"cherry-pick"},
			Params: []Param{
				ref("ref", "提交", SourceCommit, true),
				bo("no_commit", "只应用改动，不自动提交", "--no-commit", false),
			},
		},
		{
			ID: "commit.reset.soft", Category: "提交", Name: "回退提交（改动留在暂存区）",
			Description: "移动分支指针到指定提交，改动保留且处于已暂存状态",
			Base:        []string{"reset", "--soft"},
			Params:      []Param{ref("ref", "回退到", SourceCommit, true)},
		},
		{
			ID: "commit.reset.mixed", Category: "提交", Name: "回退提交（改动留在工作区）",
			Description: "移动分支指针到指定提交，改动保留但取消暂存",
			Base:        []string{"reset", "--mixed"},
			Params:      []Param{ref("ref", "回退到", SourceCommit, true)},
		},
		{
			ID: "commit.reset.hard", Category: "提交", Name: "硬回退（丢弃改动）",
			Description: "移动分支指针并丢弃所有未提交改动。不可撤销！",
			Base:        []string{"reset", "--hard"},
			Params:      []Param{ref("ref", "回退到", SourceCommit, true)},
			Dangerous:   true,
		},
		{
			ID: "commit.show", Category: "提交", Name: "查看某次提交",
			Description: "显示提交的元信息与完整改动",
			Base:        []string{"show", "--stat", "-p"},
			Params:      []Param{ref("ref", "提交", SourceCommit, true)},
			ReadOnly:    true,
		},
	}
}

// ---------------------------------------------------------------- 分支

func branchOps() []Operation {
	return []Operation{
		{
			ID: "branch.create", Category: "分支", Name: "新建分支",
			Description: "基于指定起点创建分支（不切换）",
			Base:        []string{"branch"},
			Params: []Param{
				s("name", "新分支名", true, "feature/xxx"),
				ref("start", "起点", SourceBranch, false),
			},
		},
		{
			ID: "branch.delete", Category: "分支", Name: "删除分支",
			Description: "删除已合并的本地分支",
			Base:        []string{"branch", "-d"},
			Params:      []Param{ref("name", "分支", SourceBranch, true)},
			Dangerous:   true,
		},
		{
			ID: "branch.delete.force", Category: "分支", Name: "强制删除分支",
			Description: "即使未合并也删除（未合并的提交会丢失）",
			Base:        []string{"branch", "-D"},
			Params:      []Param{ref("name", "分支", SourceBranch, true)},
			Dangerous:   true,
		},
		{
			ID: "branch.rename", Category: "分支", Name: "重命名分支",
			Description: "给当前分支或指定分支改名",
			Base:        []string{"branch", "-m"},
			Params: []Param{
				ref("old", "原分支", SourceBranch, false),
				s("new", "新分支名", true, "new-name"),
			},
		},
		{
			ID: "branch.checkout", Category: "分支", Name: "切换分支",
			Description: "检出指定分支或提交",
			Base:        []string{"checkout"},
			Params:      []Param{ref("name", "分支 / 提交", SourceRef, true)},
		},
		{
			ID: "branch.merge", Category: "分支", Name: "合并分支",
			Description: "把指定分支合并进当前分支",
			Base:        []string{"merge"},
			Params: []Param{
				ref("branch", "要合并进来的分支", SourceBranch, true),
				bo("no_ff", "总是生成合并提交（--no-ff）", "--no-ff", false),
				bo("squash", "压缩为一次改动（--squash）", "--squash", false),
			},
		},
		{
			ID: "branch.rebase", Category: "分支", Name: "变基到分支",
			Description: "把当前分支的提交重新应用到目标分支之上",
			Base:        []string{"rebase"},
			Params:      []Param{ref("onto", "目标分支", SourceBranch, true)},
		},
		{
			ID: "branch.rebase.continue", Category: "分支", Name: "继续变基",
			Description: "解决冲突并暂存后，继续被中断的变基",
			Base:        []string{"rebase", "--continue"},
		},
		{
			ID: "branch.rebase.abort", Category: "分支", Name: "中止变基",
			Description: "放弃变基并恢复到变基前的状态",
			Base:        []string{"rebase", "--abort"},
			Dangerous:   true,
		},
		{
			ID: "branch.merge.abort", Category: "分支", Name: "中止合并",
			Description: "放弃正在进行的合并，恢复到合并前状态",
			Base:        []string{"merge", "--abort"},
			Dangerous:   true,
		},
		{
			ID: "branch.setupstream", Category: "分支", Name: "设置上游分支",
			Description: "指定当前分支跟踪哪个远端分支",
			Base:        []string{"branch"},
			Params: []Param{
				sf("upstream", "上游", "--set-upstream-to", true, "origin/main"),
				ref("branch", "分支", SourceBranch, false),
			},
		},
		{
			ID: "branch.list", Category: "分支", Name: "列出分支",
			Description: "列出所有分支及其跟踪信息",
			Base:        []string{"branch", "-vv", "--all"},
			ReadOnly:    true,
		},
	}
}

// ---------------------------------------------------------------- 远端

func remoteOps() []Operation {
	return []Operation{
		{
			ID: "remote.add", Category: "远端", Name: "添加远端",
			Description: "新增一个远端仓库地址",
			Base:        []string{"remote", "add"},
			Params: []Param{
				ref("name", "远端", SourceRemote, true),
				s("url", "地址", true, "https://gitee.com/user/repo.git"),
			},
		},
		{
			ID: "remote.remove", Category: "远端", Name: "删除远端",
			Description: "移除一个远端配置（不影响远端服务器）",
			Base:        []string{"remote", "remove"},
			Params:      []Param{ref("name", "远端", SourceRemote, true)},
			Dangerous:   true,
		},
		{
			ID: "remote.rename", Category: "远端", Name: "重命名远端",
			Description: "修改远端的名字",
			Base:        []string{"remote", "rename"},
			Params: []Param{
				s("old", "原名", true, "origin"),
				s("new", "新名", true, "upstream"),
			},
		},
		{
			ID: "remote.seturl", Category: "远端", Name: "修改远端地址",
			Description: "替换某个远端的 URL",
			Base:        []string{"remote", "set-url"},
			Params: []Param{
				ref("name", "远端", SourceRemote, true),
				s("url", "新地址", true, "git@gitee.com:user/repo.git"),
			},
		},
		{
			ID: "remote.list", Category: "远端", Name: "列出远端",
			Description: "显示所有远端及其地址",
			Base:        []string{"remote", "-v"},
			ReadOnly:    true,
		},
		{
			ID: "remote.prune", Category: "远端", Name: "清理失效远端分支",
			Description: "删除远端已不存在的远程跟踪分支",
			Base:        []string{"remote", "prune"},
			Params:      []Param{ref("name", "远端", SourceRemote, true)},
		},
		{
			ID: "sync.fetch", Category: "远端", Name: "拉取远端更新",
			Description: "下载远端最新提交与分支，不合并到本地",
			Base:        []string{"fetch"},
			Params: []Param{
				ref("remote", "远端", SourceRemote, false),
				bo("prune", "顺带清理已删除的远端分支", "--prune", false),
				bo("tags", "同时拉取标签", "--tags", false),
			},
		},
		{
			ID: "sync.pull", Category: "远端", Name: "拉取并合并",
			Description: "从远端拉取并合入当前分支",
			Base:        []string{"pull"},
			Params: []Param{
				ref("remote", "远端", SourceRemote, false),
				s("branch", "分支（留空=当前）", false, ""),
				bo("ff_only", "只允许快进（--ff-only）", "--ff-only", false),
				bo("rebase", "用变基代替合并（--rebase）", "--rebase", false),
			},
		},
		{
			ID: "sync.push", Category: "远端", Name: "推送到远端",
			Description: "把当前分支的提交推送到远端",
			Base:        []string{"push"},
			Params: []Param{
				ref("remote", "远端", SourceRemote, false),
				s("branch", "分支（留空=当前）", false, ""),
				bo("set_upstream", "设为上游（-u，首次推送用）", "--set-upstream", false),
				bo("force_lease", "安全强推（--force-with-lease）", "--force-with-lease", false),
			},
		},
		{
			ID: "sync.push.tags", Category: "远端", Name: "推送所有标签",
			Description: "把所有本地标签推送到远端",
			Base:        []string{"push", "--tags"},
			Params:      []Param{ref("remote", "远端", SourceRemote, false)},
		},
		{
			ID: "sync.push.force", Category: "远端", Name: "强制推送（危险）",
			Description: "用本地历史覆盖远端，会丢弃远端提交，可能影响他人！",
			Base:        []string{"push", "--force"},
			Params: []Param{
				ref("remote", "远端", SourceRemote, true),
				s("branch", "分支", false, ""),
			},
			Dangerous: true,
		},
		{
			ID: "sync.lsremote", Category: "远端", Name: "查看远端引用",
			Description: "不下载对象，只列出远端的分支和标签",
			Base:        []string{"ls-remote"},
			Params:      []Param{ref("remote", "远端", SourceRemote, true)},
			ReadOnly:    true,
		},
	}
}

// ---------------------------------------------------------------- 标签

func tagOps() []Operation {
	return []Operation{
		{
			ID: "tag.create", Category: "标签", Name: "创建标签",
			Description: "给某个提交打标签。填了信息就是附注标签",
			Base:        []string{"tag"},
			Params: []Param{
				ref("name", "标签", SourceTag, true),
				s("ref", "目标提交（留空=HEAD）", false, "HEAD"),
				sf("message", "附注信息", "-m", false, "发布 v1.0.0"),
			},
		},
		{
			ID: "tag.delete", Category: "标签", Name: "删除本地标签",
			Description: "删除一个本地标签",
			Base:        []string{"tag", "-d"},
			Params:      []Param{ref("name", "标签", SourceTag, true)},
			Dangerous:   true,
		},
		{
			ID: "tag.push", Category: "标签", Name: "推送标签",
			Description: "把指定标签推送到远端",
			Base:        []string{"push"},
			Params: []Param{
				ref("remote", "远端", SourceRemote, true),
				ref("name", "标签", SourceTag, true),
			},
		},
		{
			ID: "tag.delete.remote", Category: "标签", Name: "删除远端标签",
			Description: "删除远端服务器上的标签",
			Base:        []string{"push"},
			Params: []Param{
				ref("remote", "远端", SourceRemote, true),
				sf("name", "标签名", "--delete", true, "v1.0.0"),
			},
			Dangerous: true,
		},
		{
			ID: "tag.list", Category: "标签", Name: "列出标签",
			Description: "按创建时间倒序列出标签",
			Base:        []string{"tag", "-l", "--sort=-creatordate"},
			Params:      []Param{s("pattern", "过滤模式（可选）", false, "v1.*")},
			ReadOnly:    true,
		},
	}
}

// ---------------------------------------------------------------- 储藏

func stashOps() []Operation {
	return []Operation{
		{
			ID: "stash.save", Category: "储藏", Name: "储藏当前改动",
			Description: "把工作区和暂存区的改动存起来，让工作区变干净",
			Base:        []string{"stash", "push"},
			Params: []Param{
				sf("message", "说明", "-m", false, "临时保存"),
				bo("untracked", "包含未跟踪文件（-u）", "--include-untracked", false),
				bo("keep_index", "保留暂存区状态（--keep-index）", "--keep-index", false),
			},
		},
		{
			ID: "stash.list", Category: "储藏", Name: "列出储藏",
			Description: "显示所有储藏记录",
			Base:        []string{"stash", "list"},
			ReadOnly:    true,
		},
		{
			ID: "stash.show", Category: "储藏", Name: "查看储藏内容",
			Description: "显示某条储藏的具体改动",
			Base:        []string{"stash", "show", "-p"},
			Params:      []Param{ref("index", "储藏", SourceStash, true)},
			ReadOnly:    true,
		},
		{
			ID: "stash.pop", Category: "储藏", Name: "弹出储藏",
			Description: "应用储藏并从列表中删除它",
			Base:        []string{"stash", "pop"},
			Params:      []Param{ref("index", "储藏", SourceStash, false)},
		},
		{
			ID: "stash.apply", Category: "储藏", Name: "应用储藏（保留记录）",
			Description: "应用储藏但保留在列表里",
			Base:        []string{"stash", "apply"},
			Params:      []Param{ref("index", "储藏", SourceStash, false)},
		},
		{
			ID: "stash.drop", Category: "储藏", Name: "删除一条储藏",
			Description: "丢弃指定储藏，不再保留",
			Base:        []string{"stash", "drop"},
			Params:      []Param{ref("index", "储藏", SourceStash, true)},
			Dangerous:   true,
		},
		{
			ID: "stash.branch", Category: "储藏", Name: "从储藏创建分支",
			Description: "以某条储藏为基础新建分支并应用它",
			Base:        []string{"stash", "branch"},
			Params: []Param{
				s("name", "新分支名", true, "recover-work"),
				ref("index", "储藏", SourceStash, false),
			},
		},
		{
			ID: "stash.clear", Category: "储藏", Name: "清空所有储藏",
			Description: "删除全部储藏记录。不可撤销！",
			Base:        []string{"stash", "clear"},
			Dangerous:   true,
		},
	}
}

// ---------------------------------------------------------------- 撤销与恢复

func undoOps() []Operation {
	return []Operation{
		{
			ID: "restore.file", Category: "撤销", Name: "丢弃文件改动",
			Description: "把文件恢复到暂存区版本，丢弃未暂存的修改",
			Base:        []string{"restore"},
			Params:      []Param{ref("__paths", "文件", SourceFile, true)},
			Dangerous:   true,
		},
		{
			ID: "restore.staged", Category: "撤销", Name: "取消暂存文件",
			Description: "把文件从暂存区退回工作区，改动本身保留",
			Base:        []string{"restore", "--staged"},
			Params:      []Param{ref("__paths", "文件", SourceFile, true)},
		},
		{
			ID: "clean.dry", Category: "撤销", Name: "预览将被清理的文件",
			Description: "列出会被 git clean 删除的未跟踪文件（不实际删除）",
			Base:        []string{"clean", "-nd"},
			ReadOnly:    true,
		},
		{
			ID: "clean.run", Category: "撤销", Name: "清理未跟踪文件",
			Description: "删除未跟踪的文件和目录。不可撤销！",
			Base:        []string{"clean", "-fd"},
			Dangerous:   true,
		},
		{
			ID: "clean.ignored", Category: "撤销", Name: "清理未跟踪与忽略文件",
			Description: "连同 .gitignore 忽略的文件一起删除（不影响已跟踪文件）",
			Base:        []string{"clean", "-fdx"},
			Dangerous:   true,
		},
		{
			ID: "reflog", Category: "撤销", Name: "查看引用日志",
			Description: "查看 HEAD 的移动历史，用于找回丢失的提交",
			Base:        []string{"reflog", "--date=relative"},
			Params:      []Param{sf("count", "显示条数", "-n", false, "50")},
			ReadOnly:    true,
		},
		{
			ID: "reflog.recover", Category: "撤销", Name: "从 reflog 恢复分支",
			Description: "把某个历史位置重新检出为分支，用于找回误删的提交",
			Base:        []string{"checkout", "-B"},
			Params: []Param{
				s("branch", "恢复出的分支名", true, "recovered"),
				s("ref", "reflog 位置", true, "HEAD@{1}"),
			},
		},
	}
}

// ---------------------------------------------------------------- 工作区

func worktreeOps() []Operation {
	return []Operation{
		{
			ID: "worktree.list", Category: "工作区", Name: "列出工作区",
			Description: "显示所有关联的工作区目录",
			Base:        []string{"worktree", "list"},
			ReadOnly:    true,
		},
		{
			ID: "worktree.add", Category: "工作区", Name: "新增工作区",
			Description: "在另一个目录检出某个分支，同时开发多个分支",
			Base:        []string{"worktree", "add"},
			Params: []Param{
				s("path", "目录路径", true, "../repo-hotfix"),
				s("branch", "分支名", true, "hotfix"),
				bo("new_branch", "同时新建该分支（-b）", "-b", false),
			},
		},
		{
			ID: "worktree.remove", Category: "工作区", Name: "删除工作区",
			Description: "移除一个工作区目录",
			Base:        []string{"worktree", "remove"},
			Params: []Param{
				s("path", "目录路径", true, "../repo-hotfix"),
				bo("force", "强制删除（有改动时）", "--force", false),
			},
			Dangerous: true,
		},
		{
			ID: "worktree.prune", Category: "工作区", Name: "清理工作区记录",
			Description: "删除目录已不存在的过期工作区记录",
			Base:        []string{"worktree", "prune"},
		},
	}
}

// ---------------------------------------------------------------- 子模块

func submoduleOps() []Operation {
	return []Operation{
		{
			ID: "submodule.status", Category: "子模块", Name: "查看子模块状态",
			Description: "列出所有子模块及其当前提交",
			Base:        []string{"submodule", "status"},
			ReadOnly:    true,
		},
		{
			ID: "submodule.add", Category: "子模块", Name: "添加子模块",
			Description: "把另一个仓库作为子目录引入当前仓库",
			Base:        []string{"submodule", "add"},
			Params: []Param{
				s("url", "仓库地址", true, "https://gitee.com/user/lib.git"),
				s("path", "放在哪个目录", true, "vendor/lib"),
			},
		},
		{
			ID: "submodule.init", Category: "子模块", Name: "初始化子模块",
			Description: "注册 .gitmodules 中定义的子模块",
			Base:        []string{"submodule", "init"},
		},
		{
			ID: "submodule.update", Category: "子模块", Name: "更新子模块",
			Description: "拉取并检出子模块的正确提交（含递归）",
			Base:        []string{"submodule", "update", "--init", "--recursive"},
		},
		{
			ID: "submodule.sync", Category: "子模块", Name: "同步子模块地址",
			Description: "把 .gitmodules 里的 URL 同步到本地配置",
			Base:        []string{"submodule", "sync", "--recursive"},
		},
		{
			ID: "submodule.deinit", Category: "子模块", Name: "注销子模块",
			Description: "从本地配置中移除子模块（工作区目录也会被清空）",
			Base:        []string{"submodule", "deinit", "-f"},
			Params:      []Param{s("path", "子模块路径", true, "vendor/lib")},
			Dangerous:   true,
		},
	}
}

// ---------------------------------------------------------------- 补丁

func patchOps() []Operation {
	return []Operation{
		{
			ID: "patch.format", Category: "补丁", Name: "导出提交为补丁",
			Description: "把若干提交导出成 .patch 文件，便于邮件或离线传递",
			Base:        []string{"format-patch"},
			Params: []Param{
				s("range", "提交范围", true, "HEAD~3..HEAD"),
				sf("output", "输出目录", "-o", false, "./patches"),
			},
		},
		{
			ID: "patch.apply", Category: "补丁", Name: "应用补丁文件",
			Description: "把 .patch / .diff 的改动应用到工作区（不产生提交）",
			Base:        []string{"apply"},
			Params: []Param{
				s("file", "补丁文件", true, "fix.patch"),
				bo("check", "只检查能否应用（--check）", "--check", false),
				bo("reverse", "反向应用（--reverse）", "--reverse", false),
			},
		},
		{
			ID: "patch.am", Category: "补丁", Name: "应用补丁并保留提交",
			Description: "用 git am 应用邮件格式补丁，保留原作者和提交信息",
			Base:        []string{"am"},
			Params:      []Param{s("file", "补丁文件", true, "0001-fix.patch")},
		},
		{
			ID: "patch.am.abort", Category: "补丁", Name: "中止 am",
			Description: "放弃正在进行的 git am",
			Base:        []string{"am", "--abort"},
			Dangerous:   true,
		},
		{
			ID: "patch.diff", Category: "补丁", Name: "查看两版本差异",
			Description: "查看两个提交之间的差异",
			Base:        []string{"diff"},
			Params:      []Param{s("range", "范围（如 main..HEAD）", true, "main..HEAD")},
			ReadOnly:    true,
		},
	}
}

// ---------------------------------------------------------------- 查询

func queryOps() []Operation {
	return []Operation{
		{
			ID: "query.log", Category: "查询", Name: "查看提交日志",
			Description: "以单行形式列出提交历史",
			Base:        []string{"log", "--oneline", "--graph", "--decorate"},
			Params: []Param{
				sf("count", "条数", "-n", false, "30"),
				s("ref", "分支或范围", false, "HEAD"),
			},
			ReadOnly: true,
		},
		{
			ID: "query.blame", Category: "查询", Name: "追溯文件每行来源",
			Description: "显示文件每一行最后由哪次提交、谁修改",
			Base:        []string{"blame"},
			Params:      []Param{ref("file", "文件", SourceFile, true)},
			ReadOnly:    true,
		},
		{
			ID: "query.grep", Category: "查询", Name: "在仓库内容中搜索",
			Description: "在已跟踪文件里搜索文本",
			Base:        []string{"grep", "-n", "-I"},
			Params: []Param{
				s("pattern", "搜索内容", true, "TODO"),
				s("ref", "在哪个版本里搜（留空=工作区）", false, ""),
			},
			ReadOnly: true,
		},
		{
			ID: "query.describe", Category: "查询", Name: "描述当前版本",
			Description: "显示离当前提交最近的那个标签",
			Base:        []string{"describe", "--tags", "--always"},
			Params:      []Param{s("ref", "提交（留空=HEAD）", false, "")},
			ReadOnly:    true,
		},
		{
			ID: "query.shortlog", Category: "查询", Name: "按作者统计提交",
			Description: "统计每位作者的提交次数",
			Base:        []string{"shortlog", "-sn", "--all"},
			ReadOnly:    true,
		},
		{
			ID: "query.lsfiles", Category: "查询", Name: "列出已跟踪文件",
			Description: "列出仓库中被 git 跟踪的文件",
			Base:        []string{"ls-files"},
			Params:      []Param{s("pattern", "过滤（可选）", false, "*.go")},
			ReadOnly:    true,
		},
		{
			ID: "query.showref", Category: "查询", Name: "列出所有引用",
			Description: "显示 refs 下所有分支、标签对应的提交",
			Base:        []string{"show-ref"},
			Params:      []Param{s("pattern", "过滤（可选）", false, "refs/heads/")},
			ReadOnly:    true,
		},
		{
			ID: "query.revparse", Category: "查询", Name: "解析引用为提交号",
			Description: "把分支名、HEAD~2 之类写法解析成完整哈希",
			Base:        []string{"rev-parse"},
			Params:      []Param{s("ref", "引用", true, "HEAD")},
			ReadOnly:    true,
		},
		{
			ID: "query.catfile", Category: "查询", Name: "查看 git 对象内容",
			Description: "直接查看某个对象（提交/树/文件）的内容，底层命令",
			Base:        []string{"cat-file", "-p"},
			Params:      []Param{s("object", "对象（哈希或 HEAD:path）", true, "HEAD:README.md")},
			ReadOnly:    true,
		},
		{
			ID: "query.status", Category: "查询", Name: "查看状态",
			Description: "git status 的完整输出",
			Base:        []string{"status"},
			ReadOnly:    true,
		},
	}
}

// ---------------------------------------------------------------- 仓库维护

func maintenanceOps() []Operation {
	return []Operation{
		{
			ID: "maint.fsck", Category: "维护", Name: "校验仓库完整性",
			Description: "检查对象数据库是否损坏、有无悬空对象",
			Base:        []string{"fsck"},
			ReadOnly:    true,
		},
		{
			ID: "maint.count", Category: "维护", Name: "统计对象数量与体积",
			Description: "显示松散对象、打包对象的数量和占用空间",
			Base:        []string{"count-objects", "-vH"},
			ReadOnly:    true,
		},
		{
			ID: "maint.gc", Category: "维护", Name: "垃圾回收",
			Description: "整理对象数据库并压缩松散对象（仓库会变小）",
			Base:        []string{"gc"},
			Params:      []Param{bo("aggressive", "深度优化（较慢）", "--aggressive", false)},
		},
		{
			ID: "maint.repack", Category: "维护", Name: "重新打包对象",
			Description: "把对象重新打包，可去掉冗余的旧包",
			Base:        []string{"repack", "-ad"},
		},
		{
			ID: "maint.prune", Category: "维护", Name: "清理不可达对象",
			Description: "删除不再被任何引用指向的对象",
			Base:        []string{"prune"},
			Dangerous:   true,
		},
		{
			ID: "maint.prune.dry", Category: "维护", Name: "预览不可达对象",
			Description: "只列出会被清理的对象，不实际删除",
			Base:        []string{"prune", "-n"},
			ReadOnly:    true,
		},
		{
			ID: "maint.maintenance", Category: "维护", Name: "运行仓库维护",
			Description: "按配置执行自动维护任务（提交图、预取等）",
			Base:        []string{"maintenance", "run"},
		},
		{
			ID: "maint.verifypack", Category: "维护", Name: "校验 pack 文件",
			Description: "检查某个 .pack 文件的完整性",
			Base:        []string{"verify-pack", "-v"},
			Params:      []Param{s("file", "pack 文件路径", true, ".git/objects/pack/pack-xxx.idx")},
			ReadOnly:    true,
		},
	}
}

// ---------------------------------------------------------------- 配置

func configOps() []Operation {
	return []Operation{
		{
			ID: "config.list", Category: "配置", Name: "列出全部配置",
			Description: "显示当前生效的所有 git 配置项及来源",
			Base:        []string{"config", "--list", "--show-origin"},
			ReadOnly:    true,
		},
		{
			ID: "config.get", Category: "配置", Name: "读取配置项",
			Description: "查询某个配置键的值",
			Base:        []string{"config", "--get"},
			Params:      []Param{s("key", "配置键", true, "user.name")},
			ReadOnly:    true,
		},
		{
			ID: "config.set.local", Category: "配置", Name: "写入仓库级配置",
			Description: "只对当前仓库生效的配置",
			Base:        []string{"config"},
			Params: []Param{
				s("key", "配置键", true, "user.email"),
				s("value", "值", true, "me@example.com"),
			},
		},
		{
			ID: "config.set.global", Category: "配置", Name: "写入全局配置",
			Description: "对当前用户所有仓库生效的配置",
			Base:        []string{"config", "--global"},
			Params: []Param{
				s("key", "配置键", true, "user.name"),
				s("value", "值", true, "你的名字"),
			},
		},
		{
			ID: "config.unset.local", Category: "配置", Name: "删除仓库级配置",
			Description: "移除当前仓库里的一项配置",
			Base:        []string{"config", "--unset"},
			Params:      []Param{s("key", "配置键", true, "some.key")},
			Dangerous:   true,
		},
		{
			ID: "config.editor", Category: "配置", Name: "查看默认编辑器",
			Description: "查看 git 使用的编辑器配置",
			Base:        []string{"var", "GIT_EDITOR"},
			ReadOnly:    true,
		},
	}
}

// ---------------------------------------------------------------- 导出

func exportOps() []Operation {
	return []Operation{
		{
			ID: "export.archive", Category: "导出", Name: "导出源码压缩包",
			Description: "把某个版本导出成 tar/zip，不含 .git 目录",
			Base:        []string{"archive"},
			Params: []Param{
				sf("output", "输出文件", "-o", true, "release.tar.gz"),
				s("ref", "版本", true, "HEAD"),
				sf("format", "格式", "--format", false, "tar.gz"),
			},
		},
		{
			ID: "export.bundle", Category: "导出", Name: "打包整个仓库为单文件",
			Description: "把仓库所有引用和对象打成 .bundle，可离线传给他人",
			Base:        []string{"bundle", "create"},
			Params:      []Param{s("file", "输出文件", true, "repo.bundle")},
		},
		{
			ID: "export.bundle.verify", Category: "导出", Name: "校验 bundle 文件",
			Description: "检查 bundle 文件是否完整可用",
			Base:        []string{"bundle", "verify"},
			Params:      []Param{s("file", "bundle 文件", true, "repo.bundle")},
			ReadOnly:    true,
		},
		{
			ID: "export.requestpull", Category: "导出", Name: "生成合并请求描述",
			Description: "生成一段可发给维护者的合并请求文本",
			Base:        []string{"request-pull"},
			Params: []Param{
				s("start", "起点", true, "origin/main"),
				s("url", "仓库地址", true, "https://gitee.com/user/repo.git"),
				s("end", "终点", false, "HEAD"),
			},
			ReadOnly: true,
		},
	}
}

// ---------------------------------------------------------------- 高级

func advancedOps() []Operation {
	return []Operation{
		{
			ID: "notes.list", Category: "高级", Name: "列出备注",
			Description: "git notes 给提交附加的说明列表",
			Base:        []string{"notes", "list"},
			ReadOnly:    true,
		},
		{
			ID: "notes.show", Category: "高级", Name: "查看某次提交的备注",
			Description: "显示附加在指定提交上的备注内容",
			Base:        []string{"notes", "show"},
			Params:      []Param{s("ref", "提交", false, "HEAD")},
			ReadOnly:    true,
		},
		{
			ID: "notes.add", Category: "高级", Name: "添加备注",
			Description: "给某次提交附加一段说明",
			Base:        []string{"notes", "add"},
			Params: []Param{
				sf("message", "备注内容", "-m", true, "这段代码后来被重构了"),
				s("ref", "提交", false, "HEAD"),
			},
		},
		{
			ID: "notes.remove", Category: "高级", Name: "删除备注",
			Description: "移除附加在提交上的备注",
			Base:        []string{"notes", "remove"},
			Params:      []Param{s("ref", "提交", false, "HEAD")},
			Dangerous:   true,
		},
		{
			ID: "bisect.start", Category: "高级", Name: "开始二分查找问题提交",
			Description: "启动 bisect，之后用 good/bad 标记定位引入 bug 的提交",
			Base:        []string{"bisect", "start"},
		},
		{
			ID: "bisect.good", Category: "高级", Name: "标记为正常（good）",
			Description: "告诉 bisect 这个版本没有问题",
			Base:        []string{"bisect", "good"},
			Params:      []Param{s("ref", "提交（留空=当前）", false, "")},
		},
		{
			ID: "bisect.bad", Category: "高级", Name: "标记为有问题（bad）",
			Description: "告诉 bisect 这个版本有问题",
			Base:        []string{"bisect", "bad"},
			Params:      []Param{s("ref", "提交（留空=当前）", false, "")},
		},
		{
			ID: "bisect.reset", Category: "高级", Name: "结束二分查找",
			Description: "退出 bisect 并回到原来的分支",
			Base:        []string{"bisect", "reset"},
		},
		{
			ID: "replace.add", Category: "高级", Name: "替换某个对象",
			Description: "让 git 读取某对象时改用替代对象（历史改写实验用）",
			Base:        []string{"replace"},
			Params: []Param{
				s("object", "被替换的对象", true, "abc1234"),
				s("replacement", "替代对象", true, "def5678"),
			},
			Dangerous: true,
		},
		{
			ID: "replace.list", Category: "高级", Name: "列出替换规则",
			Description: "显示当前的 replace 规则",
			Base:        []string{"replace", "-l"},
			ReadOnly:    true,
		},
		{
			ID: "sparse.init", Category: "高级", Name: "启用稀疏检出",
			Description: "只检出仓库的部分目录，适合超大仓库",
			Base:        []string{"sparse-checkout", "init"},
			Params:      []Param{bo("cone", "使用 cone 模式（推荐）", "--cone", true)},
		},
		{
			ID: "sparse.set", Category: "高级", Name: "设置稀疏检出目录",
			Description: "指定要检出的目录（其他目录不落到磁盘）",
			Base:        []string{"sparse-checkout", "set"},
			Params:      []Param{s("patterns", "目录（空格分隔）", true, "src docs")},
		},
		{
			ID: "sparse.disable", Category: "高级", Name: "关闭稀疏检出",
			Description: "恢复检出全部文件",
			Base:        []string{"sparse-checkout", "disable"},
		},
		{
			ID: "subtree.add", Category: "高级", Name: "引入子树",
			Description: "把另一个仓库作为一个子目录并入（历史一起并入）",
			Base:        []string{"subtree", "add"},
			Params: []Param{
				sf("prefix", "子目录", "--prefix", true, "vendor/lib"),
				s("url", "仓库地址", true, "https://gitee.com/user/lib.git"),
				s("ref", "分支/标签", false, "main"),
			},
		},
		{
			ID: "rerere.status", Category: "高级", Name: "查看冲突解决记录",
			Description: "显示 rerere 记录下来的冲突解决方案",
			Base:        []string{"rerere", "status"},
			ReadOnly:    true,
		},
		{
			ID: "hash.object", Category: "高级", Name: "计算文件的对象哈希",
			Description: "底层命令：算出某个文件存进 git 后的哈希值",
			Base:        []string{"hash-object"},
			Params: []Param{
				s("file", "文件路径", true, "README.md"),
				bo("write", "同时写入对象库（-w）", "-w", false),
			},
			ReadOnly: true,
		},
	}
}
