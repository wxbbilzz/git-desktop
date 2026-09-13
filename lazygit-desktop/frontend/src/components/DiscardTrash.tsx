import { useEffect, useState } from "react";
import type { DiscardRecord } from "../types";
import { api } from "../api";
import { PillButton } from "./PillButton";
import { IconRefresh, IconTrash, IconUndo } from "./icons";

/**
 * 「回收站」：本次运行里丢弃过的文件。
 *
 * 为什么需要它：丢弃改动是整个软件里唯一会真的让内容消失的操作 ——
 * 提交、合并、变基都能靠 reflog 救回来，但工作区里没提交过的改动一旦扔掉，
 * git 对象库里根本没有它。所以引擎在丢弃之前会把内容存成一个 git 对象，
 * 这里把它们列出来供恢复。
 *
 * 记录只存在内存里，重启后清空 —— 需要长期保存的内容应该用「储藏」。
 */
export function DiscardTrash({ onClose }: { onClose: () => void }) {
  const [records, setRecords] = useState<DiscardRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = async () => {
    setLoading(true);
    try {
      setRecords(await api.discardedFiles());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="ops-overlay" onClick={onClose}>
      <div className="ops-panel trash-panel" onClick={(e) => e.stopPropagation()}>
        <div className="ops-head">
          <span className="panel-title">
            <IconTrash /> 回收站
          </span>
          <span className="chip" style={{ height: 24 }}>
            {records.length} 个文件
          </span>
          <span className="spacer" />
          <PillButton
            size="sm"
            variant="ghost"
            icon={<IconRefresh />}
            onClick={() => void reload()}
          >
            刷新
          </PillButton>
          <PillButton size="sm" variant="ghost" onClick={onClose}>
            关闭
          </PillButton>
        </div>

        <div className="trash-body">
          <div className="hint">
            这里保存着被「丢弃改动」过的文件内容。记录只保留到本次运行结束，
            需要长期保存请用储藏。
          </div>

          {error && <div className="banner error">{error}</div>}

          <div className="list">
            {records.map((r) => (
              <div className="row" key={r.id}>
                <span className="dot deleted" />
                <div className="row-main">
                  <div className="row-name" title={r.path}>
                    {r.path}
                  </div>
                  <div className="row-sub">
                    {r.when} · {formatSize(r.size)}
                    {r.truncated ? " · 文件过大，未备份内容" : ""}
                  </div>
                </div>
                <div className="row-actions">
                  <PillButton
                    size="sm"
                    variant="primary"
                    icon={<IconUndo />}
                    disabled={r.truncated}
                    title={r.truncated ? "这个文件太大，没有备份内容" : "恢复到原路径"}
                    onClick={() => {
                      void api
                        .restoreDiscarded(r.id)
                        .then(() => reload())
                        .catch((e) =>
                          setError(e instanceof Error ? e.message : String(e)),
                        );
                    }}
                  >
                    恢复
                  </PillButton>
                </div>
              </div>
            ))}

            {!loading && records.length === 0 && (
              <div className="empty">
                <div className="empty-title">回收站是空的</div>
                <div className="empty-text">
                  丢弃过文件改动之后，内容会出现在这里。
                </div>
              </div>
            )}
            {loading && <div className="hint">正在读取…</div>}
          </div>
        </div>
      </div>
    </div>
  );
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
