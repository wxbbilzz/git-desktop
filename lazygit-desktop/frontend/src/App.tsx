import { useCallback, useEffect, useState } from "react";
import { api, isDesktop, onRepoDropFailed, onRepoDropped } from "./api";
import { isSoundEnabled, setSoundEnabled, sfx } from "./sound";
import type {
  CommitFileDTO,
  FilePatch,
  RepoFileDTO,
  RepoSnapshot,
  SidebarTab,
} from "./types";
import { TopBar } from "./components/TopBar";
import { Sidebar } from "./components/Sidebar";
import { DiffPanel } from "./components/DiffPanel";
import { CommitPanel } from "./components/CommitPanel";
import { HistoryPanel } from "./components/HistoryPanel";
import { Welcome } from "./components/Welcome";
import { TitleBar } from "./components/TitleBar";
import { Operations } from "./components/Operations";
import { PublishDialog } from "./components/PublishDialog";
import { FileViewer } from "./components/FileViewer";

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
  // 提交模式下：这次提交涉及的文件，以及当前正在查看的那个
  const [commitFiles, setCommitFiles] = useState<CommitFileDTO[]>([]);
  const [activeCommitFile, setActiveCommitFile] = useState<string | null>(null);
  // 文件模式下的结构化 patch（行级暂存用）
  const [filePatch, setFilePatch] = useState<FilePatch | null>(null);
  // 当前勾选的行（索引）
  const [selectedLines, setSelectedLines] = useState<Set<number>>(new Set());
  // 完整仓库文件树 + 当前正在浏览的文件
  const [repoFiles, setRepoFiles] = useState<RepoFileDTO[]>([]);
  const [selectedRepoFile, setSelectedRepoFile] = useState<string | null>(null);
  // 拖拽时的遮罩
  const [dragging, setDragging] = useState(false);

  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");

  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  // 是否打开「Git 操作」面板（全命令入口）
  const [showOps, setShowOps] = useState(false);
  // 是否打开「上传到托管平台」对话框
  const [showPublish, setShowPublish] = useState(false);
  // 音效开关
  const [sound, setSound] = useState(isSoundEnabled());

  // 完整仓库文件树。凡是拿到新快照的地方都要跟着刷新它，
  // 所以直接挂在 run() 里，而不是靠 useEffect 的依赖变化去猜。
  const reloadRepoFiles = useCallback(async () => {
    try {
      setRepoFiles(await api.repoFiles());
    } catch (e) {
      setRepoFiles([]);
      setError("读取文件列表失败：" + (e instanceof Error ? e.message : String(e)));
    }
  }, []);

  /** 统一处理一次“动作 → 新快照”的往返，并维护忙碌态与错误提示。 */
  const run = useCallback(
    async (label: string, fn: () => Promise<RepoSnapshot | null>) => {
      setBusy(label);
      setError(null);
      try {
        const next = await fn();
        if (next) {
          setSnapshot(next);
          // 文件集合可能变了（提交、丢弃、切换分支…），跟着刷新
          void reloadRepoFiles();
        }
        playFor(label);
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
        sfx.error();
      } finally {
        setBusy(null);
      }
    },
    [reloadRepoFiles],
  );

  // 按操作语义挑音效：同一个 run() 入口，不同操作给不同反馈
  const playFor = (label: string) => {
    switch (label) {
      case "暂存":
      case "暂存全部":
        sfx.stage();
        break;
      case "取消暂存":
      case "取消全部暂存":
        sfx.unstage();
        break;
      case "提交":
        sfx.commit();
        break;
      case "Push":
      case "Pull":
      case "Fetch":
      case "上传":
        sfx.sync();
        break;
      case "丢弃改动":
        sfx.danger();
        break;
      default:
        sfx.success();
    }
  };

  const refresh = useCallback(async () => {
    await run("刷新", () => api.snapshot());
  }, [run]);

  useEffect(() => {
    void refresh();
    // 只在挂载时加载一次
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 选中某个提交时，先取「这次提交改了哪些文件」，
  // 并默认选中第一个，这样用户点开提交就能直接看到内容。
  useEffect(() => {
    let cancelled = false;

    if (!selectedCommit) {
      setCommitFiles([]);
      setActiveCommitFile(null);
      setFilePatch(null);
      setSelectedLines(new Set());
      return;
    }

    setDiffLoading(true);
    setActiveCommitFile(null);
    setDiff("");
    void api
      .commitFiles(selectedCommit)
      .then((files) => {
        if (cancelled) return;
        setCommitFiles(files);
        // 默认打开第一个文件
        if (files.length > 0) setActiveCommitFile(files[0].path);
        else setDiffLoading(false);
      })
      .catch((e) => {
        if (cancelled) return;
        setCommitFiles([]);
        setDiffLoading(false);
        setError(e instanceof Error ? e.message : String(e));
      });

    return () => {
      cancelled = true;
    };
  }, [selectedCommit]);

  // 拉取当前应该显示的 diff：
  //   - 工作区/暂存区模式：选中文件的 diff
  //   - 提交模式：本次提交里「选中的那个文件」的 diff
  useEffect(() => {
    let cancelled = false;

    async function load() {
      if (selectedCommit) {
        if (!activeCommitFile) return;
        setDiffLoading(true);
        try {
          const text = await api.commitFileDiff(selectedCommit, activeCommitFile);
          if (!cancelled) setDiff(text);
        } catch (e) {
          if (!cancelled) {
            setDiff("");
            setError(e instanceof Error ? e.message : String(e));
          }
        } finally {
          if (!cancelled) setDiffLoading(false);
        }
        return;
      }

      if (!selectedPath) {
        setDiff("");
        return;
      }
      setDiffLoading(true);
      setSelectedLines(new Set());
      try {
        // 同时取结构化 patch（供行级暂存）和原始 diff
        const [text, patch] = await Promise.all([
          api.fileDiff(selectedPath, selectedStaged),
          api.filePatchLines(selectedPath, selectedStaged),
        ]);
        if (!cancelled) {
          setDiff(text);
          setFilePatch(patch);
        }
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
  }, [selectedPath, selectedStaged, selectedCommit, activeCommitFile]);

  // 仓库变了（比如切换仓库）后，之前的选中项可能已经不存在，清掉更安全
  useEffect(() => {
    setSelectedPath(null);
    setSelectedCommit(null);
    setSelectedRepoFile(null);
  }, [snapshot?.repoPath]);

  // 把文件夹拖进窗口即可打开仓库
  useEffect(() => {
    const offDrop = onRepoDropped((dir) => {
      setDragging(false);
      void run("打开拖入的仓库", async () => {
        const snap = await api.openRepo(dir);
        setTab("files");
        return snap;
      });
    });
    const offFail = onRepoDropFailed((_path, reason) => {
      setDragging(false);
      setError(reason);
      sfx.error();
    });
    return () => {
      offDrop();
      offFail();
    };
  }, [run]);

  // 拖拽的可视反馈（Wails 负责实际接收，这里只做提示）
  useEffect(() => {
    const onEnter = (e: DragEvent) => {
      e.preventDefault();
      setDragging(true);
    };
    const onOver = (e: DragEvent) => e.preventDefault();
    const onLeave = (e: DragEvent) => {
      if (e.relatedTarget === null) setDragging(false);
    };
    // 兜底：按 Esc 取消拖拽、或把文件丢到窗口外时，dragleave 不一定触发，
    // 不清理的话遮罩会一直盖在界面上
    const onEnd = () => setDragging(false);
    window.addEventListener("dragenter", onEnter);
    window.addEventListener("dragover", onOver);
    window.addEventListener("dragleave", onLeave);
    window.addEventListener("dragend", onEnd);
    window.addEventListener("drop", onEnd);
    window.addEventListener("blur", onEnd);
    return () => {
      window.removeEventListener("dragenter", onEnter);
      window.removeEventListener("dragover", onOver);
      window.removeEventListener("dragleave", onLeave);
      window.removeEventListener("dragend", onEnd);
      window.removeEventListener("drop", onEnd);
      window.removeEventListener("blur", onEnd);
    };
  }, []);

  // 没有打开任何仓库时，显示启动界面：
  // 新建仓库 / 打开本地仓库 / 从网址下载仓库 三个入口都在那里。
  if (!snapshot) {
    return (
      <div className="app" data-drop-target>
        <TitleBar />
        <Welcome onOpened={setSnapshot} />
      </div>
    );
  }

  const stagedCount = snapshot.files.filter((f) => f.isStaged).length;

  const selectFile = (path: string, staged: boolean) => {
    sfx.select();
    setSelectedCommit(null);
    setSelectedPath(path);
    setSelectedStaged(staged);
  };

  const selectCommit = (hash: string) => {
    sfx.select();
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
    <div className="app" data-drop-target>
      <TitleBar repoName={snapshot.repoName} branch={snapshot.branch} />

      <TopBar
        snapshot={snapshot}
        busy={busy}
        onFetch={() => void run("Fetch", () => api.fetch())}
        onPull={() => void run("Pull", () => api.pull())}
        onPush={() => void run("Push", () => api.push())}
        onRefresh={() => void refresh()}
        onOpenRepo={handleOpenRepo}
        onHome={() => setSnapshot(null)}
        onOperations={() => {
          sfx.open();
          setShowOps(true);
        }}
        soundOn={sound}
        onToggleSound={() => {
          const next = !sound;
          setSound(next);
          setSoundEnabled(next);
        }}
        onPublish={() => {
          sfx.open();
          setShowPublish(true);
        }}
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
          onCreateBranch={(name, start, checkout) =>
            void run("新建分支", () => api.createBranchFrom(name, start, checkout))
          }
          onDeleteBranch={(name) => {
            const ok = window.confirm(
              `确定删除分支「${name}」吗？\n\n未合并的分支会拒绝删除。`,
            );
            if (!ok) return;
            void run("删除分支", () => api.deleteBranch(name, false));
          }}
          repoFiles={repoFiles}
          selectedRepoFile={selectedRepoFile}
          onSelectRepoFile={(p) => {
            sfx.select();
            setSelectedRepoFile(p);
          }}
        />

        {selectedRepoFile ? (
          <FileViewer
            path={selectedRepoFile}
            onClose={() => setSelectedRepoFile(null)}
          />
        ) : (
        <DiffPanel
          mode={selectedCommit ? "commit" : "file"}
          path={selectedPath}
          hash={selectedCommit}
          staged={selectedStaged}
          diff={diff}
          loading={diffLoading}
          busy={busy}
          commitFiles={commitFiles}
          activeCommitFile={activeCommitFile}
          onSelectCommitFile={setActiveCommitFile}
          filePatch={filePatch}
          selectedLines={selectedLines}
          onToggleLine={(idx) =>
            setSelectedLines((prev) => {
              const next = new Set(prev);
              if (next.has(idx)) next.delete(idx);
              else next.add(idx);
              return next;
            })
          }
          onClearLines={() => setSelectedLines(new Set())}
          onSelectHunk={(indices) =>
            setSelectedLines((prev) => {
              const next = new Set(prev);
              const allIn = indices.every((i) => next.has(i));
              for (const i of indices) {
                if (allIn) next.delete(i);
                else next.add(i);
              }
              return next;
            })
          }
          onStageLines={() => {
            if (!selectedPath || selectedLines.size === 0) return;
            void run("暂存选中行", async () => {
              const next = await api.stageLines(
                selectedPath,
                selectedStaged,
                [...selectedLines],
              );
              return next;
            });
          }}
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
        )}

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

      {dragging && (
        <div className="drop-overlay">
          <div className="drop-card">
            <div className="drop-icon">📂</div>
            <div className="drop-title">松开即可打开这个仓库</div>
            <div className="drop-hint">
              把文件夹（或仓库里的任意文件/子目录）拖进来都可以
            </div>
          </div>
        </div>
      )}

      {showPublish && (
        <PublishDialog
          snapshot={snapshot}
          onClose={() => {
            sfx.close();
            setShowPublish(false);
          }}
          onPublished={(r) => {
            if (r.snapshot) setSnapshot(r.snapshot);
          }}
        />
      )}

      {showOps && (
        <Operations
          onClose={() => {
            sfx.close();
            setShowOps(false);
          }}
          onSnapshot={(r) => {
            // 操作改变了仓库状态，用引擎回传的新快照刷新界面
            if (r.snapshot) setSnapshot(r.snapshot);
          }}
        />
      )}
    </div>
  );
}
