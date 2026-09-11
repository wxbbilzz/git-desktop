import { useEffect, useState } from "react";
import { api, onCloneProgress } from "../api";
import type { CloneProgress, RepoSnapshot } from "../types";
import { PillButton } from "./PillButton";
import {
  IconArrowLeft,
  IconDownload,
  IconFolder,
  IconPlus,
  IconRepo,
} from "./icons";

// 启动界面：没有打开任何仓库时显示。
// 提供三种入口 —— 新建仓库 / 打开本地仓库 / 从网址下载仓库。

type Mode = "choose" | "create" | "clone";

interface Props {
  onOpened: (snapshot: RepoSnapshot) => void;
  /** 由上层统一处理「这个文件夹能不能当仓库」的判断 */
  onOpenFolder: (path: string) => void;
}

/** 仅用于界面预览的路径拼接（真实提交时走引擎的 JoinPath）。 */
function displayJoin(dir: string, name: string): string {
  if (!dir) return name;
  if (!name) return dir;
  return dir.endsWith("/") ? dir + name : dir + "/" + name;
}

export function Welcome({ onOpened, onOpenFolder }: Props) {
  const [mode, setMode] = useState<Mode>("choose");
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // 新建仓库表单
  const [baseDir, setBaseDir] = useState("");
  const [repoName, setRepoName] = useState("");
  const [initialBranch, setInitialBranch] = useState("main");

  // 下载仓库表单
  const [url, setUrl] = useState("");
  const [cloneDir, setCloneDir] = useState("");
  const [cloneName, setCloneName] = useState("");
  // 用户一旦手动改过目录名，就不再被 URL 自动覆盖
  const [nameTouched, setNameTouched] = useState(false);
  const [progress, setProgress] = useState<CloneProgress | null>(null);
  // 浅克隆：只取最近一次提交，大仓库能快很多
  const [shallow, setShallow] = useState(false);

  // 取默认存放位置
  useEffect(() => {
    let cancelled = false;
    void api.defaultBaseDir().then((dir) => {
      if (cancelled) return;
      setBaseDir((v) => v || dir);
      setCloneDir((v) => v || dir);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  // 订阅克隆进度
  useEffect(() => {
    return onCloneProgress((p) => {
      // 只保留有意义的进度，避免日志行把界面刷得太乱
      if (p.percent >= 0 || p.phase) setProgress(p);
    });
  }, []);

  // URL 变化时自动推断目录名。
  //
  // 这里必须防抖：推断要走一次 IPC 到 Go，如果每敲一个字符都调用，会在输入
  // 过程中反复触发状态更新和布局重排，导致正在输入的内容被打断（实测会把地址
  // 截断）。等输入停顿下来再推断，既省掉几十次 IPC，也让输入过程稳定。
  useEffect(() => {
    if (nameTouched) return;
    const timer = setTimeout(() => {
      void api.deriveRepoName(url).then((n) => setCloneName(n));
    }, 300);
    return () => clearTimeout(timer);
  }, [url, nameTouched]);

  const run = async (label: string, fn: () => Promise<RepoSnapshot | null>) => {
    setBusy(label);
    setError(null);
    try {
      const snap = await fn();
      if (snap) onOpened(snap);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
      setProgress(null);
    }
  };

  // 先让用户选目录，剩下的事（是不是仓库 / 要不要初始化）交给上层
  const handleOpenExisting = async () => {
    const dir = await api.pickRepo();
    if (dir) onOpenFolder(dir);
  };

  const pickBaseDir = async (setter: (v: string) => void) => {
    const dir = await api.pickDirectory("选择存放位置");
    if (dir) setter(dir);
  };

  const handleCreate = () =>
    run("创建仓库", async () => {
      if (!repoName.trim()) {
        setError("请填写仓库名");
        return null;
      }
      return api.createRepo(baseDir, repoName.trim(), initialBranch.trim());
    });

  const handleClone = () =>
    run("下载仓库", async () => {
      if (!url.trim()) {
        setError("请填写仓库地址");
        return null;
      }
      if (!cloneName.trim()) {
        setError("请填写文件夹名");
        return null;
      }
      setProgress({ phase: "连接中", percent: -1, detail: "" });
      const dest = await api.joinPath(cloneDir, cloneName.trim());
      return api.cloneRepo(url.trim(), dest, shallow ? 1 : 0);
    });

  // ---------------------------------------------------------------- 选择屏
  if (mode === "choose") {
    return (
      <div className="welcome">
        <div className="welcome-head">
          <span className="brand-dot" style={{ width: 14, height: 14 }} />
          <h1 className="welcome-title">Lazygit Desktop</h1>
          <p className="welcome-sub">选择一个开始方式</p>
        </div>

        <div className="choice-row">
          <button className="choice" onClick={() => setMode("create")}>
            <span className="choice-icon">
              <IconPlus />
            </span>
            <span className="choice-title">新建仓库</span>
            <span className="choice-desc">
              在本地新建一个
              <br />
              git 仓库
            </span>
          </button>

          <button
            className="choice"
            disabled={busy !== null}
            onClick={handleOpenExisting}
          >
            <span className="choice-icon">
              <IconFolder />
            </span>
            <span className="choice-title">打开仓库</span>
            <span className="choice-desc">
              选择电脑上已有的
              <br />
              本地仓库
            </span>
          </button>

          <button className="choice" onClick={() => setMode("clone")}>
            <span className="choice-icon">
              <IconDownload />
            </span>
            <span className="choice-title">下载仓库</span>
            <span className="choice-desc">
              从 GitHub / Gitee
              <br />
              等网址克隆
            </span>
          </button>
        </div>

        {busy && (
          <div className="banner">
            <span className="spinner" />
            正在{busy}…
          </div>
        )}
        {error && <div className="banner error">{error}</div>}
      </div>
    );
  }

  // ---------------------------------------------------------------- 表单屏
  return (
    <div className="welcome">
      <div className="welcome-form">
        <div className="form-head">
          <PillButton
            size="sm"
            variant="ghost"
            icon={<IconArrowLeft />}
            disabled={busy !== null}
            onClick={() => {
              setMode("choose");
              setError(null);
            }}
          >
            返回
          </PillButton>
          <span className="form-title">
            {mode === "create" ? (
              <>
                <IconRepo /> 新建仓库
              </>
            ) : (
              <>
                <IconDownload /> 下载仓库
              </>
            )}
          </span>
        </div>

        {mode === "create" && (
          <>
            <label className="label">存放位置</label>
            <div className="input-row">
              <input
                className="field mono"
                value={baseDir}
                onChange={(e) => setBaseDir(e.target.value)}
                placeholder="~/projects"
                spellCheck={false}
              />
              <PillButton
                disabled={busy !== null}
                onClick={() => void pickBaseDir(setBaseDir)}
              >
                <IconFolder /> 选择
              </PillButton>
            </div>

            <label className="label">仓库名</label>
            <input
              className="field mono"
              value={repoName}
              onChange={(e) => setRepoName(e.target.value)}
              placeholder="my-project"
              spellCheck={false}
              autoFocus
            />

            <label className="label">初始分支名</label>
            <input
              className="field mono"
              value={initialBranch}
              onChange={(e) => setInitialBranch(e.target.value)}
              placeholder="main"
              spellCheck={false}
            />

            {repoName && (
              <div className="path-preview">
                将创建于 {displayJoin(baseDir, repoName)}
              </div>
            )}
          </>
        )}

        {mode === "clone" && (
          <>
            <label className="label">仓库地址</label>
            <input
              className="field mono"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://github.com/user/repo.git"
              spellCheck={false}
              autoFocus
            />
            <div className="hint-row">
              支持 GitHub、Gitee、GitLab、自建 Git 服务，以及
              <code>git@host:user/repo.git</code> 形式的 SSH 地址。
            </div>

            <label className="label">存放位置</label>
            <div className="input-row">
              <input
                className="field mono"
                value={cloneDir}
                onChange={(e) => setCloneDir(e.target.value)}
                placeholder="~/projects"
                spellCheck={false}
              />
              <PillButton
                disabled={busy !== null}
                onClick={() => void pickBaseDir(setCloneDir)}
              >
                <IconFolder /> 选择
              </PillButton>
            </div>

            <label className="label">文件夹名</label>
            <input
              className="field mono"
              value={cloneName}
              onChange={(e) => {
                setNameTouched(true);
                setCloneName(e.target.value);
              }}
              placeholder="repo"
              spellCheck={false}
            />

            <label className="check-row">
              <input
                type="checkbox"
                checked={shallow}
                disabled={busy !== null}
                onChange={(e) => setShallow(e.target.checked)}
              />
              <span>
                浅克隆
                <em>只下载最近一次提交，大仓库快很多（拿不到完整历史）</em>
              </span>
            </label>

            {cloneName && (
              <div className="path-preview">
                将下载到 {displayJoin(cloneDir, cloneName)}
              </div>
            )}

            {progress && (
              <div className="progress-wrap">
                <div className="progress-bar">
                  <div
                    className={
                      "progress-fill" +
                      (progress.percent < 0 ? " indeterminate" : "")
                    }
                    style={{
                      width:
                        progress.percent >= 0 ? `${progress.percent}%` : "35%",
                    }}
                  />
                </div>
                <div className="progress-text">
                  {progress.phase}
                  {progress.percent >= 0 ? ` ${progress.percent}%` : "…"}
                </div>
              </div>
            )}
          </>
        )}

        {error && <div className="banner error">{error}</div>}

        <div className="form-actions">
          <PillButton
            variant="primary"
            disabled={
              busy !== null ||
              (mode === "create" ? !repoName.trim() : !url.trim() || !cloneName.trim())
            }
            onClick={mode === "create" ? handleCreate : handleClone}
          >
            {busy
              ? `正在${busy}…`
              : mode === "create"
                ? "创建并打开"
                : "下载并打开"}
          </PillButton>
        </div>
      </div>
    </div>
  );
}
