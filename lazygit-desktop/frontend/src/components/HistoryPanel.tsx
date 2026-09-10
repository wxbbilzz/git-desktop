import type { CommitDTO } from "../types";

interface Props {
  commits: CommitDTO[];
  selectedHash: string | null;
  onSelect: (hash: string) => void;
}

export function HistoryPanel({ commits, selectedHash, onSelect }: Props) {
  return (
    <section className="panel">
      <div className="panel-header">
        <span className="panel-title">历史</span>
        <span className="chip" style={{ height: 24 }}>
          {commits.length} 条
        </span>
      </div>

      <div className="panel-body">
        <div className="commit-list">
          {commits.map((c) => (
            <div
              key={c.hash}
              className={"commit" + (selectedHash === c.hash ? " selected" : "")}
              onClick={() => onSelect(c.hash)}
            >
              <div className="commit-subject" title={c.subject}>
                {c.subject}
              </div>

              <div className="commit-meta">
                <span className="commit-hash">{c.shortHash}</span>
                <span>{c.author}</span>
                <span>·</span>
                <span>{c.when}</span>
                {c.extraInfo && (
                  <span className="ref-pill" title={c.extraInfo}>
                    {c.extraInfo}
                  </span>
                )}
              </div>
            </div>
          ))}

          {commits.length === 0 && (
            <div className="empty">
              <div className="empty-title">还没有历史</div>
              <div className="empty-text">
                这个仓库还没有任何提交，完成第一次提交后就会出现在这里。
              </div>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
