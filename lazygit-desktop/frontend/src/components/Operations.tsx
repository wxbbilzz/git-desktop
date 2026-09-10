import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import type { OperationSummary, Param, RunResult } from "../types";
import { PillButton } from "./PillButton";
import { IconCheck, IconRefresh, IconTrash } from "./icons";

// 「Git 操作」面板：把 git 的命令目录渲染成可搜索、可填表、可执行的界面。
//
// 这是「全命令 GUI 化」的前端一半。它不认识任何具体命令，
// 完全由后端目录驱动 —— 后端加一条数据，这里就多一个操作。

interface Props {
  onClose: () => void;
  onSnapshot: (r: RunResult) => void;
}

// 伪操作：原始命令，作为目录覆盖不到时的兜底
const RAW: OperationSummary = {
  id: "__raw__",
  category: "原始命令",
  name: "执行任意 git 命令",
  description:
    "直接输入任意 git 子命令与参数，覆盖上面没有收录的命令（尤其是底层 plumbing 命令）。支持引号。",
  params: [
    {
      name: "command",
      label: "命令（可省略开头的 git）",
      kind: "text",
      required: true,
      default: "",
      placeholder: 'log --oneline -n 20\ncat-file -p HEAD:README.md\nls-files "*.go"',
      choices: [],
      help: "",
      flag: "",
    },
  ],
  dangerous: false,
  readOnly: false,
};

export function Operations({ onClose, onSnapshot }: Props) {
  const [ops, setOps] = useState<OperationSummary[]>([]);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("全部");
  const [selected, setSelected] = useState<OperationSummary | null>(null);
  const [args, setArgs] = useState<Record<string, string>>({});
  const [result, setResult] = useState<RunResult | null>(null);
  const [running, setRunning] = useState(false);

  useEffect(() => {
    void api.operations().then(setOps);
  }, []);

  // Esc 关闭
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [onClose]);

  const all = useMemo(() => [...ops, RAW], [ops]);

  const categories = useMemo(() => {
    const set: string[] = [];
    for (const op of all) {
      if (!set.includes(op.category)) set.push(op.category);
    }
    return ["全部", ...set];
  }, [all]);

  // 搜索匹配名称/描述/分类
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return all.filter((op) => {
      if (category !== "全部" && op.category !== category) return false;
      if (!q) return true;
      return (
        op.name.toLowerCase().includes(q) ||
        op.description.toLowerCase().includes(q) ||
        op.category.toLowerCase().includes(q) ||
        op.id.toLowerCase().includes(q)
      );
    });
  }, [all, query, category]);

  const select = (op: OperationSummary) => {
    setSelected(op);
    setResult(null);
    const init: Record<string, string> = {};
    for (const p of op.params) init[p.name] = p.default ?? "";
    setArgs(init);
  };

  const run = async () => {
    if (!selected) return;
    if (selected.dangerous) {
      const ok = window.confirm(
        `「${selected.name}」是一个不可逆操作，确定执行吗？\n\n${selected.description}`,
      );
      if (!ok) return;
    }

    setRunning(true);
    setResult(null);
    try {
      const res =
        selected.id === "__raw__"
          ? await api.runRawGit(args["command"] ?? "")
          : await api.runOperation(selected.id, args);

      setResult(res);
      if (res.snapshot) onSnapshot(res);
    } catch (e) {
      setResult({
        operationId: selected.id,
        command: "",
        output: "",
        ok: false,
        error: e instanceof Error ? e.message : String(e),
        snapshot: null,
      });
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="ops-overlay" onClick={onClose}>
      <div className="ops-panel" onClick={(e) => e.stopPropagation()}>
        <div className="ops-head">
          <span className="panel-title">Git 操作</span>
          <span className="chip" style={{ height: 24 }}>
            共 {all.length} 项
          </span>
          <span className="spacer" />
          <PillButton size="sm" variant="ghost" onClick={onClose}>
            关闭 (Esc)
          </PillButton>
        </div>

        <div className="ops-search">
          <input
            className="field"
            placeholder="搜索命令，例如：合并 / rebase / tag / stash"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            spellCheck={false}
            autoFocus
          />
        </div>

        <div className="ops-body">
          {/* 分类 */}
          <div className="ops-categories">
            {categories.map((c) => (
              <button
                key={c}
                className={"ops-cat" + (category === c ? " active" : "")}
                onClick={() => setCategory(c)}
              >
                {c}
              </button>
            ))}
          </div>

          {/* 操作列表 */}
          <div className="ops-list">
            {filtered.map((op) => (
              <button
                key={op.id}
                className={"ops-item" + (selected?.id === op.id ? " active" : "")}
                onClick={() => select(op)}
              >
                <span className="ops-item-name">{op.name}</span>
                <span className="ops-item-cat">
                  {op.category}
                  {op.dangerous && <span className="ops-danger">危险</span>}
                  {op.readOnly && <span className="ops-ro">只读</span>}
                </span>
              </button>
            ))}
            {filtered.length === 0 && (
              <div className="empty">
                <div className="empty-text">没有匹配的操作</div>
              </div>
            )}
          </div>

          {/* 详情与执行 */}
          <div className="ops-detail">
            {!selected && (
              <div className="empty">
                <div className="empty-title">选一个操作</div>
                <div className="empty-text">
                  从中间列表选择，或直接用搜索。找不到的命令用「原始命令」。
                </div>
              </div>
            )}

            {selected && (
              <>
                <div className="ops-detail-head">
                  <div className="ops-detail-title">{selected.name}</div>
                  <div className="ops-detail-desc">{selected.description}</div>
                </div>

                <div className="ops-form">
                  {selected.params.length === 0 && (
                    <div className="hint-row">这个操作没有参数，直接执行即可。</div>
                  )}

                  {selected.params.map((p) => (
                    <ParamField
                      key={p.name}
                      param={p}
                      value={args[p.name] ?? ""}
                      disabled={running}
                      onChange={(v) => setArgs((a) => ({ ...a, [p.name]: v }))}
                    />
                  ))}
                </div>

                <div className="ops-actions">
                  <PillButton
                    variant={selected.dangerous ? "danger" : "primary"}
                    icon={selected.dangerous ? <IconTrash /> : <IconCheck />}
                    disabled={running}
                    onClick={() => void run()}
                  >
                    {running ? "执行中…" : selected.readOnly ? "查询" : "执行"}
                  </PillButton>
                  <PillButton
                    variant="ghost"
                    icon={<IconRefresh />}
                    disabled={running}
                    onClick={() => select(selected)}
                  >
                    重置参数
                  </PillButton>
                </div>

                {result && (
                  <div className="ops-result">
                    {result.command && (
                      <div className="ops-cmd">
                        <span className="ops-cmd-prompt">$</span> {result.command}
                      </div>
                    )}
                    <div className={"ops-out" + (result.ok ? "" : " bad")}>
                      {result.output ||
                        (result.ok ? "（命令执行成功，没有输出）" : "")}
                      {!result.ok && result.error && (
                        <div className="ops-err">{result.error}</div>
                      )}
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

/** 按参数类型渲染一个输入控件 */
function ParamField({
  param,
  value,
  disabled,
  onChange,
}: {
  param: Param;
  value: string;
  disabled: boolean;
  onChange: (v: string) => void;
}) {
  const label = (
    <label className="label">
      {param.label}
      {param.required && <span style={{ color: "var(--red)" }}> *</span>}
    </label>
  );

  if (param.kind === "bool") {
    return (
      <label className="check-row">
        <input
          type="checkbox"
          checked={value === "true"}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked ? "true" : "false")}
        />
        <span>
          {param.label}
          {param.flag && <em>对应参数 {param.flag}</em>}
        </span>
      </label>
    );
  }

  if (param.kind === "text") {
    return (
      <>
        {label}
        <textarea
          className="field mono"
          rows={3}
          value={value}
          disabled={disabled}
          placeholder={param.placeholder}
          spellCheck={false}
          onChange={(e) => onChange(e.target.value)}
        />
      </>
    );
  }

  return (
    <>
      {label}
      <input
        className="field mono"
        value={value}
        disabled={disabled}
        placeholder={param.placeholder}
        spellCheck={false}
        onChange={(e) => onChange(e.target.value)}
      />
    </>
  );
}
