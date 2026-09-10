import { useMemo } from "react";
import type { CommitFileDTO } from "../types";
import { PillButton } from "./PillButton";
import { IconCommit, IconMinus, IconPlus, IconTrash } from "./icons";

// 统一 diff 的逐行解析。
//
// 引擎已经把 git 的 ANSI 颜色去掉了（plain=true），所以这里自己判断每行类型
// 并维护新旧行号。这是「重写 UI」里最需要自己实现的一块。

type Kind = "meta" | "file" | "hunk" | "add" | "del" | "ctx";

interface DiffLine {
  kind: Kind;
  text: string;
  oldNo: number | null;
  newNo: number | null;
}

const META_PREFIXES = [
  "diff ",
  "index ",
  "new file",
  "deleted file",
  "similarity ",
  "dissimilarity ",
  "rename ",
  "copy ",
  "old mode",
  "new mode",
  "\\ No newline",
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
  // 提交模式下涉及的文件列表，以及当前正在看的那个文件
  commitFiles?: CommitFileDTO[];
  activeCommitFile?: string | null;
  onSelectCommitFile?: (path: string) => void;
  onStage?: () => void;
  onUnstage?: () => void;
  onDiscard?: () => void;
}

export function DiffPanel({
  mode,
  path,
  hash,
  staged,
  diff,
  loading,
  busy,
  commitFiles = [],
  activeCommitFile = null,
  onSelectCommitFile,
  onStage,
  onUnstage,
  onDiscard,
}: Props) {
  const lines = useMemo(() => parseDiff(diff), [diff]);
  const hasSelection = mode === "file" ? !!path : !!hash;

  // 提交模式下有文件列表时，才是「左列表 + 右 diff」的双栏布局
  const splitView = mode === "commit" && commitFiles.length > 0;

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
              <span className="chip" style={{ height: 24 }}>
                {commitFiles.length} 个文件
              </span>
            )}
            <span className="spacer" />

            {mode === "file" && (
              <>
                {staged ? (
                  <PillButton size="sm" icon={<IconMinus />} disabled={busy !== null} onClick={onUnstage}>
                    取消暂存
                  </PillButton>
                ) : (
                  <PillButton
                    size="sm"
                    variant="success"
                    icon={<IconPlus />}
                    disabled={busy !== null}
                    onClick={onStage}
                  >
                    暂存
                  </PillButton>
                )}
                {!staged && (
                  <PillButton
                    size="sm"
                    variant="danger"
                    icon={<IconTrash />}
                    disabled={busy !== null}
                    onClick={onDiscard}
                  >
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
            在左侧点选文件查看改动，或在右下方点选提交，按文件查看那次提交改了什么。
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
          {/* 提交模式：左边是这次提交涉及的文件列表 */}
          {splitView && (
            <div className="diff-filelist">
              {commitFiles.map((f) => (
                <button
                  key={f.path}
                  className={
                    "diff-file" + (activeCommitFile === f.path ? " active" : "")
                  }
                  onClick={() => onSelectCommitFile?.(f.path)}
                  title={f.path}
                >
                  <span className={"dot " + f.kind} />
                  <span className="diff-file-main">
                    <span className="diff-file-name">{basename(f.path)}</span>
                    {dirname(f.path) && (
                      <span className="diff-file-dir">{dirname(f.path)}</span>
                    )}
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
        </div>
      )}
    </section>
  );
}
