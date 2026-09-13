import { useEffect } from "react";
import type { ReactNode } from "react";
import { PillButton } from "./PillButton";
import { IconAlert, IconCheck, IconTrash } from "./icons";

/**
 * 自绘的确认对话框，替代 window.confirm。
 *
 * 换掉系统弹窗有两个理由：
 *   - 原生弹窗和整套圆角面板 / 胶囊按钮的设计语言完全不搭，一弹出来就出戏
 *   - 更重要的是能顺便把「即将执行的 git 命令」显示出来 ——
 *     用户点「硬回退」之前，应该看得见它到底要跑什么
 */
export interface ConfirmSpec {
  title: string;
  /** 一句话说清后果，用完整的句子 */
  body: string;
  /** 即将执行的 git 命令（可选）。显示出来让用户有底。 */
  command?: string;
  /** 额外补充，比如「改动会保留在暂存区」 */
  note?: string;
  confirmLabel?: string;
  /** true 时按钮用危险样式（红色） */
  danger?: boolean;
  onConfirm: () => void;
}

interface Props extends ConfirmSpec {
  onCancel: () => void;
}

export function ConfirmDialog({
  title,
  body,
  command,
  note,
  confirmLabel,
  danger,
  onConfirm,
  onCancel,
}: Props) {
  // Esc 取消、Enter 确认 —— 键盘用户不用去够鼠标
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onCancel();
      } else if (e.key === "Enter") {
        e.preventDefault();
        onConfirm();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [onConfirm, onCancel]);

  return (
    <div className="ops-overlay" onClick={onCancel}>
      <div
        className="ops-panel confirm-panel"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="ops-head">
          <span className="panel-title">
            {danger ? <IconAlert /> : <IconCheck />} {title}
          </span>
          <span className="spacer" />
          <PillButton size="sm" variant="ghost" onClick={onCancel}>
            取消
          </PillButton>
        </div>

        <div className="confirm-body">
          <div className="confirm-text">{body as ReactNode}</div>

          {command && (
            <div className="confirm-command">
              <span className="confirm-command-label">将执行</span>
              <code>{command}</code>
            </div>
          )}

          {note && <div className="confirm-note">{note}</div>}

          <div className="form-actions">
            <PillButton size="sm" variant="ghost" onClick={onCancel}>
              取消（Esc）
            </PillButton>
            <PillButton
              size="sm"
              variant={danger ? "danger" : "primary"}
              icon={danger ? <IconTrash /> : <IconCheck />}
              onClick={onConfirm}
            >
              {confirmLabel || "确定"}
            </PillButton>
          </div>
        </div>
      </div>
    </div>
  );
}
