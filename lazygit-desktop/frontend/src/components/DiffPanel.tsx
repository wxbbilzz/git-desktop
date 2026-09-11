import { useMemo } from "react";
import type { CommitFileDTO, FilePatch, PatchLineDTO } from "../types";
import { PillButton } from "./PillButton";
import { IconCheck, IconCommit, IconMinus, IconPlus, IconTrash } from "./icons";

// 统一 diff 的逐行解析（提交模式仍用原始文本）。
type Kind = "meta" | "file" | "hunk" | "add" | "del" | "ctx";
interface DiffLine {
  kind: Kind;
  text: string;
  oldNo: number | null;
  newNo: number | null;
}

const META_PREFIXES = [
  "diff ", "index ", "new file", "deleted file", "similarity ",
  "dissimilarity ", "rename ", "copy ", "old mode", "new mode", "\\ No newline",
];

function parseDiff(raw: string): DiffLine[] {
  const out: DiffLine[] = [];
  let oldNo = 0;
  let newNo = 0;
  for (const line of raw.split("\n")) {
    if (line.startsWith("@@")) {
      const m = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(line);
      if (m) {
        oldNo = parseInt(m[1], 10);
        newNo = parseInt(m[2], 10);
      }
      out.push({ kind: "hunk", text: line, oldNo: null, newNo: null });
      continue;
    }
    if (line.startsWith("+++") || line.startsWith("---")) {
      out.push({ kind: "file", text: line, oldNo: null, newNo: null });
      continue;
    }
    if (META_PREFIXES.some((p) => line.startsWith(p))) {
      out.push({ kind: "meta", text: line, oldNo: null, newNo: null });
      continue;
    }
    if (line.startsWith("+")) {
      out.push({ kind: "add", text: line.slice(1), oldNo: null, newNo: newNo++ });
      continue;
    }
    if (line.startsWith("-")) {
      out.push({ kind: "del", text: line.slice(1), oldNo: oldNo++, newNo: null });
      continue;
    }
    if (line.startsWith(" ")) {
      out.push({ kind: "ctx", text: line.slice(1), oldNo: oldNo++, newNo: newNo++ });
      continue;
    }
    if (line === "") {
      out.push({ kind: "ctx", text: "", oldNo: null, newNo: null });
      continue;
    }
    out.push({ kind: "ctx", text: line, oldNo: oldNo++, newNo: newNo++ });
  }
  return out;
}

function basename(path: string): string {
  const i = path.lastIndexOf("/");
  return i === -1 ? path : path.slice(i + 1);
}
function dirname(path: string): string {
  const i = path.lastIndexOf("/");
  return i === -1 ? "" : path.slice(0, i);
}

interface Props {
  mode: "file" | "commit";
  path: string | null;
  hash?: string | null;
  staged: boolean;
  diff: string;
  loading: boolean;
  busy: string | null;
  commitFiles?: CommitFileDTO[];
  activeCommitFile?: string | null;
  onSelectCommitFile?: (path: string) => void;

  // 行级暂存
  filePatch?: FilePatch | null;
  selectedLines?: Set<number>;
  onToggleLine?: (index: number) => void;
  onClearLines?: () => void;
  onSelectHunk?: (indices: number[]) => void;
  onStageLines?: () => void;

  onStage?: () => void;
  onUnstage?: () => void;
  onDiscard?: () => void;
}

export function DiffPanel({
  mode, path, hash, staged, diff, loading, busy,
  commitFiles = [], activeCommitFile = null, onSelectCommitFile,
  filePatch = null, selectedLines, onToggleLine, onClearLines, onSelectHunk, onStageLines,
  onStage, onUnstage, onDiscard,
}: Props) {
  const lines = useMemo(() => parseDiff(diff), [diff]);
  const hasSelection = mode === "file" ? !!path : !!hash;
  const splitView = mode === "commit" && commitFiles.length > 0;

  const sel = selectedLines ?? new Set<number>();
  const selectedCount = sel.size;
  // 只有「工作区」模式下才允许勾选行；暂存区模式取消暂存整行容易混淆，
  // 而且当前需求是「挑一部分改动提交」，所以先只在工作区提供。
  const canPickLines = mode === "file" && !staged && !!filePatch?.hasChanges;

  return (
    <section className="panel">
      <div className="diff-head">
        {hasSelection ? (
          <>
            <span className="chip" style={{ height: 24 }}>
              {mode === "file" ? (staged ? "暂存区" : "工作区") : "提交"}
            </span>
            <span className="diff-path" title={mode === "file" ? (path ?? "") : (hash ?? "")}>
              {mode === "file" ? path : hash?.slice(0, 10)}
            </span>
            {splitView && (
              <span className="chip" style={{ height: 24 }}>{commitFiles.length} 个文件</span>
            )}

            {selectedCount > 0 && (
              <span className="chip accent" style={{ height: 24 }}>
                已选 {selectedCount} 行
              </span>
            )}

            <span className="spacer" />

            {selectedCount > 0 && (
              <>
                <PillButton size="sm" variant="ghost" onClick={onClearLines} disabled={busy !== null}>
                  清空选择
                </PillButton>
                <PillButton
                  size="sm"
                  variant="success"
                  icon={<IconCheck />}
                  disabled={busy !== null}
                  onClick={onStageLines}
                >
                  暂存选中行
                </PillButton>
              </>
            )}

            {mode === "file" && (
              <>
                {staged ? (
                  <PillButton size="sm" icon={<IconMinus />} disabled={busy !== null} onClick={onUnstage}>
                    取消暂存
                  </PillButton>
                ) : (
                  <PillButton size="sm" variant="success" icon={<IconPlus />} disabled={busy !== null} onClick={onStage}>
                    暂存整个文件
                  </PillButton>
                )}
                {!staged && (
                  <PillButton size="sm" variant="danger" icon={<IconTrash />} disabled={busy !== null} onClick={onDiscard}>
                    丢弃
                  </PillButton>
                )}
              </>
            )}
          </>
        ) : (
          <span className="panel-title">Diff</span>
        )}
      </div>

      {!hasSelection && (
        <div className="empty">
          <IconCommit />
          <div className="empty-title">选一个文件或提交</div>
          <div className="empty-text">
            在左侧点选文件查看改动。想只提交部分内容？在工作区点选具体的行，再点「暂存选中行」。
          </div>
        </div>
      )}

      {hasSelection && loading && (
        <div className="empty">
          <span className="spinner" />
          <div className="empty-text">正在读取…</div>
        </div>
      )}

      {hasSelection && !loading && (
        <div className={"diff-body" + (splitView ? " split" : "")}>
          {splitView && (
            <div className="diff-filelist">
              {commitFiles.map((f) => (
                <button
                  key={f.path}
                  className={"diff-file" + (activeCommitFile === f.path ? " active" : "")}
                  onClick={() => onSelectCommitFile?.(f.path)}
                  title={f.path}
                >
                  <span className={"dot " + f.kind} />
                  <span className="diff-file-main">
                    <span className="diff-file-name">{basename(f.path)}</span>
                    {dirname(f.path) && <span className="diff-file-dir">{dirname(f.path)}</span>}
                  </span>
                  {(f.additions > 0 || f.deletions > 0) && (
                    <span className="stat">
                      <span className="add">+{f.additions}</span>{" "}
                      <span className="del">-{f.deletions}</span>
                    </span>
                  )}
                  <span className={"badge " + f.kind}>{f.statusLabel}</span>
                </button>
              ))}
            </div>
          )}

          {/* 文件模式且拿得到结构化 patch：渲染成可勾选的行 */}
          {canPickLines && filePatch ? (
            <div className="diff">
              {filePatch.lines.map((l) => (
                <PatchRow
                  key={l.index}
                  line={l}
                  selected={sel.has(l.index)}
                  onToggle={onToggleLine}
                  onSelectHunk={onSelectHunk}
                  allLines={filePatch.lines}
                />
              ))}
            </div>
          ) : (
            <div className="diff">
              {lines.length === 0 ? (
                <div className="empty">
                  <div className="empty-title">
                    {splitView && !activeCommitFile ? "选一个文件" : "没有差异内容"}
                  </div>
                  <div className="empty-text">
                    {splitView && !activeCommitFile
                      ? "在左边点选一个文件，这里会显示它在那次提交里的改动。"
                      : "这个文件可能只有权限或文件名变化，git 没有产生文本 diff。"}
                  </div>
                </div>
              ) : (
                lines.map((l, i) => (
                  <div key={i} className={"diff-line " + l.kind}>
                    <span className="diff-no">{l.oldNo ?? ""}</span>
                    <span className="diff-no">{l.newNo ?? ""}</span>
                    <span className="diff-text">{l.text || " "}</span>
                  </div>
                ))
              )}
            </div>
          )}
        </div>
      )}
    </section>
  );
}

/** 一行结构化 patch：可勾选的行带复选框，hunk 头带「全选本块」。 */
function PatchRow({
  line, selected, onToggle, onSelectHunk, allLines,
}: {
  line: PatchLineDTO;
  selected: boolean;
  onToggle?: (i: number) => void;
  onSelectHunk?: (indices: number[]) => void;
  allLines: PatchLineDTO[];
}) {
  if (line.kind === "hunk") {
    // 找出这个 hunk 覆盖的可选行，方便「全选本块」
    const start = allLines.findIndex((l) => l.index === line.index);
    const indices: number[] = [];
    for (let i = start + 1; i < allLines.length; i++) {
      if (allLines[i].kind === "hunk" || allLines[i].kind === "header") break;
      if (allLines[i].selectable) indices.push(allLines[i].index);
    }
    return (
      <div className="diff-line hunk patch-hunk">
        <span className="diff-text">{line.text}</span>
        {indices.length > 0 && (
          <button className="hunk-select" onClick={() => onSelectHunk?.(indices)}>
            全选本块
          </button>
        )}
      </div>
    );
  }

  const cls =
    line.kind === "addition" ? "add" :
    line.kind === "deletion" ? "del" :
    line.kind === "header" ? "meta" : "ctx";

  if (!line.selectable) {
    return (
      <div className={"diff-line " + cls}>
        <span className="diff-no">{line.oldNo || ""}</span>
        <span className="diff-no">{line.newNo || ""}</span>
        <span className="diff-text">{line.text || " "}</span>
      </div>
    );
  }

  return (
    <div
      className={"diff-line " + cls + " pickable" + (selected ? " picked" : "")}
      onClick={() => onToggle?.(line.index)}
    >
      <span className="diff-check">{selected ? "✓" : ""}</span>
      <span className="diff-no">{line.oldNo || ""}</span>
      <span className="diff-no">{line.newNo || ""}</span>
      <span className="diff-text">{line.text || " "}</span>
    </div>
  );
}
