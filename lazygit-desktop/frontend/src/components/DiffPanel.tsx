import { useMemo } from "react";
import { PillButton } from "./PillButton";
import { IconCommit, IconMinus, IconPlus, IconTrash } from "./icons";

// 统一 diff 的逐行解析。
//
// 引擎已经把 git 的 ANSI 颜色去掉了（plain=true），所以这里自己判断每行类型
// 并维护新旧行号 —— 这正好对应之前说的「diff 视图是重写 UI 的主要工作量」。
// 这里先做最实用的行级渲染，后续可以在此扩展行内高亮、折叠、双栏对比。

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
      out.push({
        kind: "ctx",
        text: line.slice(1),
        oldNo: oldNo++,
        newNo: newNo++,
      });
      continue;
    }

    // 空行、以及无法归类的行（例如 diff 末尾的空串）
    if (line === "") {
      out.push({ kind: "ctx", text: "", oldNo: null, newNo: null });
      continue;
    }
    out.push({ kind: "ctx", text: line, oldNo: oldNo++, newNo: newNo++ });
  }

  return out;
}

interface Props {
  // mode 决定这是「文件的改动」还是「某次提交的改动」
  mode: "file" | "commit";
  path: string | null;
  hash?: string | null;
  staged: boolean;
  diff: string;
  loading: boolean;
  busy: string | null;
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
  onStage,
  onUnstage,
  onDiscard,
}: Props) {
  const lines = useMemo(() => parseDiff(diff), [diff]);

  const hasSelection = mode === "file" ? !!path : !!hash;

  return (
    <section className="panel">
      <div className="diff-head">
        {hasSelection ? (
          <>
            <span className="chip" style={{ height: 24 }}>
              {mode === "file" ? (staged ? "暂存区" : "工作区") : "提交"}
            </span>
            <span className="diff-path" title={path ?? hash ?? ""}>
              {mode === "file" ? path : hash}
            </span>
            <span className="spacer" />

            {mode === "file" && (
              <>
                {staged ? (
                  <PillButton
                    size="sm"
                    icon={<IconMinus />}
                    disabled={busy !== null}
                    onClick={onUnstage}
                  >
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
            在左侧点选文件查看改动，或在右下方点选提交查看它的 diff。
          </div>
        </div>
      )}

      {hasSelection && loading && (
        <div className="empty">
          <span className="spinner" />
          <div className="empty-text">正在读取 diff…</div>
        </div>
      )}

      {hasSelection && !loading && lines.length === 0 && (
        <div className="empty">
          <div className="empty-title">没有差异内容</div>
          <div className="empty-text">
            这个文件可能只有权限或文件名变化，git 没有产生文本 diff。
          </div>
        </div>
      )}

      {hasSelection && !loading && lines.length > 0 && (
        <div className="diff">
          {lines.map((l, i) => (
            <div key={i} className={"diff-line " + l.kind}>
              <span className="diff-no">{l.oldNo ?? ""}</span>
              <span className="diff-no">{l.newNo ?? ""}</span>
              <span className="diff-text">{l.text || " "}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
