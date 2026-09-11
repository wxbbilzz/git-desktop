import { useState } from "react";
import type { BranchDTO, FileDTO, RepoFileDTO, RepoSnapshot, SidebarTab } from "../types";
import { PillButton } from "./PillButton";
import { FileTree } from "./FileTree";
import { RepoFileTree } from "./RepoFileTree";
import {
  IconBranch,
  IconCheck,
  IconMinus,
  IconPlus,
  IconTrash,
  IconLayers,
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
  onCheckoutBranch: (name: string) => void;
  onCreateBranch: (name: string, start: string, checkout: boolean) => void;
  onDeleteBranch: (name: string) => void;
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

function FileRow({
  file,
  staged,
  selected,
  busy,
  onSelect,
  onStage,
  onUnstage,
  onDiscard,
}: {
  file: FileDTO;
  staged: boolean;
  selected: boolean;
  busy: string | null;
  onSelect: () => void;
  onStage: () => void;
  onUnstage: () => void;
  onDiscard: () => void;
}) {
  const { name, dir } = splitPath(file.path);

  return (
    <div className={"row" + (selected ? " selected" : "")} onClick={onSelect}>
      <span className={"dot " + file.kind} />

      <div className="row-main">
        <div className="row-name" title={file.path}>
          {name}
          {file.previousPath && (
            <span style={{ color: "var(--muted)" }}> ← </span>
          )}
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
}: {
  branch: BranchDTO;
  busy: string | null;
  onCheckout: () => void;
  onNewFrom: () => void;
  onDelete: () => void;
}) {
  return (
    <div className={"row" + (branch.isHead ? " selected" : "")}>
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
        {!branch.isHead && (
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

  const changedFiles = snapshot.files.filter(
    (f) => f.isUnstaged || (!f.isStaged && !f.isUnstaged),
  );
  const stagedFiles = snapshot.files.filter((f) => f.isStaged);

  return (
    <section className="panel">
      <div className="panel-header">
        <div className="tabs">
          <button
            className={"tab" + (tab === "changes" ? " active" : "")}
            onClick={() => onTabChange("changes")}
          >
            工作区<span className="count">{changedFiles.length}</span>
          </button>
          <button
            className={"tab" + (tab === "staged" ? " active" : "")}
            onClick={() => onTabChange("staged")}
          >
            暂存区<span className="count">{stagedFiles.length}</span>
          </button>
          <button
            className={"tab" + (tab === "branches" ? " active" : "")}
            onClick={() => onTabChange("branches")}
          >
            分支<span className="count">{snapshot.branches.length}</span>
          </button>
          <button
            className={"tab" + (tab === "files" ? " active" : "")}
            onClick={() => onTabChange("files")}
          >
            文件<span className="count">{repoFiles.length}</span>
          </button>
        </div>

        {tab !== "branches" && (
          <PillButton
            size="sm"
            variant="ghost"
            icon={viewMode === "tree" ? <IconLayers /> : <IconTree />}
            title={viewMode === "tree" ? "切换为平铺列表" : "切换为目录树"}
            onClick={() => setViewMode(viewMode === "tree" ? "flat" : "tree")}
          />
        )}
      </div>

      <div className="panel-body">
        {tab === "changes" && (
          <>
            <div style={{ padding: 8 }}>
              <PillButton
                size="sm"
                variant="success"
                icon={<IconPlus />}
                disabled={busy !== null || changedFiles.length === 0}
                onClick={onStageAll}
                style={{ width: "100%" }}
              >
                全部暂存
              </PillButton>
            </div>
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
                />
              ))}
              {changedFiles.length === 0 && (
                <div className="empty">
                  <div className="empty-title">工作区是干净的</div>
                  <div className="empty-text">没有未暂存的改动。</div>
                </div>
              )}
            </div>
            )}
          </>
        )}

        {tab === "staged" && (
          <>
            <div style={{ padding: 8 }}>
              <PillButton
                size="sm"
                icon={<IconUndo />}
                disabled={busy !== null || stagedFiles.length === 0}
                onClick={onUnstageAll}
                style={{ width: "100%" }}
              >
                全部取消暂存
              </PillButton>
            </div>
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
                />
              ))}
              {stagedFiles.length === 0 && (
                <div className="empty">
                  <div className="empty-title">暂存区是空的</div>
                  <div className="empty-text">
                    在「工作区」里点 + 号把改动放进暂存区。
                  </div>
                </div>
              )}
            </div>
            )}
          </>
        )}

        {tab === "files" && (
          <>
            {repoFiles.length === 0 ? (
              <div className="empty">
                <div className="empty-title">正在读取文件列表…</div>
                <div className="empty-text">
                  数量：{repoFiles.length}（若一直为 0，说明没取到数据）
                </div>
              </div>
            ) : (
              <RepoFileTree
                files={repoFiles}
                selectedPath={selectedRepoFile}
                onSelectFile={onSelectRepoFile}
              />
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
              <div style={{ padding: 8 }}>
                <PillButton
                  size="sm"
                  variant="success"
                  icon={<IconPlus />}
                  disabled={busy !== null}
                  style={{ width: "100%" }}
                  onClick={() => {
                    setNewBranchStart("");
                    setNewBranchOpen(true);
                  }}
                >
                  新建分支
                </PillButton>
              </div>
            )}

            <div className="list">
            {snapshot.branches.map((b) => (
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
              />
            ))}
            {snapshot.branches.length === 0 && (
              <div className="empty">
                <div className="empty-title">没有分支</div>
                <span className="empty-text">
                  <IconBranch /> 这个仓库还没有任何本地分支。
                </span>
              </div>
            )}
            </div>
          </>
        )}
      </div>
    </section>
  );
}
