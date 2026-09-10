import { useEffect, useState } from "react";
import { api, onPublishProgress } from "../api";
import type { PublishResult, RepoSnapshot } from "../types";
import { PillButton } from "./PillButton";
import { IconCheck, IconDownload, IconPush } from "./icons";

// 「上传到 GitHub / Gitee」对话框。
//
// 做三件事：调用平台 API 创建远端仓库 -> 配置 origin -> 推送当前分支。
// 需要用户提供访问令牌（token），因为创建仓库是平台 API 的写操作。

interface Props {
  snapshot: RepoSnapshot;
  onClose: () => void;
  onPublished: (r: PublishResult) => void;
}

type Platform = "github" | "gitee";

const PLATFORM_INFO: Record<
  Platform,
  { label: string; host: string; tokenURL: string; tokenHint: string }
> = {
  gitee: {
    label: "Gitee",
    host: "gitee.com",
    tokenURL: "https://gitee.com/personal_access_tokens",
    tokenHint: "需要 projects 权限",
  },
  github: {
    label: "GitHub",
    host: "github.com",
    tokenURL: "https://github.com/settings/tokens",
    tokenHint: "需要 repo 权限",
  },
};

export function PublishDialog({ snapshot, onClose, onPublished }: Props) {
  const [platform, setPlatform] = useState<Platform>("gitee");
  const [token, setToken] = useState("");
  const [name, setName] = useState(snapshot.repoName || "");
  const [description, setDescription] = useState("");
  const [privateRepo, setPrivateRepo] = useState(false);
  const [storeToken, setStoreToken] = useState(true);

  const [busy, setBusy] = useState(false);
  const [step, setStep] = useState("");
  const [result, setResult] = useState<PublishResult | null>(null);

  useEffect(() => {
    const off = onPublishProgress((s) => setStep(s));
    return off;
  }, []);

  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [busy, onClose]);

  const info = PLATFORM_INFO[platform];
  const canSubmit = token.trim().length > 0 && name.trim().length > 0 && !busy;

  const submit = async () => {
    if (!canSubmit) return;
    setBusy(true);
    setResult(null);
    setStep("准备中");
    try {
      const res = await api.publish(
        platform,
        token.trim(),
        name.trim(),
        description.trim(),
        privateRepo,
        storeToken,
      );
      setResult(res);
      if (res.snapshot) onPublished(res);
    } catch (e) {
      setResult({
        repoUrl: "",
        cloneUrl: "",
        command: "",
        output: "",
        ok: false,
        error: e instanceof Error ? e.message : String(e),
        snapshot: null,
      });
    } finally {
      setBusy(false);
      setStep("");
    }
  };

  return (
    <div className="ops-overlay" onClick={() => !busy && onClose()}>
      <div className="ops-panel publish-panel" onClick={(e) => e.stopPropagation()}>
        <div className="ops-head">
          <span className="panel-title">上传到代码托管平台</span>
          <span className="chip" style={{ height: 24 }}>
            {snapshot.repoName}
          </span>
          <span className="spacer" />
          <PillButton size="sm" variant="ghost" disabled={busy} onClick={onClose}>
            关闭
          </PillButton>
        </div>

        <div className="publish-body">
          <div className="label">选择平台</div>
          <div className="tabs">
            {(Object.keys(PLATFORM_INFO) as Platform[]).map((p) => (
              <button
                key={p}
                className={"tab" + (platform === p ? " active" : "")}
                disabled={busy}
                onClick={() => setPlatform(p)}
              >
                {PLATFORM_INFO[p].label}
              </button>
            ))}
          </div>

          <div className="label">访问令牌（token）</div>
          <input
            className="field mono"
            type="password"
            placeholder="粘贴你的 access token"
            value={token}
            disabled={busy}
            spellCheck={false}
            onChange={(e) => setToken(e.target.value)}
          />
          <div className="hint-row">
            到{" "}
            <a href={info.tokenURL} target="_blank" rel="noreferrer">
              {info.host} 的令牌页面
            </a>{" "}
            生成，{info.tokenHint}。令牌只用于创建仓库和推送，不会上传到别处。
          </div>

          <div className="label">仓库名</div>
          <input
            className="field mono"
            value={name}
            disabled={busy}
            spellCheck={false}
            onChange={(e) => setName(e.target.value)}
          />

          <div className="label">仓库描述（可选）</div>
          <input
            className="field"
            placeholder="一句话说明这个项目"
            value={description}
            disabled={busy}
            spellCheck={false}
            onChange={(e) => setDescription(e.target.value)}
          />

          <label className="check-row">
            <input
              type="checkbox"
              checked={privateRepo}
              disabled={busy}
              onChange={(e) => setPrivateRepo(e.target.checked)}
            />
            <span>
              创建为私有仓库
              <em>不勾选则是公开仓库，任何人都能看到</em>
            </span>
          </label>

          <label className="check-row">
            <input
              type="checkbox"
              checked={storeToken}
              disabled={busy}
              onChange={(e) => setStoreToken(e.target.checked)}
            />
            <span>
              记住凭据
              <em>
                把令牌写进本仓库的 .git/config（不会提交、不会外传），
                这样以后在软件里点 Push 也能直接用
              </em>
            </span>
          </label>

          <div className="hint-row" style={{ marginTop: 4 }}>
            将会推送当前分支 <code>{snapshot.branch || "(未知)"}</code>，
            并把远端添加为 <code>origin</code>。
          </div>

          {busy && (
            <div className="banner" style={{ marginTop: 8 }}>
              <span className="spinner" />
              {step || "正在上传…"}
            </div>
          )}

          {result && (
            <div className={"banner" + (result.ok ? "" : " error")} style={{ marginTop: 8 }}>
              {result.ok ? (
                <>
                  <IconCheck />
                  上传成功！仓库地址：
                  <a href={result.repoUrl} target="_blank" rel="noreferrer">
                    {result.repoUrl}
                  </a>
                </>
              ) : (
                <>上传失败：{result.error}</>
              )}
            </div>
          )}

          {result?.output && (
            <div className="ops-out" style={{ marginTop: 8 }}>
              {result.command && <div style={{ color: "var(--accent-2)" }}>$ {result.command}</div>}
              {result.output}
            </div>
          )}

          <div className="form-actions">
            <PillButton
              variant="primary"
              icon={<IconPush />}
              disabled={!canSubmit}
              onClick={() => void submit()}
            >
              {busy ? "上传中…" : "创建并上传"}
            </PillButton>
          </div>
        </div>
      </div>
    </div>
  );
}
