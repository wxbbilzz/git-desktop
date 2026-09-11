import { useEffect, useState } from "react";
import { api } from "../api";
import { sfx } from "../sound";

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
}

export function TitleBar({ repoName, branch }: Props) {
  const [maximised, setMaximised] = useState(false);

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

      {repoName && (
        <>
          <span className="titlebar-sep">·</span>
          <span className="titlebar-repo" title={repoName}>
            {repoName}
          </span>
        </>
      )}

      {branch && (
        <span className="titlebar-branch" title={branch}>
          {branch}
        </span>
      )}

      <span className="titlebar-spacer" />

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
