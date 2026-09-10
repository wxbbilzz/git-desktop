import { useCallback, useEffect, useState } from "react";
import { api, isDesktop } from "./api";
import type { RepoSnapshot, SidebarTab } from "./types";
import { TopBar } from "./components/TopBar";
import { Sidebar } from "./components/Sidebar";
import { DiffPanel } from "./components/DiffPanel";
import { CommitPanel } from "./components/CommitPanel";
import { HistoryPanel } from "./components/HistoryPanel";
import { Welcome } from "./components/Welcome";
import { Operations } from "./components/Operations";

// App 只负责“编排”：持有界面状态、调用 api、把结果分发到各面板。
// 它不包含任何 git 逻辑 —— 那是引擎的职责。
// 这也是重写 UI 后最直接的好处：界面可以随便改，只要 api 契约不变。
export default function App() {
  const [snapshot, setSnapshot] = useState<RepoSnapshot | null>(null);
  const [tab, setTab] = useState<SidebarTab>("changes");

  // 选中的目标：文件或提交，两者互斥
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [selectedStaged, setSelectedStaged] = useState(false);
  const [selectedCommit, setSelectedCommit] = useState<string | null>(null);

  const [diff, setDiff] = useState("");
  const [diffLoading, setDiffLoading] = useState(false);

  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");

  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  // 是否打开「Git 操作」面板（全命令入口）
  const [showOps, setShowOps] = useState(false);

  /** 统一处理一次“动作 → 新快照”的往返，并维护忙碌态与错误提示。 */
  const run = useCallback(
    async (label: string, fn: () => Promise<RepoSnapshot | null>) => {
      setBusy(label);
      setError(null);
      try {
        const next = await fn();
        if (next) setSnapshot(next);
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setBusy(null);
      }
    },
    [],
  );

  const refresh = useCallback(async () => {
    await run("刷新", () => api.snapshot());
  }, [run]);

  useEffect(() => {
    void refresh();
    // 只在挂载时加载一次
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 选中目标变化时拉取对应的 diff
  useEffect(() => {
    let cancelled = false;

    async function load() {
      if (!selectedPath && !selectedCommit) {
        setDiff("");
        return;
      }
      setDiffLoading(true);
      try {
        const text = selectedCommit
          ? await api.commitDiff(selectedCommit)
          : await api.fileDiff(selectedPath as string, selectedStaged);
        if (!cancelled) setDiff(text);
      } catch (e) {
        if (!cancelled) {
          setDiff("");
          setError(e instanceof Error ? e.message : String(e));
        }
      } finally {
        if (!cancelled) setDiffLoading(false);
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [selectedPath, selectedStaged, selectedCommit]);

  // 仓库变了（比如切换仓库）后，之前的选中项可能已经不存在，清掉更安全
  useEffect(() => {
    setSelectedPath(null);
    setSelectedCommit(null);
  }, [snapshot?.repoPath]);

  // 没有打开任何仓库时，显示启动界面：
  // 新建仓库 / 打开本地仓库 / 从网址下载仓库 三个入口都在那里。
  if (!snapshot) {
    return (
      <div className="app">
        <Welcome onOpened={setSnapshot} />
      </div>
    );
  }

  const stagedCount = snapshot.files.filter((f) => f.isStaged).length;

  const selectFile = (path: string, staged: boolean) => {
    setSelectedCommit(null);
    setSelectedPath(path);
    setSelectedStaged(staged);
  };

  const selectCommit = (hash: string) => {
    setSelectedPath(null);
    setSelectedCommit(hash);
  };

  const handleDiscard = (path: string) => {
    const ok = window.confirm(
      `确定丢弃「${path}」的改动吗？\n\n这个操作不可撤销。`,
    );
    if (!ok) return;
    void run("丢弃改动", () => api.discardFile(path));
  };

  const handleCommit = () => {
    if (summary.trim().length === 0 || stagedCount === 0) return;
    void run("提交", async () => {
      const next = await api.commit(summary, description);
      setSummary("");
      setDescription("");
      return next;
    });
  };

  const handleOpenRepo = () => {
    void run("打开仓库", async () => {
      const next = await api.chooseAndOpenRepo();
      return next;
    });
  };

  return (
    <div className="app">
      <TopBar
        snapshot={snapshot}
        busy={busy}
        onFetch={() => void run("Fetch", () => api.fetch())}
        onPull={() => void run("Pull", () => api.pull())}
        onPush={() => void run("Push", () => api.push())}
        onRefresh={() => void refresh()}
        onOpenRepo={handleOpenRepo}
        onHome={() => setSnapshot(null)}
        onOperations={() => setShowOps(true)}
      />

      {(busy || error || !isDesktop()) && (
        <div className={"banner" + (error ? " error" : "")}>
          {busy && <span className="spinner" />}
          {error
            ? error
            : busy
              ? `正在${busy}…`
              : "浏览器预览模式：当前是演示数据，用 wails dev 运行才会操作真实仓库。"}
        </div>
      )}

      <div className="layout">
        <Sidebar
          snapshot={snapshot}
          tab={tab}
          onTabChange={setTab}
          selectedPath={selectedPath}
          selectedStaged={selectedStaged}
          busy={busy}
          onSelectFile={selectFile}
          onStage={(path) => void run("暂存", () => api.stageFile(path))}
          onUnstage={(path) => void run("取消暂存", () => api.unstageFile(path))}
          onStageAll={() => void run("暂存全部", () => api.stageAll())}
          onUnstageAll={() => void run("取消全部暂存", () => api.unstageAll())}
          onDiscard={handleDiscard}
          onCheckoutBranch={(name) =>
            void run("切换分支", () => api.checkoutBranch(name))
          }
        />

        <DiffPanel
          mode={selectedCommit ? "commit" : "file"}
          path={selectedPath}
          hash={selectedCommit}
          staged={selectedStaged}
          diff={diff}
          loading={diffLoading}
          busy={busy}
          onStage={() => {
            // 显式守卫：闭包里不能依赖外层 selectedPath 的类型收窄
            if (!selectedPath) return;
            void run("暂存", () => api.stageFile(selectedPath));
          }}
          onUnstage={() => {
            if (!selectedPath) return;
            void run("取消暂存", () => api.unstageFile(selectedPath));
          }}
          onDiscard={() => {
            if (!selectedPath) return;
            handleDiscard(selectedPath);
          }}
        />

        <div className="right-col">
          <CommitPanel
            summary={summary}
            description={description}
            stagedCount={stagedCount}
            busy={busy}
            identityName={snapshot.identityName}
            identityEmail={snapshot.identityEmail}
            onSummaryChange={setSummary}
            onDescriptionChange={setDescription}
            onCommit={handleCommit}
            onSaveIdentity={(n, e) =>
              void run("保存身份", () => api.setIdentity(n, e, true))
            }
          />
          <HistoryPanel
            commits={snapshot.commits}
            selectedHash={selectedCommit}
            onSelect={selectCommit}
          />
        </div>
      </div>

      {showOps && (
        <Operations
          onClose={() => setShowOps(false)}
          onSnapshot={(r) => {
            // 操作改变了仓库状态，用引擎回传的新快照刷新界面
            if (r.snapshot) setSnapshot(r.snapshot);
          }}
        />
      )}
    </div>
  );
}
