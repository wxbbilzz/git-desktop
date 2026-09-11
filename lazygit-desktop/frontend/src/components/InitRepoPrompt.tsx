import { useState } from "react";
import type { FolderInfo } from "../types";
import { PillButton } from "./PillButton";
import { IconAlert, IconRepo } from "./icons";

// 打开一个「不受 git 管理」的文件夹时的询问框。
//
// 用户拖进来（或在选择框里选中）一个普通文件夹时，不该直接报错，
// 而是问一句：要不要在这里建一个 git 仓库？

interface Props {
  info: FolderInfo;
  busy: boolean;
  onConfirm: (initialBranch: string) => void;
  onCancel: () => void;
}

export function InitRepoPrompt({ info, busy, onConfirm, onCancel }: Props) {
  const [branch, setBranch] = useState("main");
  const name = info.path.split("/").filter(Boolean).pop() ?? info.path;

  return (
    <div className="ops-overlay" onClick={() => !busy && onCancel()}>
      <div className="ops-panel init-panel" onClick={(e) => e.stopPropagation()}>
        <div className="ops-head">
          <span className="panel-title">这个文件夹还没有纳入 git 管理</span>
        </div>

        <div className="publish-body">
          <div className="init-path">
            <IconRepo />
            <span className="init-path-text" title={info.path}>
              {info.path}
            </span>
          </div>

          <div className="hint-row">
            要在这个文件夹里创建一个 git 仓库吗？创建后就能对它做版本控制了。
            {info.fileCount > 0 && (
              <>
                <br />
                <span style={{ color: "var(--yellow)" }}>
                  <IconAlert /> 目录里已经有 {info.fileCount} 个文件/子目录，它们不会被删除，
                  只是开始被 git 记录。
                </span>
              </>
            )}
          </div>

          <label className="label">初始分支名</label>
          <input
            className="field mono"
            value={branch}
            disabled={busy}
            spellCheck={false}
            onChange={(e) => setBranch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") onConfirm(branch.trim());
            }}
          />

          <div className="hint-row">
            相当于在这个目录执行 <code>git init</code>。不会提交任何东西，
            你可以先在「工作区」里挑要提交的文件。
          </div>

          <div className="form-actions">
            <PillButton variant="ghost" disabled={busy} onClick={onCancel}>
              取消
            </PillButton>
            <PillButton
              variant="primary"
              icon={<IconRepo />}
              disabled={busy || !branch.trim()}
              onClick={() => onConfirm(branch.trim())}
            >
              {busy ? "正在创建…" : `在「${name}」建立仓库`}
            </PillButton>
          </div>
        </div>
      </div>
    </div>
  );
}
