import { useState } from "react";
import type { KeyboardEvent } from "react";
import { PillButton } from "./PillButton";
import { IconCheck, IconCommit, IconPencil } from "./icons";

interface Props {
  summary: string;
  description: string;
  stagedCount: number;
  busy: string | null;
  // 当前生效的提交身份（来自 git config）
  identityName: string;
  identityEmail: string;
  /** 上一次提交的说明，用于「修补」时告诉用户会对哪一条生效 */
  lastCommitSubject: string;
  onSummaryChange: (v: string) => void;
  onDescriptionChange: (v: string) => void;
  onCommit: () => void;
  /** 修补最后一次提交（把暂存的内容合进去，或改提交信息） */
  onAmend: () => void;
  onSaveIdentity: (name: string, email: string) => void;
}

export function CommitPanel({
  summary,
  description,
  stagedCount,
  busy,
  identityName,
  identityEmail,
  lastCommitSubject,
  onSummaryChange,
  onDescriptionChange,
  onCommit,
  onAmend,
  onSaveIdentity,
}: Props) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  // 修补模式：把改动合进上一次提交，而不是新增一条。
  // 「忘了加一个文件」是极高频的场景，以前只能去命令目录里搜 commit.amend。
  const [amend, setAmend] = useState(false);

  // git 提交必须知道「你是谁」。没配置的话直接在面板里引导填，
  // 而不是等用户点了提交再丢一句 fatal 报错。
  const identityMissing = !identityName || !identityEmail;

  // 修补模式下允许「只改提交信息」（不带改动也能提交）
  const canCommit =
    !identityMissing &&
    summary.trim().length > 0 &&
    (stagedCount > 0 || amend) &&
    busy === null;
  const canSave = name.trim().length > 0 && email.includes("@") && busy === null;

  const handleKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter" && canCommit) {
      e.preventDefault();
      if (amend) onAmend();
      else onCommit();
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

        {lastCommitSubject && (
          <label className="amend-toggle" title={lastCommitSubject}>
            <input
              type="checkbox"
              checked={amend}
              disabled={busy !== null}
              onChange={(e) => setAmend(e.target.checked)}
            />
            修补上一次提交
            <span className="amend-subject">（{truncate(lastCommitSubject, 28)}）</span>
          </label>
        )}

        {amend && (
          <div className="hint">
            改动会合进上一次提交，不会新增提交记录。只改提交信息也可以。
          </div>
        )}

        <div className="form-footer">
          <span className="hint" title={`${identityName} <${identityEmail}>`}>
            作者 {identityName} · Ctrl/⌘ + Enter{" "}
            {amend ? "修补" : "提交"}
          </span>
          <PillButton
            variant={amend ? "success" : "primary"}
            icon={amend ? <IconPencil /> : <IconCommit />}
            disabled={!canCommit}
            onClick={amend ? onAmend : onCommit}
          >
            {amend ? "修补提交" : "提交"}
          </PillButton>
        </div>
      </div>
    </section>
  );
}

/** 提交说明太长时截断，避免把开关挤到换行。 */
function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max - 1) + "…" : s;
}
