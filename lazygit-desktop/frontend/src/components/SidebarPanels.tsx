import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import type { RemoteDTO, RepoSnapshot, StashEntryDTO, TagDTO } from "../types";
import { api } from "../api";
import { PillButton } from "./PillButton";
import {
  IconArchive,
  IconCheck,
  IconCloud,
  IconCopy,
  IconMinus,
  IconPlus,
  IconSearch,
  IconTag,
  IconTrash,
  IconUpload,
} from "./icons";

// 侧栏里那几个「列表 + 表单」面板：标签、远端、储藏。
//
// 它们之前只存在于「Git 操作」的全命令目录里 —— 想打个标签得先打开操作面板、
// 搜到 tag.create、再填表单。这些都是每天要用的动作，所以提到侧栏做成直接可点的界面。
// 三个面板的结构一致：一个可过滤的列表 + 一个可展开的新建表单。

/** 列表上方的过滤框。列表一长，没有它就只能拿眼睛翻。 */
export function FilterBox({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
}) {
  return (
    <div className="filter-box">
      <IconSearch />
      <input
        className="filter-input"
        placeholder={placeholder}
        value={value}
        spellCheck={false}
        onChange={(e) => onChange(e.target.value)}
      />
      {value && (
        <button className="filter-clear" title="清空" onClick={() => onChange("")}>
          <IconMinus />
        </button>
      )}
    </div>
  );
}

/** 列表为空时的统一提示。 */
function Empty({ title, text }: { title: string; text: ReactNode }) {
  return (
    <div className="empty">
      <div className="empty-title">{title}</div>
      <div className="empty-text">{text}</div>
    </div>
  );
}

/** 复制一段文本到剪贴板，并给一个短暂的「已复制」反馈。 */
function CopyButton({ text, title }: { text: string; title: string }) {
  const [done, setDone] = useState(false);

  useEffect(() => {
    if (!done) return;
    const t = setTimeout(() => setDone(false), 1200);
    return () => clearTimeout(t);
  }, [done]);

  return (
    <PillButton
      size="sm"
      variant="ghost"
      icon={done ? <IconCheck /> : <IconCopy />}
      title={done ? "已复制" : title}
      onClick={() => {
        void navigator.clipboard?.writeText(text).then(
          () => setDone(true),
          () => setDone(false),
        );
      }}
    />
  );
}

// ---------------------------------------------------------------- 标签

export function TagsPanel({
  snapshot,
  busy,
  presetRef,
  onCreate,
  onDelete,
  onPush,
  onPushAll,
}: {
  snapshot: RepoSnapshot;
  busy: string | null;
  /** 从历史面板「给这次提交打标签」带过来的提交号；有值时标签打在它上面 */
  presetRef?: string | null;
  onCreate: (name: string, ref: string, message: string) => void;
  onDelete: (name: string) => void;
  onPush: (name: string, remote: string) => void;
  onPushAll: (remote: string) => void;
}) {
  const [filter, setFilter] = useState("");
  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");
  // 目标提交：默认 HEAD，从历史面板过来时是那一次提交
  const [ref, setRef] = useState(presetRef ?? "");

  const defaultRemote = snapshot.remotes[0]?.name || "origin";

  const tags = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return snapshot.tags;
    return snapshot.tags.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        t.subject.toLowerCase().includes(q),
    );
  }, [snapshot.tags, filter]);

  const submit = () => {
    if (!name.trim()) return;
    onCreate(name.trim(), ref.trim(), message.trim());
    setFormOpen(false);
    setName("");
    setMessage("");
  };

  return (
    <>
      {formOpen ? (
        <div className="branch-form">
          <label className="label">标签名</label>
          <input
            className="field mono"
            placeholder="v1.0.0"
            value={name}
            autoFocus
            disabled={busy !== null}
            spellCheck={false}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") submit();
            }}
          />
          <label className="label">说明（留空 = 轻量标签）</label>
          <input
            className="field"
            placeholder="发布 1.0"
            value={message}
            disabled={busy !== null}
            spellCheck={false}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") submit();
            }}
          />
          <label className="label">打在哪个提交上</label>
          <input
            className="field mono"
            placeholder="留空 = 当前提交"
            value={ref}
            disabled={busy !== null}
            spellCheck={false}
            onChange={(e) => setRef(e.target.value)}
          />
          <div className="hint">
            {ref ? `标签会打在 ${ref.slice(0, 10)} 上。` : "标签会打在当前的提交上。"}
          </div>
          <div className="branch-form-actions">
            <PillButton
              size="sm"
              variant="primary"
              disabled={busy !== null || !name.trim()}
              onClick={submit}
            >
              创建标签
            </PillButton>
            <PillButton size="sm" variant="ghost" onClick={() => setFormOpen(false)}>
              取消
            </PillButton>
          </div>
        </div>
      ) : (
        <div className="list-actions">
          <PillButton
            size="sm"
            variant="success"
            icon={<IconPlus />}
            disabled={busy !== null}
            onClick={() => {
              setRef(presetRef ?? "");
              setFormOpen(true);
            }}
          >
            新建标签
          </PillButton>
          <PillButton
            size="sm"
            icon={<IconUpload />}
            disabled={busy !== null || snapshot.tags.length === 0}
            title={`把所有标签推送到 ${defaultRemote}`}
            onClick={() => onPushAll(defaultRemote)}
          >
            全部推送
          </PillButton>
        </div>
      )}

      {snapshot.tags.length > 4 && (
        <FilterBox value={filter} onChange={setFilter} placeholder="过滤标签…" />
      )}

      <div className="list">
        {tags.map((t: TagDTO) => (
          <div className="row" key={t.name}>
            <span className="dot new" />
            <div className="row-main">
              <div className="row-name" title={t.name}>
                {t.name}
                {t.isAnnotated && <span className="badge new">附注</span>}
              </div>
              <div className="row-sub">
                {t.shortHash}
                {t.when ? ` · ${t.when}` : ""}
                {t.subject ? ` · ${t.subject}` : ""}
              </div>
            </div>
            <div className="row-actions">
              <CopyButton text={t.name} title="复制标签名" />
              <PillButton
                size="sm"
                variant="ghost"
                icon={<IconUpload />}
                title={`推送到 ${defaultRemote}`}
                disabled={busy !== null}
                onClick={() => onPush(t.name, defaultRemote)}
              />
              <PillButton
                size="sm"
                variant="danger"
                icon={<IconTrash />}
                title="删除本地标签"
                disabled={busy !== null}
                onClick={() => onDelete(t.name)}
              />
            </div>
          </div>
        ))}

        {snapshot.tags.length === 0 && (
          <Empty
            title="还没有标签"
            text={
              <>
                <IconTag /> 标签用来标记发布点，比如 v1.0.0。
              </>
            }
          />
        )}
        {snapshot.tags.length > 0 && tags.length === 0 && (
          <Empty title="没有匹配的标签" text="换个关键词试试。" />
        )}
      </div>
    </>
  );
}

// ---------------------------------------------------------------- 远端

export function RemotesPanel({
  remotes,
  busy,
  onAdd,
  onRemove,
  onSetUrl,
}: {
  remotes: RemoteDTO[];
  busy: string | null;
  onAdd: (name: string, url: string) => void;
  onRemove: (name: string) => void;
  onSetUrl: (name: string, url: string) => void;
}) {
  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState("origin");
  const [url, setUrl] = useState("");
  const [editing, setEditing] = useState<string | null>(null);
  const [editUrl, setEditUrl] = useState("");

  return (
    <>
      {formOpen ? (
        <div className="branch-form">
          <label className="label">远端名</label>
          <input
            className="field mono"
            value={name}
            autoFocus
            disabled={busy !== null}
            spellCheck={false}
            onChange={(e) => setName(e.target.value)}
          />
          <label className="label">仓库地址</label>
          <input
            className="field mono"
            placeholder="https://github.com/用户名/仓库.git"
            value={url}
            disabled={busy !== null}
            spellCheck={false}
            onChange={(e) => setUrl(e.target.value)}
          />
          <div className="hint">也支持 git@ 开头的 SSH 地址。</div>
          <div className="branch-form-actions">
            <PillButton
              size="sm"
              variant="primary"
              disabled={busy !== null || !name.trim() || !url.trim()}
              onClick={() => {
                onAdd(name.trim(), url.trim());
                setFormOpen(false);
                setUrl("");
              }}
            >
              添加远端
            </PillButton>
            <PillButton size="sm" variant="ghost" onClick={() => setFormOpen(false)}>
              取消
            </PillButton>
          </div>
        </div>
      ) : (
        <div className="list-actions">
          <PillButton
            size="sm"
            variant="success"
            icon={<IconPlus />}
            disabled={busy !== null}
            onClick={() => setFormOpen(true)}
          >
            添加远端
          </PillButton>
        </div>
      )}

      <div className="list">
        {remotes.map((r) => (
          <div className="row" key={r.name}>
            <span className="dot new" />
            <div className="row-main">
              <div className="row-name">{r.name}</div>
              {editing === r.name ? (
                <input
                  className="field mono"
                  style={{ marginTop: 4 }}
                  value={editUrl}
                  autoFocus
                  disabled={busy !== null}
                  spellCheck={false}
                  onChange={(e) => setEditUrl(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && editUrl.trim()) {
                      onSetUrl(r.name, editUrl.trim());
                      setEditing(null);
                    }
                    if (e.key === "Escape") setEditing(null);
                  }}
                />
              ) : (
                <div className="row-sub mono" title={r.url}>
                  {r.url}
                </div>
              )}
            </div>
            <div className="row-actions">
              <CopyButton text={r.url} title="复制地址" />
              {editing === r.name ? (
                <PillButton
                  size="sm"
                  variant="success"
                  icon={<IconCheck />}
                  title="保存地址"
                  disabled={busy !== null || !editUrl.trim()}
                  onClick={() => {
                    onSetUrl(r.name, editUrl.trim());
                    setEditing(null);
                  }}
                />
              ) : (
                <PillButton
                  size="sm"
                  variant="ghost"
                  icon={<IconCloud />}
                  title="修改地址"
                  disabled={busy !== null}
                  onClick={() => {
                    setEditing(r.name);
                    setEditUrl(r.url);
                  }}
                />
              )}
              <PillButton
                size="sm"
                variant="danger"
                icon={<IconTrash />}
                title="删除这个远端"
                disabled={busy !== null}
                onClick={() => onRemove(r.name)}
              />
            </div>
          </div>
        ))}

        {remotes.length === 0 && (
          <Empty
            title="还没有配置远端"
            text={
              <>
                <IconCloud /> 添加之后就能推送、拉取了。
              </>
            }
          />
        )}
      </div>
    </>
  );
}

// ---------------------------------------------------------------- 储藏

export function StashPanel({
  busy,
  onSave,
  onPop,
  onApply,
  onDrop,
  onShow,
}: {
  busy: string | null;
  onSave: (message: string, includeUntracked: boolean) => void;
  onPop: (index: number) => void;
  onApply: (index: number) => void;
  onDrop: (index: number) => void;
  onShow: (index: number) => Promise<string>;
}) {
  const [stashes, setStashes] = useState<StashEntryDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [message, setMessage] = useState("");
  const [includeUntracked, setIncludeUntracked] = useState(true);
  const [preview, setPreview] = useState<{ index: number; text: string } | null>(
    null,
  );

  const reload = async () => {
    setLoading(true);
    try {
      setStashes(await api.stashes());
    } catch {
      setStashes([]);
    } finally {
      setLoading(false);
    }
  };

  // 储藏列表不在快照里（它是另一条查询），所以这里自己拉。
  // 依赖 busy：任何一次操作结束后都会回到 null，正好用来触发重新拉取。
  useEffect(() => {
    if (busy === null) void reload();
  }, [busy]);

  return (
    <>
      {formOpen ? (
        <div className="branch-form">
          <label className="label">储藏说明（可留空）</label>
          <input
            className="field"
            placeholder="WIP：改到一半"
            value={message}
            autoFocus
            disabled={busy !== null}
            spellCheck={false}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                onSave(message.trim(), includeUntracked);
                setFormOpen(false);
                setMessage("");
              }
            }}
          />
          <label className="stash-check">
            <input
              type="checkbox"
              checked={includeUntracked}
              disabled={busy !== null}
              onChange={(e) => setIncludeUntracked(e.target.checked)}
            />
            连未跟踪的文件一起储藏
          </label>
          <div className="branch-form-actions">
            <PillButton
              size="sm"
              variant="primary"
              disabled={busy !== null}
              onClick={() => {
                onSave(message.trim(), includeUntracked);
                setFormOpen(false);
                setMessage("");
              }}
            >
              储藏当前改动
            </PillButton>
            <PillButton size="sm" variant="ghost" onClick={() => setFormOpen(false)}>
              取消
            </PillButton>
          </div>
        </div>
      ) : (
        <div className="list-actions">
          <PillButton
            size="sm"
            variant="success"
            icon={<IconArchive />}
            disabled={busy !== null}
            onClick={() => setFormOpen(true)}
          >
            储藏当前改动
          </PillButton>
        </div>
      )}

      <div className="list">
        {stashes.map((s) => (
          <div className="row" key={s.index}>
            <span className="dot modified" />
            <div className="row-main">
              <div className="row-name" title={s.message}>
                {s.message || "(无说明)"}
              </div>
              <div className="row-sub">
                {s.ref}
                {s.branch ? ` · 来自 ${s.branch}` : ""}
              </div>
            </div>
            <div className="row-actions">
              <PillButton
                size="sm"
                variant="ghost"
                title="预览内容"
                disabled={busy !== null}
                onClick={() =>
                  void onShow(s.index).then((text) =>
                    setPreview({ index: s.index, text }),
                  )
                }
              >
                查看
              </PillButton>
              <PillButton
                size="sm"
                variant="ghost"
                title="应用（保留这条记录）"
                disabled={busy !== null}
                onClick={() => onApply(s.index)}
              >
                应用
              </PillButton>
              <PillButton
                size="sm"
                variant="success"
                title="弹出（应用并删除这条记录）"
                disabled={busy !== null}
                onClick={() => onPop(s.index)}
              >
                弹出
              </PillButton>
              <PillButton
                size="sm"
                variant="danger"
                icon={<IconTrash />}
                title="删除这条储藏"
                disabled={busy !== null}
                onClick={() => onDrop(s.index)}
              />
            </div>
          </div>
        ))}

        {stashes.length === 0 && !loading && (
          <Empty
            title="没有储藏"
            text={
              <>
                <IconArchive /> 改到一半要切分支时，把改动储藏起来。
              </>
            }
          />
        )}
        {loading && <div className="hint">正在读取储藏…</div>}
      </div>

      {preview && (
        <div className="ops-overlay" onClick={() => setPreview(null)}>
          <div className="ops-panel" onClick={(e) => e.stopPropagation()}>
            <div className="ops-head">
              <span className="panel-title">stash@{"{" + preview.index + "}"} 的内容</span>
              <span className="spacer" />
              <PillButton size="sm" variant="ghost" onClick={() => setPreview(null)}>
                关闭
              </PillButton>
            </div>
            <pre className="ops-out" style={{ maxHeight: "60vh" }}>
              {preview.text || "(空)"}
            </pre>
          </div>
        </div>
      )}
    </>
  );
}
