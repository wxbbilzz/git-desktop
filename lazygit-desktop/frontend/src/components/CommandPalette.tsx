import { useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";

/** 命令面板里的一条命令。 */
export interface Command {
  id: string;
  label: string;
  /** 分组名，用来在列表里分隔显示 */
  group: string;
  hint?: string;
  icon?: ReactNode;
  run: () => void;
}

interface Props {
  commands: Command[];
  onClose: () => void;
}

/**
 * 命令面板（Ctrl/⌘ + K）。
 *
 * 这个软件里的动作已经上百个了 —— 光「Git 操作」目录就有 150 多条命令，
 * 再加上分支、标签、远端各自的操作。全都做成按钮的话界面会变成仪表盘，
 * 于是提供这条「打字就能到」的路径：键盘流用户敲几个字回车即可，
 * 鼠标用户也能当搜索框用。
 */
export function CommandPalette({ commands, onClose }: Props) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  // 简单的加权匹配：完整包含 > 首字母缩写 > 顺序散落匹配
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands.slice(0, 40);

    const scored: { c: Command; score: number }[] = [];
    for (const c of commands) {
      const hay = (c.label + " " + c.group + " " + (c.hint || "")).toLowerCase();
      let score = -1;
      const idx = hay.indexOf(q);
      if (idx >= 0) {
        // 越靠前命中越相关；短标签命中更相关
        score = 1000 - idx * 5 - Math.min(hay.length, 100);
      } else if (isSubsequence(q, hay)) {
        score = 100;
      }
      if (score > 0) scored.push({ c, score });
    }
    scored.sort((a, b) => b.score - a.score);
    return scored.slice(0, 40).map((s) => s.c);
  }, [commands, query]);

  useEffect(() => setActive(0), [query]);

  // 键盘导航：上下选择、回车执行、Esc 关闭
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        setActive((i) => Math.min(i + 1, filtered.length - 1));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setActive((i) => Math.max(i - 1, 0));
      } else if (e.key === "Enter") {
        e.preventDefault();
        const pick = filtered[active];
        if (pick) {
          onClose();
          pick.run();
        }
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [filtered, active, onClose]);

  // 选中项滚进视野
  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(".palette-item.active");
    el?.scrollIntoView({ block: "nearest" });
  }, [active]);

  return (
    <div className="ops-overlay palette-overlay" onClick={onClose}>
      <div className="palette" onClick={(e) => e.stopPropagation()}>
        <input
          className="palette-input"
          placeholder="输入命令名…  例如：合并 / rebase / 标签 / 检出"
          value={query}
          autoFocus
          spellCheck={false}
          onChange={(e) => setQuery(e.target.value)}
        />

        <div className="palette-list" ref={listRef}>
          {filtered.map((c, i) => {
            const showGroup = i === 0 || filtered[i - 1].group !== c.group;
            return (
              <div key={c.id}>
                {showGroup && <div className="palette-group">{c.group}</div>}
                <button
                  className={"palette-item" + (i === active ? " active" : "")}
                  onMouseEnter={() => setActive(i)}
                  onClick={() => {
                    onClose();
                    c.run();
                  }}
                >
                  {c.icon && <span className="palette-icon">{c.icon}</span>}
                  <span className="palette-label">{c.label}</span>
                  {c.hint && <span className="palette-hint">{c.hint}</span>}
                </button>
              </div>
            );
          })}

          {filtered.length === 0 && (
            <div className="empty">
              <div className="empty-title">没有匹配的命令</div>
              <div className="empty-text">换个说法试试，比如「分支」「储藏」「远端」。</div>
            </div>
          )}
        </div>

        <div className="palette-foot">
          <span>↑↓ 选择</span>
          <span>Enter 执行</span>
          <span>Esc 关闭</span>
        </div>
      </div>
    </div>
  );
}

/** 判断 q 的字符是否按顺序散布在 hay 里（用于「cnf」匹配「checkout new feature」）。 */
function isSubsequence(q: string, hay: string): boolean {
  let i = 0;
  for (const ch of hay) {
    if (ch === q[i]) i++;
    if (i === q.length) return true;
  }
  return i === q.length;
}
