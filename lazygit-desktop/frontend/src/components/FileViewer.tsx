import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import type { FileContentDTO } from "../types";
import { PillButton } from "./PillButton";
import { IconRefresh } from "./icons";

// 只读文件查看器。用于「文件」标签里浏览仓库内容。
//
// 带一个非常轻量的语法着色：只识别注释、字符串、数字和关键字，
// 用正则分行处理 —— 目标是「看得舒服」，不是做成完整编辑器。

const KEYWORDS =
  /\b(func|package|import|return|if|else|for|range|struct|interface|type|const|var|map|chan|go|defer|switch|case|break|continue|class|export|default|from|new|await|async|let|const|try|catch|throw|typeof|instanceof|def|self|None|True|False|public|private|void|int|string|bool)\b/;

interface Token {
  kind: "plain" | "comment" | "string" | "number" | "keyword";
  text: string;
}

function tokenize(line: string): Token[] {
  const out: Token[] = [];
  const trimmed = line.trimStart();

  // 整行注释
  if (
    trimmed.startsWith("//") ||
    trimmed.startsWith("#") ||
    trimmed.startsWith("--") ||
    trimmed.startsWith("/*") ||
    trimmed.startsWith("*")
  ) {
    return [{ kind: "comment", text: line }];
  }

  // 逐段匹配：字符串 / 数字 / 关键字 / 其他
  const re = /("[^"]*"|'[^']*'|`[^`]*`|\/\/.*$|#[^!].*$|\b\d+(\.\d+)?\b)|([A-Za-z_][A-Za-z0-9_]*)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(line)) !== null) {
    if (m.index > last) {
      out.push({ kind: "plain", text: line.slice(last, m.index) });
    }
    const text = m[0];
    let kind: Token["kind"] = "plain";
    if (/^["'`]/.test(text)) kind = "string";
    else if (/^(\/\/|#)/.test(text)) kind = "comment";
    else if (/^\d/.test(text)) kind = "number";
    else if (KEYWORDS.test(text)) kind = "keyword";
    out.push({ kind, text });
    last = m.index + text.length;
  }
  if (last < line.length) out.push({ kind: "plain", text: line.slice(last) });
  return out;
}

interface Props {
  path: string;
  onClose: () => void;
}

export function FileViewer({ path, onClose }: Props) {
  const [data, setData] = useState<FileContentDTO | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = () => {
    setLoading(true);
    setError(null);
    void api
      .fileContent(path)
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false));
  };

  useEffect(load, [path]);

  const lines = useMemo(() => data?.content.split("\n") ?? [], [data]);

  return (
    <section className="panel">
      <div className="diff-head">
        <span className="chip" style={{ height: 24 }}>
          文件
        </span>
        <span className="diff-path" title={path}>
          {path}
        </span>
        {data && (
          <span className="chip" style={{ height: 24 }}>
            {data.lines} 行 · {formatSize(data.size)}
          </span>
        )}
        <span className="spacer" />
        <PillButton size="sm" icon={<IconRefresh />} onClick={load} disabled={loading}>
          重新读取
        </PillButton>
        <PillButton size="sm" variant="ghost" onClick={onClose}>
          关闭
        </PillButton>
      </div>

      {loading && (
        <div className="empty">
          <span className="spinner" />
          <div className="empty-text">正在读取…</div>
        </div>
      )}

      {error && (
        <div className="empty">
          <div className="empty-title">读取失败</div>
          <div className="empty-text">{error}</div>
        </div>
      )}

      {!loading && !error && data?.binary && (
        <div className="empty">
          <div className="empty-title">二进制文件</div>
          <div className="empty-text">
            这个文件（{formatSize(data.size)}）无法以文本方式预览。
          </div>
        </div>
      )}

      {!loading && !error && data && !data.binary && (
        <div className="diff file-view">
          {data.truncated && (
            <div className="file-truncated">
              文件较大，只显示前 {Math.round(data.content.length / 1024)} KB
            </div>
          )}
          {lines.map((line, i) => (
            <div key={i} className="code-line">
              <span className="code-no">{i + 1}</span>
              <span className="code-text">
                {tokenize(line).map((t, j) => (
                  <span key={j} className={"tok-" + t.kind}>
                    {t.text}
                  </span>
                ))}
              </span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
