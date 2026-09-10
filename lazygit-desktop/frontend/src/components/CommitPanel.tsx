import { useState } from "react";
import type { KeyboardEvent } from "react";
import { PillButton } from "./PillButton";
import { IconCheck, IconCommit } from "./icons";

interface Props {
  summary: string;
  description: string;
  stagedCount: number;
  busy: string | null;
  // 当前生效的提交身份（来自 git config）
  identityName: string;
  identityEmail: string;
  onSummaryChange: (v: string) => void;
  onDescriptionChange: (v: string) => void;
  onCommit: () => void;
  onSaveIdentity: (name: string, email: string) => void;
}

export function CommitPanel({
  summary,
  description,
  stagedCount,
  busy,
  identityName,
  identityEmail,
  onSummaryChange,
  onDescriptionChange,
  onCommit,
  onSaveIdentity,
}: Props) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");

  // git 提交必须知道「你是谁」。没配置的话直接在面板里引导填，
  // 而不是等用户点了提交再丢一句 fatal 报错。
  const identityMissing = !identityName || !identityEmail;

  const canCommit =
    !identityMissing && summary.trim().length > 0 && stagedCount > 0 && busy === null;
  const canSave = name.trim().length > 0 && email.includes("@") && busy === null;

  const handleKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter" && canCommit) {
      e.preventDefault();
      onCommit();
    }
  };

  if (identityMissing) {
    return (
      <section className="panel">
        <div className="panel-header">
          <span className="panel-title">提交</span>
          <span className="chip warn" style={{ height: 24 }}>
            需要先配置身份
          </span>
        </div>

        <div className="form">
          <div className="identity-note">
            git 在提交时会记录「是谁提交的」，但你的
            <code>user.name</code> / <code>user.email</code> 还没有配置，
            所以现在无法提交。
            <br />
            填一次即可（写入全局配置，之后所有仓库都不用再填）。
          </div>

          <label className="label">名字</label>
          <input
            className="field mono"
            placeholder="张三"
            value={name}
            disabled={busy !== null}
            spellCheck={false}
            autoFocus
            onChange={(e) => setName(e.target.value)}
          />

          <label className="label">邮箱</label>
          <input
            className="field mono"
            placeholder="zhangsan@example.com"
            value={email}
            disabled={busy !== null}
            spellCheck={false}
            onChange={(e) => setEmail(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && canSave) onSaveIdentity(name, email);
            }}
          />

          <div className="form-footer">
            <span className="hint">只写进 git 配置，不会上传到任何地方</span>
            <PillButton
              variant="primary"
              icon={<IconCheck />}
              disabled={!canSave}
              onClick={() => onSaveIdentity(name, email)}
            >
              保存身份
            </PillButton>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="panel">
      <div className="panel-header">
        <span className="panel-title">提交</span>
        <span
          className={"chip" + (stagedCount > 0 ? " accent" : "")}
          style={{ height: 24 }}
          title={`${identityName} <${identityEmail}>`}
        >
          {stagedCount} 个文件已暂存
        </span>
      </div>

      <div className="form" onKeyDown={handleKeyDown}>
        <input
          className="field"
          placeholder="提交信息（必填）"
          value={summary}
          disabled={busy !== null}
          onChange={(e) => onSummaryChange(e.target.value)}
          spellCheck={false}
        />
        <textarea
          className="field"
          placeholder="详细描述（可选）"
          value={description}
          disabled={busy !== null}
          onChange={(e) => onDescriptionChange(e.target.value)}
          rows={3}
          spellCheck={false}
        />
        <div className="form-footer">
          <span className="hint" title={`${identityName} <${identityEmail}>`}>
            作者 {identityName} · Ctrl/⌘ + Enter 提交
          </span>
          <PillButton
            variant="primary"
            icon={<IconCommit />}
            disabled={!canCommit}
            onClick={onCommit}
          >
            提交
          </PillButton>
        </div>
      </div>
    </section>
  );
}
