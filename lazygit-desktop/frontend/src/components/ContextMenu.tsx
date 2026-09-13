import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { ReactNode } from "react";

/** 菜单里的一项。separator 为真时这一项渲染成分隔线。 */
export interface MenuItem {
  label: string;
  icon?: ReactNode;
  /** 显示成危险色（删除、硬回退这类） */
  danger?: boolean;
  disabled?: boolean;
  /** 右侧的快捷键提示，例如 ⌘C */
  hint?: string;
  separator?: boolean;
  onClick?: () => void;
}

/** 右键菜单的位置与内容。 */
export interface MenuSpec {
  x: number;
  y: number;
  items: MenuItem[];
}

interface Props {
  spec: MenuSpec;
  onClose: () => void;
}

/**
 * 列表行的右键菜单。
 *
 * 为什么需要它：分支、提交、文件这些列表里，每一行都能做七八件事
 * （合并、变基、拣选、回退、复制哈希…）。全做成行内按钮会让列表彻底糊掉，
 * 所以把「能做的事」都收进右键菜单，行内只留最高频的一两个。
 */
export function ContextMenu({ spec, onClose }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ x: spec.x, y: spec.y });

  // 贴边时把菜单翻到另一边，别让它跑到窗口外面去
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    let { x, y } = spec;
    if (x + rect.width > window.innerWidth - 8) {
      x = Math.max(8, window.innerWidth - rect.width - 8);
    }
    if (y + rect.height > window.innerHeight - 8) {
      y = Math.max(8, window.innerHeight - rect.height - 8);
    }
    setPos({ x, y });
  }, [spec]);

  // 点别处、按 Esc、滚动都关掉
  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("mousedown", onDown, true);
    window.addEventListener("keydown", onKey, true);
    window.addEventListener("wheel", onClose, true);
    window.addEventListener("resize", onClose);
    return () => {
      window.removeEventListener("mousedown", onDown, true);
      window.removeEventListener("keydown", onKey, true);
      window.removeEventListener("wheel", onClose, true);
      window.removeEventListener("resize", onClose);
    };
  }, [onClose]);

  return (
    <div
      ref={ref}
      className="ctx-menu"
      style={{ left: pos.x, top: pos.y }}
      onContextMenu={(e) => e.preventDefault()}
    >
      {spec.items.map((it, i) => {
        if (it.separator) return <div key={i} className="ctx-sep" />;
        return (
          <button
            key={i}
            className={"ctx-item" + (it.danger ? " danger" : "")}
            disabled={it.disabled}
            onClick={() => {
              if (it.disabled) return;
              onClose();
              it.onClick?.();
            }}
          >
            {it.icon && <span className="ctx-icon">{it.icon}</span>}
            <span className="ctx-label">{it.label}</span>
            {it.hint && <span className="ctx-hint">{it.hint}</span>}
          </button>
        );
      })}
    </div>
  );
}
