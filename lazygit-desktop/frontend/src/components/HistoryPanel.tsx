import { useMemo, useState } from "react";
import type { CommitDTO } from "../types";
import { computeGraph } from "../commitGraph";
import { CommitGraph } from "./CommitGraph";
import { FilterBox } from "./SidebarPanels";
import type { MenuSpec } from "./ContextMenu";
import {
  IconCherry,
  IconCopy,
  IconPlus,
  IconRewind,
  IconTag,
  IconUndo,
} from "./icons";

interface Props {
  commits: CommitDTO[];
  selectedHash: string | null;
  onSelect: (hash: string) => void;
  busy: string | null;
  /** 把「这次提交」作为参数的动作。以前这些只能去命令面板里手填哈希 */
  onCherryPick: (hash: string) => void;
  onRevert: (hash: string) => void;
  onResetTo: (hash: string, mode: "soft" | "mixed" | "hard") => void;
  onTagCommit: (hash: string) => void;
  onCreateBranchFrom: (hash: string) => void;
  onMenu: (spec: MenuSpec) => void;
  onCopy: (text: string, label: string) => void;
}

export function HistoryPanel({
  commits,
  selectedHash,
  onSelect,
  busy,
  onCherryPick,
  onRevert,
  onResetTo,
  onTagCommit,
  onCreateBranchFrom,
  onMenu,
  onCopy,
}: Props) {
  const [filter, setFilter] = useState("");

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return commits;
    return commits.filter(
      (c) =>
        c.subject.toLowerCase().includes(q) ||
        c.author.toLowerCase().includes(q) ||
        c.shortHash.toLowerCase().includes(q) ||
        c.tags.some((t) => t.toLowerCase().includes(q)),
    );
  }, [commits, filter]);

  // 泳道图按过滤后的列表算：图只反映当前显示出来的这些提交
  const rows = useMemo(() => computeGraph(filtered), [filtered]);

  const openMenu = (e: React.MouseEvent, c: CommitDTO) => {
    e.preventDefault();
    e.stopPropagation();
    const disabled = busy !== null;
    onMenu({
      x: e.clientX,
      y: e.clientY,
      items: [
        {
          label: "拣选到当前分支",
          icon: <IconCherry />,
          disabled,
          onClick: () => onCherryPick(c.hash),
        },
        {
          label: "生成反向提交（撤销这次改动）",
          icon: <IconUndo />,
          disabled,
          onClick: () => onRevert(c.hash),
        },
        {
          label: "以此为起点新建分支",
          icon: <IconPlus />,
          disabled,
          onClick: () => onCreateBranchFrom(c.hash),
        },
        {
          label: "给这次提交打标签",
          icon: <IconTag />,
          disabled,
          onClick: () => onTagCommit(c.hash),
        },
        { label: "", separator: true },
        {
          label: "回退到这里（改动留在暂存区）",
          icon: <IconRewind />,
          disabled,
          onClick: () => onResetTo(c.hash, "soft"),
        },
        {
          label: "回退到这里（改动留在工作区）",
          icon: <IconRewind />,
          disabled,
          onClick: () => onResetTo(c.hash, "mixed"),
        },
        {
          label: "回退到这里（丢弃所有改动）",
          icon: <IconRewind />,
          danger: true,
          disabled,
          onClick: () => onResetTo(c.hash, "hard"),
        },
        { label: "", separator: true },
        {
          label: "复制提交号",
          icon: <IconCopy />,
          onClick: () => onCopy(c.hash, "提交号"),
        },
        {
          label: "复制短提交号",
          icon: <IconCopy />,
          onClick: () => onCopy(c.shortHash, "短提交号"),
        },
        {
          label: "复制提交说明",
          icon: <IconCopy />,
          onClick: () => onCopy(c.subject, "提交说明"),
        },
      ],
    });
  };

  return (
    <section className="panel">
      <div className="panel-header">
        <span className="panel-title">历史</span>
        <span className="chip" style={{ height: 24 }}>
          {filter ? `${filtered.length}/${commits.length}` : commits.length} 条
        </span>
      </div>

      <div className="panel-body">
        {commits.length > 8 && (
          <FilterBox
            value={filter}
            onChange={setFilter}
            placeholder="搜索提交信息 / 作者 / 提交号…"
          />
        )}

        <div className="commit-list">
          {filtered.map((c, i) => (
            <div
              key={c.hash}
              className={"commit" + (selectedHash === c.hash ? " selected" : "")}
              onClick={() => onSelect(c.hash)}
              onContextMenu={(e) => openMenu(e, c)}
            >
              {rows[i] && <CommitGraph row={rows[i]} />}
              <div className="commit-main">
                <div className="commit-subject" title={c.subject}>
                  {c.subject}
                </div>

                <div className="commit-meta">
                  <span className="commit-hash">{c.shortHash}</span>
                  <span>{c.author}</span>
                  <span>·</span>
                  <span>{c.when}</span>
                  {c.tags.length > 0 && (
                    <span className="ref-pill" title={c.tags.join(", ")}>
                      <IconTag /> {c.tags.join(", ")}
                    </span>
                  )}
                  {c.extraInfo && (
                    <span className="ref-pill" title={c.extraInfo}>
                      {c.extraInfo}
                    </span>
                  )}
                </div>
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
          {commits.length > 0 && filtered.length === 0 && (
            <div className="empty">
              <div className="empty-title">没有匹配的提交</div>
              <div className="empty-text">换个关键词试试。</div>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
