import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { sfx } from "../sound";
import { IconInfo } from "./icons";

// 自绘标题栏。
//
// 窗口设了 Frameless（系统标题栏被去掉），所以这里要自己提供：
//   · 应用标识（图标 + 名字 + 当前仓库）
//   · 三个窗口按钮（最小化 / 最大化 / 关闭）
//   · 拖动区域（用 CSS 的 --wails-draggable: drag 交给 Wails 处理）
//
// 按钮上必须设 --wails-draggable: no-drag，否则点按钮会变成拖窗口。

interface Props {
  repoName?: string;
  branch?: string;
  /** 当前仓库的绝对路径，作为仓库名的悬停提示 */
  repoPath?: string;
  /** 最近打开过多少个仓库。没有打开仓库时，用它决定要不要显示「最近项目」入口 */
  recentCount?: number;
  /** 「切换项目」下拉是否展开（用来把按钮画成展开态、箭头翻转） */
  switcherOpen?: boolean;
  /** 点仓库名：把触发元素和它的屏幕位置交给上层去决定开还是收 */
  onToggleSwitcher?: (anchor: HTMLElement, x: number, y: number) => void;
  /** 点「关于」 */
  onAbout?: () => void;
}

export function TitleBar({
  repoName,
  branch,
  repoPath,
  recentCount = 0,
  switcherOpen,
  onToggleSwitcher,
  onAbout,
}: Props) {
  const [maximised, setMaximised] = useState(false);
  const repoBtnRef = useRef<HTMLButtonElement>(null);

  // 下拉触发器的文字：
  //   打开着仓库 -> 仓库名（点了是「切换项目」）
  //   没打开仓库但有过记录 -> 「最近项目」，这样一进软件就能直接开上次的项目
  //   都没有 -> 不显示
  const switcherLabel = repoName ?? (recentCount > 0 ? "最近项目" : null);
  // 没有仓库名时它不是一条路径，别用等宽字体渲染
  const isRecentEntry = !repoName && switcherLabel !== null;

  // 窗口最大化状态会变（用户双击标题栏、拖到屏幕边缘触发贴边等），
  // 所以定时同步一下，用它切换「最大化 / 还原」按钮的图标。
  useEffect(() => {
    let alive = true;
    const sync = async () => {
      try {
        const v = await api.isWindowMaximised();
        if (alive) setMaximised(v);
      } catch {
        /* 忽略 */
      }
    };
    void sync();
    const t = setInterval(sync, 800);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, []);

  const minimise = () => {
    sfx.click();
    void api.minimiseWindow();
  };
  const toggle = () => {
    sfx.click();
    void api.toggleMaximiseWindow();
  };
  const close = () => {
    sfx.close();
    void api.closeWindow();
  };

  return (
    <div className="titlebar" onDoubleClick={toggle}>
      <img className="titlebar-logo" src="/bingit-icon.svg" alt="" />

      <span className="titlebar-name">Bingit</span>

      {switcherLabel && (
        <>
          <span className="titlebar-sep">·</span>
          <button
            ref={repoBtnRef}
            type="button"
            className={
              "titlebar-repo" +
              (isRecentEntry ? " recent" : "") +
              (switcherOpen ? " open" : "")
            }
            title={
              repoPath ??
              (repoName ? repoName : "最近打开过的项目，点开可以切换")
            }
            onClick={() => {
              const el = repoBtnRef.current;
              if (!el || !onToggleSwitcher) return;
              sfx.click();
              const rect = el.getBoundingClientRect();
              onToggleSwitcher(el, rect.left, rect.bottom + 6);
            }}
            // 标题栏的双击是「最大化/还原」，双击这个按钮不应该触发它
            onDoubleClick={(e) => e.stopPropagation()}
          >
            <span className="titlebar-repo-name">{switcherLabel}</span>
            <svg
              className="titlebar-caret"
              viewBox="0 0 12 12"
              width="10"
              height="10"
            >
              <path
                d="m3 4.5 3 3 3-3"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </button>
        </>
      )}

      {branch && (
        <span className="titlebar-branch" title={branch}>
          {branch}
        </span>
      )}

      <span className="titlebar-spacer" />

      {onAbout && (
        <button
          type="button"
          className="titlebar-about"
          title="关于 Bingit"
          onClick={() => {
            sfx.click();
            onAbout();
          }}
          onDoubleClick={(e) => e.stopPropagation()}
        >
          <IconInfo />
        </button>
      )}

      <div className="titlebar-controls">
        <button className="winbtn" title="最小化" onClick={minimise}>
          <svg viewBox="0 0 12 12" width="12" height="12">
            <path d="M2 6h8" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
          </svg>
        </button>

        <button className="winbtn" title={maximised ? "还原" : "最大化"} onClick={toggle}>
          {maximised ? (
            <svg viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="1.3">
              <rect x="2.2" y="3.6" width="6.2" height="6.2" rx="1.2" />
              <path d="M4.2 3.6V2.4a1 1 0 0 1 1-1h4.2a1 1 0 0 1 1 1v4.2a1 1 0 0 1-1 1H8.4" />
            </svg>
          ) : (
            <svg viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="1.3">
              <rect x="2.4" y="2.4" width="7.2" height="7.2" rx="1.4" />
            </svg>
          )}
        </button>

        <button className="winbtn winbtn-close" title="关闭" onClick={close}>
          <svg viewBox="0 0 12 12" width="12" height="12" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round">
            <path d="M3 3l6 6M9 3l-6 6" />
          </svg>
        </button>
      </div>
    </div>
  );
}
