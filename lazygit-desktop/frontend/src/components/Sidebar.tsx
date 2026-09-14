import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import type {
  BranchDTO,
  FileDTO,
  RemoteBranchDTO,
  RemoteDTO,
  RepoFileDTO,
  RepoSnapshot,
  SidebarTab,
} from "../types";
import { PillButton } from "./PillButton";
import { FileTree } from "./FileTree";
import { RepoFileTree } from "./RepoFileTree";
import { FilterBox, RemotesPanel, StashPanel, TagsPanel } from "./SidebarPanels";
import type { MenuSpec } from "./ContextMenu";
import {
  IconArchive,
  IconBranch,
  IconCheck,
  IconCherry,
  IconCloud,
  IconCopy,
  IconFolder,
  IconLayers,
  IconMerge,
  IconMinus,
  IconPencil,
  IconPlus,
  IconPull,
  IconPush,
  IconTag,
  IconTrash,
  IconTree,
  IconUndo,
} from "./icons";

interface Props {
  snapshot: RepoSnapshot;
  tab: SidebarTab;
  onTabChange: (tab: SidebarTab) => void;
  selectedPath: string | null;
  selectedStaged: boolean;
  busy: string | null;
  onSelectFile: (path: string, staged: boolean) => void;
  onStage: (path: string) => void;
  onUnstage: (path: string) => void;
  onStageAll: () => void;
  onUnstageAll: () => void;
  onDiscard: (path: string) => void;
  // 分支
  onCheckoutBranch: (name: string) => void;
  onCreateBranch: (name: string, start: string, checkout: boolean) => void;
  onDeleteBranch: (name: string) => void;
  onRenameBranch: (oldName: string, newName: string) => void;
  onMergeBranch: (name: string) => void;
  onRebaseOnto: (name: string) => void;
  onCheckoutRemoteBranch: (remoteBranch: string, local: string) => void;
  // 标签 / 远端 / 储藏
  onTagCreate: (name: string, ref: string, message: string) => void;
  onTagDelete: (name: string) => void;
  onTagPush: (name: string, remote: string) => void;
  onTagPushAll: (remote: string) => void;
  onRemoteAdd: (name: string, url: string) => void;
  onRemoteRemove: (name: string) => void;
  onRemoteSetUrl: (name: string, url: string) => void;
  onStashSave: (message: string, includeUntracked: boolean) => void;
  onStashPop: (index: number) => void;
  onStashApply: (index: number) => void;
  onStashDrop: (index: number) => void;
  onStashShow: (index: number) => Promise<string>;
  // 右键菜单与复制
  onMenu: (spec: MenuSpec) => void;
  onCopy: (text: string, label: string) => void;
  /** 从历史面板「以此为起点新建分支」带过来的起点提交号 */
  branchStart?: string | null;
  onBranchStartUsed: () => void;
  /** 从历史面板「给这次提交打标签」带过来的提交号 */
  tagRef?: string | null;
  onTagRefUsed: () => void;
  // 完整仓库文件树
  repoFiles: RepoFileDTO[];
  selectedRepoFile: string | null;
  onSelectRepoFile: (path: string) => void;
}

function splitPath(path: string): { name: string; dir: string } {
  const parts = path.split("/");
  const name = parts.pop() ?? path;
  return { name, dir: parts.join("/") };
}

/** 复制类菜单项，避免在每个菜单里重复写。 */
function copyItems(text: string, label: string, onCopy: (t: string, l: string) => void) {
  return [
    { label: `复制${label}`, icon: <IconCopy />, onClick: () => onCopy(text, label) },
  ];
}

function FileRow({
  file,
  staged,
  selected,
  busy,
  onSelect,
  onStage,
  onUnstage,
  onDiscard,
  onMenu,
  onCopy,
}: {
  file: FileDTO;
  staged: boolean;
  selected: boolean;
  busy: string | null;
  onSelect: () => void;
  onStage: () => void;
  onUnstage: () => void;
  onDiscard: () => void;
  onMenu: (spec: MenuSpec) => void;
  onCopy: (text: string, label: string) => void;
}) {
  const { name, dir } = splitPath(file.path);

  const openMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const items = staged
      ? [
          {
            label: "取消暂存",
            icon: <IconMinus />,
            disabled: busy !== null,
            onClick: onUnstage,
          },
        ]
      : [
          {
            label: "暂存",
            icon: <IconPlus />,
            disabled: busy !== null,
            onClick: onStage,
          },
          {
            label: "丢弃改动",
            icon: <IconTrash />,
            danger: true,
            disabled: busy !== null,
            onClick: onDiscard,
          },
        ];
    onMenu({
      x: e.clientX,
      y: e.clientY,
      items: [
        ...items,
        { label: "", separator: true },
        ...copyItems(file.path, "路径", onCopy),
        ...(dir
          ? [
              {
                label: "复制所在目录",
                icon: <IconCopy />,
                onClick: () => onCopy(dir, "目录"),
              },
            ]
          : []),
      ],
    });
  };

  return (
    <div
      className={"row" + (selected ? " selected" : "")}
      onClick={onSelect}
      onContextMenu={openMenu}
    >
      <span className={"dot " + file.kind} />

      <div className="row-main">
        <div className="row-name" title={file.path}>
          {name}
          {file.previousPath && <span style={{ color: "var(--muted)" }}> ← </span>}
        </div>
        {dir && <div className="row-sub">{dir}</div>}
      </div>

      {(file.linesAdded > 0 || file.linesDeleted > 0) && (
        <span className="stat">
          <span className="add">+{file.linesAdded}</span>{" "}
          <span className="del">-{file.linesDeleted}</span>
        </span>
      )}

      <span className={"badge " + file.kind}>{file.statusLabel}</span>

      {/* 悬停才出现的行内操作，避免列表过于嘈杂 */}
      <div className="row-actions" onClick={(e) => e.stopPropagation()}>
        {staged ? (
          <PillButton
            size="sm"
            variant="ghost"
            icon={<IconMinus />}
            title="取消暂存"
            disabled={busy !== null}
            onClick={onUnstage}
          />
        ) : (
          <PillButton
            size="sm"
            variant="success"
            icon={<IconPlus />}
            title="暂存"
            disabled={busy !== null}
            onClick={onStage}
          />
        )}
        {!staged && (
          <PillButton
            size="sm"
            variant="danger"
            icon={<IconTrash />}
            title="丢弃改动"
            disabled={busy !== null}
            onClick={onDiscard}
          />
        )}
      </div>
    </div>
  );
}

function BranchRow({
  branch,
  busy,
  onCheckout,
  onNewFrom,
  onDelete,
  onMerge,
  onRebase,
  onRename,
  onMenu,
  onCopy,
}: {
  branch: BranchDTO;
  busy: string | null;
  onCheckout: () => void;
  onNewFrom: () => void;
  onDelete: () => void;
  onMerge: () => void;
  onRebase: () => void;
  onRename: () => void;
  onMenu: (spec: MenuSpec) => void;
  onCopy: (text: string, label: string) => void;
}) {
  // 当前分支上不能「合并自己」「变基到自己」，菜单里也不该出现这两项
  const other = !branch.isHead;

  const openMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    onMenu({
      x: e.clientX,
      y: e.clientY,
      items: [
        {
          label: other ? "切换到此分支" : "重新检出当前分支",
          icon: <IconCheck />,
          disabled: busy !== null,
          onClick: onCheckout,
        },
        {
          label: "以此为起点新建分支",
          icon: <IconPlus />,
          disabled: busy !== null,
          onClick: onNewFrom,
        },
        // 这三个动作的参数就是「用户点的那个分支」，所以直接放进右键菜单，
        // 不用先去命令面板搜操作、再手填分支名
        ...(other
          ? [
              {
                label: `合并到当前分支`,
                icon: <IconMerge />,
                disabled: busy !== null,
                onClick: onMerge,
              },
              {
                label: `把当前分支变基到 ${branch.name}`,
                icon: <IconCherry />,
                disabled: busy !== null,
                onClick: onRebase,
              },
            ]
          : []),
        { label: "", separator: true },
        {
          label: "重命名",
          icon: <IconPencil />,
          disabled: busy !== null,
          onClick: onRename,
        },
        ...copyItems(branch.name, "分支名", onCopy),
        { label: "", separator: true },
        ...(other
          ? [
              {
                label: "删除分支",
                icon: <IconTrash />,
                danger: true,
                disabled: busy !== null,
                onClick: onDelete,
              },
            ]
          : []),
      ],
    });
  };

  return (
    <div
      className={"row" + (branch.isHead ? " selected" : "")}
      onContextMenu={openMenu}
    >
      <span className={"dot " + (branch.isHead ? "new" : "modified")} />

      <div className="row-main">
        <div className="row-name">{branch.name}</div>
        <div className="row-sub">
          {branch.upstream || "（无上游）"}
          {branch.subject ? ` · ${branch.subject}` : ""}
        </div>
      </div>

      {branch.ahead && <span className="branch-ahead">↑{branch.ahead}</span>}
      {branch.behind && <span className="branch-behind">↓{branch.behind}</span>}

      <div className="row-actions" onClick={(e) => e.stopPropagation()}>
        <PillButton
          size="sm"
          variant="ghost"
          icon={<IconPlus />}
          title="以这个分支为起点新建分支"
          disabled={busy !== null}
          onClick={onNewFrom}
        />
        {other && (
          <>
            <PillButton
              size="sm"
              variant="ghost"
              icon={<IconCheck />}
              title="切换到此分支"
              disabled={busy !== null}
              onClick={onCheckout}
            />
            <PillButton
              size="sm"
              variant="danger"
              icon={<IconTrash />}
              title="删除这个分支"
              disabled={busy !== null}
              onClick={onDelete}
            />
          </>
        )}
      </div>
    </div>
  );
}

function RemoteBranchRow({
  branch,
  busy,
  onCheckout,
  onMenu,
  onCopy,
}: {
  branch: RemoteBranchDTO;
  busy: string | null;
  onCheckout: () => void;
  onMenu: (spec: MenuSpec) => void;
  onCopy: (text: string, label: string) => void;
}) {
  const openMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    onMenu({
      x: e.clientX,
      y: e.clientY,
      items: [
        {
          label: branch.hasLocal ? "切换到本地同名分支" : "检出为本地分支",
          icon: <IconCheck />,
          disabled: busy !== null,
          onClick: onCheckout,
        },
        { label: "", separator: true },
        ...copyItems(branch.name, "远端分支名", onCopy),
      ],
    });
  };

  return (
    <div className="row" onContextMenu={openMenu}>
      <span className="dot untracked" />

      <div className="row-main">
        <div className="row-name" title={branch.name}>
          {branch.short}
          {branch.isCurrentUpstream && (
            <span className="badge new" style={{ marginLeft: 6 }}>
              上游
            </span>
          )}
        </div>
        <div className="row-sub">
          {branch.remote} · {branch.shortHash} · {branch.when}
        </div>
      </div>

      <div className="row-actions" onClick={(e) => e.stopPropagation()}>
        <PillButton
          size="sm"
          variant="ghost"
          icon={<IconPull />}
          title={branch.hasLocal ? "切换到本地同名分支" : "检出为本地分支"}
          disabled={busy !== null}
          onClick={onCheckout}
        />
      </div>
    </div>
  );
}

export function Sidebar({
  snapshot,
  tab,
  onTabChange,
  selectedPath,
  selectedStaged,
  busy,
  onSelectFile,
  onStage,
  onUnstage,
  onStageAll,
  onUnstageAll,
  onDiscard,
  onCheckoutBranch,
  onCreateBranch,
  onDeleteBranch,
  onRenameBranch,
  onMergeBranch,
  onRebaseOnto,
  onCheckoutRemoteBranch,
  onTagCreate,
  onTagDelete,
  onTagPush,
  onTagPushAll,
  onRemoteAdd,
  onRemoteRemove,
  onRemoteSetUrl,
  onStashSave,
  onStashPop,
  onStashApply,
  onStashDrop,
  onStashShow,
  onMenu,
  onCopy,
  branchStart,
  onBranchStartUsed,
  tagRef,
  onTagRefUsed,
  repoFiles,
  selectedRepoFile,
  onSelectRepoFile,
}: Props) {
  // 文件视图：目录树 或 平铺列表
  const [viewMode, setViewMode] = useState<"tree" | "flat">("tree");
  // 新建分支表单
  const [newBranchOpen, setNewBranchOpen] = useState(false);
  const [newBranchName, setNewBranchName] = useState("");
  const [newBranchStart, setNewBranchStart] = useState("");
  // 重命名分支
  const [renaming, setRenaming] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");
  // 各列表的过滤词
  const [fileFilter, setFileFilter] = useState("");
  const [stagedFilter, setStagedFilter] = useState("");
  const [branchFilter, setBranchFilter] = useState("");
  const [remoteBranchFilter, setRemoteBranchFilter] = useState("");
  const [repoFileFilter, setRepoFileFilter] = useState("");

  const changedFiles = useMemo(
    () => matchFiles(
      snapshot.files.filter((f) => f.isUnstaged || (!f.isStaged && !f.isUnstaged)),
      fileFilter,
    ),
    [snapshot.files, fileFilter],
  );
  const stagedFiles = useMemo(
    () => matchFiles(snapshot.files.filter((f) => f.isStaged), stagedFilter),
    [snapshot.files, stagedFilter],
  );
  const branches = useMemo(() => {
    const q = branchFilter.trim().toLowerCase();
    if (!q) return snapshot.branches;
    return snapshot.branches.filter(
      (b) =>
        b.name.toLowerCase().includes(q) ||
        b.subject.toLowerCase().includes(q) ||
        b.upstream.toLowerCase().includes(q),
    );
  }, [snapshot.branches, branchFilter]);
  const remoteBranches = useMemo(() => {
    const q = remoteBranchFilter.trim().toLowerCase();
    if (!q) return snapshot.remoteBranches;
    return snapshot.remoteBranches.filter(
      (b) =>
        b.name.toLowerCase().includes(q) || b.subject.toLowerCase().includes(q),
    );
  }, [snapshot.remoteBranches, remoteBranchFilter]);
  const files = useMemo(() => {
    const q = repoFileFilter.trim().toLowerCase();
    if (!q) return repoFiles;
    return repoFiles.filter((f) => f.path.toLowerCase().includes(q));
  }, [repoFiles, repoFileFilter]);

  const changedCount = snapshot.files.filter(
    (f) => f.isUnstaged || (!f.isStaged && !f.isUnstaged),
  ).length;
  const stagedCount = snapshot.files.filter((f) => f.isStaged).length;

  // 从历史面板点「以此为起点新建分支」时，自动打开表单并填好起点
  useEffect(() => {
    if (branchStart) {
      setNewBranchStart(branchStart);
      setNewBranchOpen(true);
      onBranchStartUsed();
    }
  }, [branchStart, onBranchStartUsed]);

  // 侧栏标签改成了左侧竖向导轨，只放图标；名称和数量走悬停气泡。
  // 之所以不再横排：7 个标签挤在 320px 宽的侧栏里，每个只剩约 40px，
  // 文字全被省略号截断，还白占掉一整行高度。
  const railTabs: {
    id: SidebarTab;
    label: string;
    count?: number;
    icon: ReactNode;
  }[] = [
    { id: "changes", label: "变更", count: changedCount, icon: <IconPencil /> },
    { id: "staged", label: "暂存", count: stagedCount, icon: <IconCheck /> },
    {
      id: "branches",
      label: "分支",
      count: snapshot.branches.length,
      icon: <IconBranch />,
    },
    { id: "tags", label: "标签", count: snapshot.tags.length, icon: <IconTag /> },
    {
      id: "remotes",
      label: "远端",
      count: snapshot.remotes.length,
      icon: <IconCloud />,
    },
    { id: "stashes", label: "储藏", icon: <IconArchive /> },
    { id: "files", label: "文件", count: repoFiles.length, icon: <IconFolder /> },
  ];

  return (
    <section className="panel side-panel">
      <nav className="side-rail">
        {railTabs.map((t) => (
          <button
            key={t.id}
            type="button"
            className={"rail-tab" + (tab === t.id ? " active" : "")}
            data-label={t.count ? `${t.label} ${t.count}` : t.label}
            aria-label={t.label}
            aria-current={tab === t.id}
            onClick={() => onTabChange(t.id)}
          >
            {t.icon}
            {!!t.count && (
              <span className="count">{t.count > 99 ? "99+" : t.count}</span>
            )}
          </button>
        ))}
      </nav>

      <div className="panel-body">
        {tab === "changes" && (
          <>
            <div className="list-actions">
              <PillButton
                size="sm"
                variant="success"
                icon={<IconPlus />}
                disabled={busy !== null || changedCount === 0}
                onClick={onStageAll}
              >
                全部暂存
              </PillButton>
              <PillButton
                size="sm"
                variant="ghost"
                icon={viewMode === "tree" ? <IconLayers /> : <IconTree />}
                title={viewMode === "tree" ? "切换为平铺列表" : "切换为目录树"}
                onClick={() => setViewMode(viewMode === "tree" ? "flat" : "tree")}
              />
            </div>
            {changedCount > 3 && (
              <FilterBox
                value={fileFilter}
                onChange={setFileFilter}
                placeholder="过滤文件…"
              />
            )}
            {viewMode === "tree" ? (
              <FileTree
                files={changedFiles}
                staged={false}
                selectedPath={selectedPath}
                selectedStaged={selectedStaged}
                busy={busy}
                onSelectFile={onSelectFile}
                onStage={onStage}
                onUnstage={onUnstage}
                onDiscard={onDiscard}
              />
            ) : (
              <div className="list">
                {changedFiles.map((f) => (
                  <FileRow
                    key={f.path}
                    file={f}
                    staged={false}
                    busy={busy}
                    selected={selectedPath === f.path && !selectedStaged}
                    onSelect={() => onSelectFile(f.path, false)}
                    onStage={() => onStage(f.path)}
                    onUnstage={() => onUnstage(f.path)}
                    onDiscard={() => onDiscard(f.path)}
                    onMenu={onMenu}
                    onCopy={onCopy}
                  />
                ))}
                {changedCount === 0 && (
                  <div className="empty">
                    <div className="empty-title">工作区是干净的</div>
                    <div className="empty-text">没有未暂存的改动。</div>
                  </div>
                )}
                {changedCount > 0 && changedFiles.length === 0 && (
                  <div className="empty">
                    <div className="empty-title">没有匹配的文件</div>
                    <div className="empty-text">换个关键词试试。</div>
                  </div>
                )}
              </div>
            )}
          </>
        )}

        {tab === "staged" && (
          <>
            <div className="list-actions">
              <PillButton
                size="sm"
                icon={<IconUndo />}
                disabled={busy !== null || stagedCount === 0}
                onClick={onUnstageAll}
              >
                全部取消暂存
              </PillButton>
              <PillButton
                size="sm"
                variant="ghost"
                icon={viewMode === "tree" ? <IconLayers /> : <IconTree />}
                title={viewMode === "tree" ? "切换为平铺列表" : "切换为目录树"}
                onClick={() => setViewMode(viewMode === "tree" ? "flat" : "tree")}
              />
            </div>
            {stagedCount > 3 && (
              <FilterBox
                value={stagedFilter}
                onChange={setStagedFilter}
                placeholder="过滤文件…"
              />
            )}
            {viewMode === "tree" ? (
              <FileTree
                files={stagedFiles}
                staged
                selectedPath={selectedPath}
                selectedStaged={selectedStaged}
                busy={busy}
                onSelectFile={onSelectFile}
                onStage={onStage}
                onUnstage={onUnstage}
                onDiscard={onDiscard}
              />
            ) : (
              <div className="list">
                {stagedFiles.map((f) => (
                  <FileRow
                    key={f.path}
                    file={f}
                    staged
                    busy={busy}
                    selected={selectedPath === f.path && selectedStaged}
                    onSelect={() => onSelectFile(f.path, true)}
                    onStage={() => onStage(f.path)}
                    onUnstage={() => onUnstage(f.path)}
                    onDiscard={() => onDiscard(f.path)}
                    onMenu={onMenu}
                    onCopy={onCopy}
                  />
                ))}
                {stagedCount === 0 && (
                  <div className="empty">
                    <div className="empty-title">暂存区是空的</div>
                    <div className="empty-text">
                      在「变更」里点 + 号把改动放进暂存区。
                    </div>
                  </div>
                )}
              </div>
            )}
          </>
        )}

        {tab === "branches" && (
          <>
            {newBranchOpen ? (
              <div className="branch-form">
                <label className="label">新分支名</label>
                <input
                  className="field mono"
                  placeholder="feature/xxx"
                  value={newBranchName}
                  disabled={busy !== null}
                  spellCheck={false}
                  autoFocus
                  onChange={(e) => setNewBranchName(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && newBranchName.trim()) {
                      onCreateBranch(newBranchName.trim(), newBranchStart.trim(), true);
                      setNewBranchOpen(false);
                      setNewBranchName("");
                      setNewBranchStart("");
                    }
                  }}
                />
                <label className="label">起点分支</label>
                <input
                  className="field mono"
                  placeholder="留空 = 当前分支"
                  value={newBranchStart}
                  disabled={busy !== null}
                  spellCheck={false}
                  onChange={(e) => setNewBranchStart(e.target.value)}
                />
                <div className="branch-form-actions">
                  <PillButton
                    size="sm"
                    variant="primary"
                    disabled={busy !== null || !newBranchName.trim()}
                    onClick={() => {
                      onCreateBranch(newBranchName.trim(), newBranchStart.trim(), true);
                      setNewBranchOpen(false);
                      setNewBranchName("");
                      setNewBranchStart("");
                    }}
                  >
                    创建并切换
                  </PillButton>
                  <PillButton
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      setNewBranchOpen(false);
                      setNewBranchName("");
                      setNewBranchStart("");
                    }}
                  >
                    取消
                  </PillButton>
                </div>
              </div>
            ) : (
              <div className="list-actions">
                <PillButton
                  size="sm"
                  variant="success"
                  icon={<IconPlus />}
                  disabled={busy !== null}
                  onClick={() => {
                    setNewBranchStart("");
                    setNewBranchOpen(true);
                  }}
                >
                  新建分支
                </PillButton>
              </div>
            )}

            {(snapshot.branches.length > 5 || branchFilter) && (
              <FilterBox
                value={branchFilter}
                onChange={setBranchFilter}
                placeholder="过滤分支…"
              />
            )}

            <div className="list">
              {branches.map((b) => {
                if (renaming === b.name) {
                  return (
                    <div className="branch-form" key={b.name}>
                      <label className="label">重命名分支</label>
                      <input
                        className="field mono"
                        value={renameValue}
                        autoFocus
                        disabled={busy !== null}
                        spellCheck={false}
                        onChange={(e) => setRenameValue(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" && renameValue.trim()) {
                            onRenameBranch(b.name, renameValue.trim());
                            setRenaming(null);
                          }
                          if (e.key === "Escape") setRenaming(null);
                        }}
                      />
                      <div className="branch-form-actions">
                        <PillButton
                          size="sm"
                          variant="primary"
                          disabled={busy !== null || !renameValue.trim()}
                          onClick={() => {
                            onRenameBranch(b.name, renameValue.trim());
                            setRenaming(null);
                          }}
                        >
                          保存
                        </PillButton>
                        <PillButton
                          size="sm"
                          variant="ghost"
                          onClick={() => setRenaming(null)}
                        >
                          取消
                        </PillButton>
                      </div>
                    </div>
                  );
                }
                return (
                  <BranchRow
                    key={b.name}
                    branch={b}
                    busy={busy}
                    onCheckout={() => onCheckoutBranch(b.name)}
                    onNewFrom={() => {
                      setNewBranchStart(b.name);
                      setNewBranchOpen(true);
                    }}
                    onDelete={() => onDeleteBranch(b.name)}
                    onMerge={() => onMergeBranch(b.name)}
                    onRebase={() => onRebaseOnto(b.name)}
                    onRename={() => {
                      setRenaming(b.name);
                      setRenameValue(b.name);
                    }}
                    onMenu={onMenu}
                    onCopy={onCopy}
                  />
                );
              })}
              {snapshot.branches.length === 0 && (
                <div className="empty">
                  <div className="empty-title">没有分支</div>
                  <span className="empty-text">
                    <IconBranch /> 这个仓库还没有任何本地分支。
                  </span>
                </div>
              )}
            </div>

            {/* 远端分支：以前完全看不到，想检出只能去命令目录手敲 */}
            {snapshot.remoteBranches.length > 0 && (
              <>
                <div className="list-section">
                  <IconCloud /> 远端分支
                  <span className="count">{snapshot.remoteBranches.length}</span>
                </div>
                {snapshot.remoteBranches.length > 5 && (
                  <FilterBox
                    value={remoteBranchFilter}
                    onChange={setRemoteBranchFilter}
                    placeholder="过滤远端分支…"
                  />
                )}
                <div className="list">
                  {remoteBranches.map((rb) => (
                    <RemoteBranchRow
                      key={rb.name}
                      branch={rb}
                      busy={busy}
                      onCheckout={() =>
                        onCheckoutRemoteBranch(rb.name, rb.hasLocal ? rb.short : "")
                      }
                      onMenu={onMenu}
                      onCopy={onCopy}
                    />
                  ))}
                </div>
              </>
            )}
          </>
        )}

        {tab === "tags" && (
          <TagsPanel
            snapshot={snapshot}
            busy={busy}
            presetRef={tagRef}
            onCreate={onTagCreate}
            onDelete={onTagDelete}
            onPush={onTagPush}
            onPushAll={onTagPushAll}
          />
        )}

        {tab === "remotes" && (
          <RemotesPanel
            remotes={snapshot.remotes as RemoteDTO[]}
            busy={busy}
            onAdd={onRemoteAdd}
            onRemove={onRemoteRemove}
            onSetUrl={onRemoteSetUrl}
          />
        )}

        {tab === "stashes" && (
          <StashPanel
            busy={busy}
            onSave={onStashSave}
            onPop={onStashPop}
            onApply={onStashApply}
            onDrop={onStashDrop}
            onShow={onStashShow}
          />
        )}

        {tab === "files" && (
          <>
            {repoFiles.length > 0 && (
              <FilterBox
                value={repoFileFilter}
                onChange={setRepoFileFilter}
                placeholder="按路径过滤文件…"
              />
            )}
            {repoFiles.length === 0 ? (
              <div className="empty">
                <div className="empty-title">正在读取文件列表…</div>
                <div className="empty-text">
                  数量：{repoFiles.length}（若一直为 0，说明没取到数据）
                </div>
              </div>
            ) : (
              <RepoFileTree
                files={files}
                selectedPath={selectedRepoFile}
                onSelectFile={onSelectRepoFile}
              />
            )}
          </>
        )}
      </div>
    </section>
  );
}

/** 按路径 / 状态标签过滤文件。 */
function matchFiles(files: FileDTO[], filter: string): FileDTO[] {
  const q = filter.trim().toLowerCase();
  if (!q) return files;
  return files.filter(
    (f) =>
      f.path.toLowerCase().includes(q) ||
      f.statusLabel.toLowerCase().includes(q) ||
      f.kind.toLowerCase().includes(q),
  );
}
