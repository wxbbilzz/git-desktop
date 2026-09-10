// 这些类型与 Go 侧 engine/dto.go 里的结构体一一对应。
// Wails 会依据 Go 结构体自动生成一份等价定义，这里手写是为了让前端
// 在没有编译 Go 的情况下也能独立跑起来（npm run dev 预览）。

export type FileKind =
  | "new"
  | "modified"
  | "deleted"
  | "renamed"
  | "conflict"
  | "untracked";

export interface FileDTO {
  path: string;
  previousPath: string;
  status: string;
  statusLabel: string;
  kind: FileKind | string;
  isStaged: boolean;
  isUnstaged: boolean;
  isTracked: boolean;
  hasConflicts: boolean;
  linesAdded: number;
  linesDeleted: number;
}

export interface CommitDTO {
  hash: string;
  shortHash: string;
  subject: string;
  author: string;
  when: string;
  tags: string[];
  extraInfo: string;
}

export interface BranchDTO {
  name: string;
  isHead: boolean;
  ahead: string;
  behind: string;
  upstream: string;
  subject: string;
}

export interface RepoSnapshot {
  repoPath: string;
  repoName: string;
  branch: string;
  isDetached: boolean;
  state: string;
  /** 提交身份；任一为空时 git 会拒绝提交 */
  identityName: string;
  identityEmail: string;
  files: FileDTO[];
  commits: CommitDTO[];
  branches: BranchDTO[];
}

export type SidebarTab = "changes" | "staged" | "branches";

export interface Selection {
  // 选中一个文件时，需要同时知道它在工作区还是暂存区，因为两者 diff 不同
  file: { path: string; staged: boolean } | null;
  commitHash: string | null;
}

/** 克隆过程中从引擎推来的进度事件。 */
export interface CloneProgress {
  /** 中文阶段名，例如「接收对象」 */
  phase: string;
  /** 0-100；-1 表示当前阶段没有百分比 */
  percent: number;
  /** git 的原始输出行 */
  detail: string;
}

/** 一个 git 操作的参数描述（与 Go 侧 engine.Param 对应）。 */
export interface Param {
  name: string;
  label: string;
  kind: "string" | "text" | "bool" | "choice" | string;
  required: boolean;
  default: string;
  placeholder: string;
  choices: string[];
  help: string;
  flag: string;
}

/** 一个 git 操作的概要。 */
export interface OperationSummary {
  id: string;
  category: string;
  name: string;
  description: string;
  params: Param[];
  dangerous: boolean;
  readOnly: boolean;
}

/** 一次操作的执行结果。 */
export interface RunResult {
  operationId: string;
  command: string;
  output: string;
  ok: boolean;
  error: string;
  snapshot: RepoSnapshot | null;
}

/** 某个提交里改动的一个文件。 */
export interface CommitFileDTO {
  path: string;
  oldPath: string;
  status: string;
  statusLabel: string;
  kind: string;
  additions: number;
  deletions: number;
}

/** 上传到托管平台的结果。 */
export interface PublishResult {
  repoUrl: string;
  cloneUrl: string;
  command: string;
  output: string;
  ok: boolean;
  error: string;
  snapshot: RepoSnapshot | null;
}
