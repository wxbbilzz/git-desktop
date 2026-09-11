// 前端与引擎之间的唯一通道。
//
// Wails 会把 app.go 上所有导出方法注入到 window.go.main.App.*，
// 所以这里不需要任何手写的 IPC 协议或代码生成产物。
//
// 关键设计：如果 window.go 不存在（也就是我们在浏览器里跑 `npm run dev`
// 单独预览界面），就自动回退到 mock.ts 的演示数据。
// 这让“改 UI”和“跑真实 git”彻底解耦 —— 调样式时完全不需要编译 Go。

import type {
  CloneProgress,
  CommitFileDTO,
  ConflictChoice,
  ConflictFile,
  FileContentDTO,
  FilePatch,
  FolderInfo,
  OperationChoices,
  OperationSummary,
  PublishDefaults,
  PublishResult,
  RepoFileDTO,
  RepoSnapshot,
  RunResult,
  StashEntryDTO,
} from "./types";
import {
  mockCheckout,
  mockCommit,
  mockDiff,
  mockDiscard,
  mockSnapshot,
  mockStage,
  mockStageAll,
  mockUnstage,
  mockUnstageAll,
} from "./mock";

interface DesktopBridge {
  Snapshot(): Promise<RepoSnapshot>;
  OpenRepo(path: string): Promise<RepoSnapshot>;
  PickRepo(): Promise<string>;
  ChooseAndOpenRepo(): Promise<RepoSnapshot | null>;

  // 仓库建立
  PickDirectory(title: string): Promise<string>;
  DefaultBaseDir(): string;
  DeriveRepoName(url: string): string;
  JoinPath(dir: string, name: string): string;
  CreateRepo(parentDir: string, name: string, initialBranch: string): Promise<RepoSnapshot>;
  CloneRepo(url: string, dest: string, depth: number): Promise<RepoSnapshot>;

  FileDiff(path: string, staged: boolean): Promise<string>;
  CommitDiff(hash: string): Promise<string>;
  CommitFiles(hash: string): Promise<CommitFileDTO[]>;
  CommitFileDiff(hash: string, path: string): Promise<string>;
  StageFile(path: string): Promise<RepoSnapshot>;
  UnstageFile(path: string): Promise<RepoSnapshot>;
  StageAll(): Promise<RepoSnapshot>;
  UnstageAll(): Promise<RepoSnapshot>;
  DiscardFile(path: string): Promise<RepoSnapshot>;
  Commit(summary: string, description: string): Promise<RepoSnapshot>;
  CheckoutBranch(name: string): Promise<RepoSnapshot>;
  CreateBranch(name: string): Promise<RepoSnapshot>;
  CreateBranchFrom(name: string, start: string, checkout: boolean): Promise<RepoSnapshot>;
  DeleteBranch(name: string, force: boolean): Promise<RepoSnapshot>;
  Fetch(): Promise<RepoSnapshot>;
  Pull(): Promise<RepoSnapshot>;
  Push(): Promise<RepoSnapshot>;

  // 提交身份
  Identity(): Promise<[string, string]>;
  SetIdentity(name: string, email: string, global: boolean): Promise<RepoSnapshot>;

  // git 全命令
  Operations(): Promise<OperationSummary[]>;
  OperationChoices(): Promise<OperationChoices>;
  RunOperation(id: string, args: Record<string, string>): Promise<RunResult>;
  RunRawGit(command: string): Promise<RunResult>;

  // 行级暂存
  FilePatchLines(path: string, staged: boolean): Promise<FilePatch>;
  StageLines(path: string, staged: boolean, lineIndices: number[]): Promise<RepoSnapshot>;

  // 冲突解决
  ReadConflictFile(path: string): Promise<ConflictFile>;
  ResolveConflicts(path: string, choices: ConflictChoice[]): Promise<RepoSnapshot>;

  // 储藏
  Stashes(): Promise<StashEntryDTO[]>;
  StashShow(index: number): Promise<string>;
  StashSave(message: string, includeUntracked: boolean): Promise<RepoSnapshot>;
  StashPop(index: number): Promise<RepoSnapshot>;
  StashApply(index: number): Promise<RepoSnapshot>;
  StashDrop(index: number): Promise<RepoSnapshot>;

  // 完整仓库文件树
  InspectFolder(path: string): Promise<FolderInfo>;
  PendingStartupFolder(): Promise<FolderInfo | null>;
  InitRepoHere(path: string, initialBranch: string): Promise<RepoSnapshot>;
  RepoFiles(): Promise<RepoFileDTO[]>;
  FileContent(path: string): Promise<FileContentDTO>;

  // 窗口控制（自绘标题栏用）
  MinimiseWindow(): void;
  ToggleMaximiseWindow(): void;
  IsWindowMaximised(): Promise<boolean>;
  CloseWindow(): void;

  // 上传到托管平台
  PublishDefaults(): Promise<PublishDefaults>;
  Publish(
    platform: string,
    mode: string,
    token: string,
    name: string,
    description: string,
    repoUrl: string,
    privateRepo: boolean,
    storeToken: boolean,
  ): Promise<PublishResult>;
}

declare global {
  interface Window {
    go?: { main?: { App?: DesktopBridge } };
    runtime?: {
      EventsOn: (name: string, cb: (...args: any[]) => void) => () => void;
    };
  }
}

function bridge(): DesktopBridge | undefined {
  if (typeof window === "undefined") return undefined;
  return window.go?.main?.App;
}

/** 是否运行在 Wails 桌面外壳里（false 表示浏览器预览模式）。 */
export function isDesktop(): boolean {
  return !!bridge();
}

/** 订阅远端操作（push/pull/fetch）的进度事件。 */
export function onSyncProgress(cb: (p: CloneProgress) => void): () => void {
  const rt = typeof window !== "undefined" ? window.runtime : undefined;
  if (!rt?.EventsOn) return () => {};
  return rt.EventsOn("sync:progress", cb as (...args: any[]) => void);
}

/** 订阅「文件夹被拖进窗口」事件；返回取消订阅的函数。 */
export function onRepoDropped(cb: (dir: string) => void): () => void {
  const rt = typeof window !== "undefined" ? window.runtime : undefined;
  if (!rt?.EventsOn) return () => {};
  return rt.EventsOn("repo:dropped", cb as (...args: any[]) => void);
}

/** 订阅「拖进来的东西不是仓库」事件。 */
export function onRepoDropFailed(
  cb: (path: string, reason: string) => void,
): () => void {
  const rt = typeof window !== "undefined" ? window.runtime : undefined;
  if (!rt?.EventsOn) return () => {};
  return rt.EventsOn("repo:drop-failed", cb as (...args: any[]) => void);
}

/** 订阅上传进度事件；返回取消订阅的函数。 */
export function onPublishProgress(cb: (step: string) => void): () => void {
  const rt = typeof window !== "undefined" ? window.runtime : undefined;
  if (!rt?.EventsOn) return () => {};
  return rt.EventsOn("publish:progress", cb as (...args: any[]) => void);
}

/** 订阅克隆进度事件；返回取消订阅的函数。 */
export function onCloneProgress(cb: (p: CloneProgress) => void): () => void {
  const rt = typeof window !== "undefined" ? window.runtime : undefined;
  if (!rt?.EventsOn) return () => {};
  return rt.EventsOn("clone:progress", cb as (...args: any[]) => void);
}

/** 浏览器预览模式下模拟的提交文件列表。 */
const MOCK_COMMIT_FILES: CommitFileDTO[] = [
  { path: "pkg/engine/session.go", oldPath: "", status: "M", statusLabel: "修改", kind: "modified", additions: 24, deletions: 6 },
  { path: "frontend/src/components/DiffPanel.tsx", oldPath: "", status: "A", statusLabel: "新增", kind: "new", additions: 96, deletions: 0 },
  { path: "old/legacy.ts", oldPath: "old/legacy.ts", status: "R100", statusLabel: "重命名", kind: "renamed", additions: 2, deletions: 2 },
];

/** 浏览器预览模式下的假路径拼接。 */
function fakeJoin(dir: string, name: string): string {
  if (!dir) return name;
  return dir.endsWith("/") ? dir + name : dir + "/" + name;
}

export const api = {
  async snapshot(): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.Snapshot() : mockSnapshot();
  },

  async pickRepo(): Promise<string> {
    const b = bridge();
    return b ? b.PickRepo() : "";
  },

  async chooseAndOpenRepo(): Promise<RepoSnapshot | null> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.ChooseAndOpenRepo();
  },

  async openRepo(path: string): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.OpenRepo(path);
  },

  // ---- 仓库建立 ----

  async pickDirectory(title: string): Promise<string> {
    const b = bridge();
    return b ? b.PickDirectory(title) : "/home/user/projects";
  },

  async defaultBaseDir(): Promise<string> {
    const b = bridge();
    return b ? b.DefaultBaseDir() : "/home/user/projects";
  },

  async deriveRepoName(url: string): Promise<string> {
    const b = bridge();
    if (b) return b.DeriveRepoName(url);
    // 浏览器预览模式下的等价实现
    let s = (url || "").trim().replace(/\/+$/, "").replace(/\.git$/, "");
    if (!s) return "";
    const colon = s.lastIndexOf(":");
    if (colon !== -1 && !s.slice(colon).includes("/")) s = s.slice(colon + 1);
    const slash = s.lastIndexOf("/");
    if (slash !== -1) s = s.slice(slash + 1);
    return s;
  },

  async joinPath(dir: string, name: string): Promise<string> {
    const b = bridge();
    return b ? b.JoinPath(dir, name) : fakeJoin(dir, name);
  },

  async createRepo(
    parentDir: string,
    name: string,
    initialBranch: string,
  ): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.CreateRepo(parentDir, name, initialBranch);
  },

  async cloneRepo(url: string, dest: string, depth: number): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.CloneRepo(url, dest, depth);
  },

  // ---- 读写 ----

  async fileDiff(path: string, staged: boolean): Promise<string> {
    const b = bridge();
    return b ? b.FileDiff(path, staged) : mockDiff(path, staged);
  },

  async commitDiff(hash: string): Promise<string> {
    const b = bridge();
    return b ? b.CommitDiff(hash) : mockDiff(hash, false);
  },

  async commitFiles(hash: string): Promise<CommitFileDTO[]> {
    const b = bridge();
    if (!b) return MOCK_COMMIT_FILES;
    return b.CommitFiles(hash);
  },

  async commitFileDiff(hash: string, path: string): Promise<string> {
    const b = bridge();
    if (!b) return mockDiff(path, false);
    return b.CommitFileDiff(hash, path);
  },

  async stageFile(path: string): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.StageFile(path) : mockStage(path);
  },

  async unstageFile(path: string): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.UnstageFile(path) : mockUnstage(path);
  },

  async stageAll(): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.StageAll() : mockStageAll();
  },

  async unstageAll(): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.UnstageAll() : mockUnstageAll();
  },

  async discardFile(path: string): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.DiscardFile(path) : mockDiscard(path);
  },

  async commit(summary: string, description: string): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.Commit(summary, description) : mockCommit(summary);
  },

  async checkoutBranch(name: string): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.CheckoutBranch(name) : mockCheckout(name);
  },

  async createBranch(name: string): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.CreateBranch(name) : mockCheckout(name);
  },

  async createBranchFrom(
    name: string,
    start: string,
    checkout: boolean,
  ): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockCheckout(name);
    return b.CreateBranchFrom(name, start, checkout);
  },

  async deleteBranch(name: string, force: boolean): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.DeleteBranch(name, force);
  },

  async fetch(): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.Fetch() : mockSnapshot();
  },

  async pull(): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.Pull() : mockSnapshot();
  },

  async push(): Promise<RepoSnapshot> {
    const b = bridge();
    return b ? b.Push() : mockSnapshot();
  },

  // ---- 提交身份 ----

  async setIdentity(
    name: string,
    email: string,
    global: boolean,
  ): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.SetIdentity(name, email, global);
  },

  // ---- git 全命令 ----

  async operations(): Promise<OperationSummary[]> {
    const b = bridge();
    return b ? b.Operations() : [];
  },

  async operationChoices(): Promise<OperationChoices> {
    const b = bridge();
    if (!b) {
      return {
        branches: [], refs: [], commits: [], files: [], remotes: [], tags: [], stashes: [],
      };
    }
    return b.OperationChoices();
  },

  async runOperation(
    id: string,
    args: Record<string, string>,
  ): Promise<RunResult> {
    const b = bridge();
    if (!b) {
      return {
        operationId: id,
        command: "git（演示模式不执行）",
        output: "浏览器预览模式：这里不会真正执行 git 命令。",
        ok: true,
        error: "",
        snapshot: null,
      };
    }
    return b.RunOperation(id, args);
  },

  async runRawGit(command: string): Promise<RunResult> {
    const b = bridge();
    if (!b) {
      return {
        operationId: "raw",
        command: "git " + command,
        output: "浏览器预览模式：这里不会真正执行 git 命令。",
        ok: true,
        error: "",
        snapshot: null,
      };
    }
    return b.RunRawGit(command);
  },

  async publishDefaults(): Promise<PublishDefaults> {
    const b = bridge();
    if (!b) return { remoteUrl: "", remoteName: "", repoName: "" };
    return b.PublishDefaults();
  },

  async publish(
    platform: string,
    mode: string,
    token: string,
    name: string,
    description: string,
    repoUrl: string,
    privateRepo: boolean,
    storeToken: boolean,
  ): Promise<PublishResult> {
    const b = bridge();
    if (!b) {
      return {
        repoUrl: "https://example.com/demo",
        cloneUrl: "",
        command: "git push（演示模式不执行）",
        output: "浏览器预览模式：这里不会真正上传。",
        ok: true,
        error: "",
        snapshot: null,
      };
    }
    return b.Publish(
      platform,
      mode,
      token,
      name,
      description,
      repoUrl,
      privateRepo,
      storeToken,
    );
  },

  // ---- 行级暂存 ----

  async filePatchLines(path: string, staged: boolean): Promise<FilePatch> {
    const b = bridge();
    if (!b) return { path, staged, lines: [], hasChanges: false };
    return b.FilePatchLines(path, staged);
  },

  async stageLines(
    path: string,
    staged: boolean,
    lineIndices: number[],
  ): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.StageLines(path, staged, lineIndices);
  },

  // ---- 冲突解决 ----

  async readConflictFile(path: string): Promise<ConflictFile> {
    const b = bridge();
    if (!b) return { path, lines: [], blocks: [], markerSize: 7 };
    return b.ReadConflictFile(path);
  },

  async resolveConflicts(
    path: string,
    choices: ConflictChoice[],
  ): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.ResolveConflicts(path, choices);
  },

  // ---- 储藏 ----

  async stashes(): Promise<StashEntryDTO[]> {
    const b = bridge();
    return b ? b.Stashes() : [];
  },

  async stashShow(index: number): Promise<string> {
    const b = bridge();
    if (!b) return mockDiff("stash", false);
    return b.StashShow(index);
  },

  async stashSave(message: string, includeUntracked: boolean): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.StashSave(message, includeUntracked);
  },

  async stashPop(index: number): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.StashPop(index);
  },

  async stashApply(index: number): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.StashApply(index);
  },

  async stashDrop(index: number): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.StashDrop(index);
  },

  async repoFiles(): Promise<RepoFileDTO[]> {
    const b = bridge();
    if (!b) return [];
    return b.RepoFiles();
  },

  async fileContent(path: string): Promise<FileContentDTO> {
    const b = bridge();
    if (!b) {
      return {
        path,
        content: "",
        binary: false,
        truncated: false,
        lines: 0,
        size: 0,
        fromIndex: false,
      };
    }
    return b.FileContent(path);
  },

  // ---- 窗口控制 ----

  async minimiseWindow(): Promise<void> {
    bridge()?.MinimiseWindow();
  },

  async toggleMaximiseWindow(): Promise<void> {
    bridge()?.ToggleMaximiseWindow();
  },

  async isWindowMaximised(): Promise<boolean> {
    const b = bridge();
    return b ? b.IsWindowMaximised() : false;
  },

  async closeWindow(): Promise<void> {
    bridge()?.CloseWindow();
  },

  // ---- 打开「可能还不是仓库」的文件夹 ----

  async inspectFolder(path: string): Promise<FolderInfo> {
    const b = bridge();
    if (!b) return { path, isRepo: true, parentRepo: "", fileCount: 0 };
    return b.InspectFolder(path);
  },

  async initRepoHere(path: string, initialBranch: string): Promise<RepoSnapshot> {
    const b = bridge();
    if (!b) return mockSnapshot();
    return b.InitRepoHere(path, initialBranch);
  },

  async pendingStartupFolder(): Promise<FolderInfo | null> {
    const b = bridge();
    if (!b) return null;
    return b.PendingStartupFolder();
  },
};
