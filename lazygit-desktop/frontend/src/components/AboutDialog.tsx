import { useEffect } from "react";
import { PillButton } from "./PillButton";
import { IconInfo } from "./icons";
import { APP_VERSION } from "../version";

// 「关于」对话框。
//
// 和 ConfirmDialog / DiscardTrash 共用 .ops-overlay + .ops-panel 这套壳，
// 所以圆角、阴影、遮罩点击关闭的行为都和别处一致。

interface Props {
  onClose: () => void;
  /** 复制诊断信息。复用 App 里那套「复制成功」提示 */
  onCopy: (text: string, label: string) => void;
  /** 当前打开的项目路径。没打开仓库时不显示这一行 */
  repoPath?: string;
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div className="about-row">
      <span className="about-key">{k}</span>
      <span className="about-val">{v}</span>
    </div>
  );
}

export function AboutDialog({ onClose, onCopy, repoPath }: Props) {
  // Esc 关闭。用捕获阶段，免得被别的 keydown 抢先
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [onClose]);

  // 反馈问题时把这几行一起发过去，比只说「版本 2.0.0.0」有用得多
  const diagnostics = [
    `Bingit ${APP_VERSION}`,
    `平台 ${navigator.platform}`,
    repoPath ? `项目 ${repoPath}` : "项目 （未打开）",
  ].join("\n");

  return (
    <div className="ops-overlay" onClick={onClose}>
      <div className="ops-panel about-panel" onClick={(e) => e.stopPropagation()}>
        <div className="ops-head">
          <span className="panel-title">
            <IconInfo /> 关于
          </span>
          <span className="spacer" />
          <PillButton size="sm" variant="ghost" onClick={onClose}>
            关闭（Esc）
          </PillButton>
        </div>

        <div className="about-body">
          <img className="about-logo" src="/bingit-icon.svg" alt="" />
          <div className="about-name">Bingit</div>
          <div className="about-version">版本 {APP_VERSION}</div>

          <p className="about-desc">
            冰冰的 git 客户端 —— 复用 lazygit 的核心逻辑、图形界面完全重写的桌面版 Git
            工具。文件树、行级暂存、提交图，以及 110 个 git 操作的图形面板都在这里。
          </p>

          <div className="about-rows">
            <Row k="开发者" v="一只小冰冰" />
            <Row k="技术栈" v="Go · Wails v2 · React · TypeScript" />
            <Row k="Git 命令层" v="lazygit（MIT 协议）" />
            <Row k="界面框架" v="Wails（MIT 协议）" />
            <Row k="开源许可" v="本仓库暂未声明许可协议" />
          </div>

          <div className="about-foot">
            <span className="hint">遇到问题时把版本信息一起发过来</span>
            <PillButton
              size="sm"
              variant="ghost"
              onClick={() => onCopy(diagnostics, "版本信息")}
            >
              复制版本信息
            </PillButton>
          </div>
        </div>
      </div>
    </div>
  );
}
