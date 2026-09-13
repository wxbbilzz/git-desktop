import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import {
  api,
  isDesktop,
  onRepoChanged,
  onRepoDropFailed,
  onRepoDropped,
  onSyncProgress,
} from "./api";
import { isSoundEnabled, setSoundEnabled, sfx } from "./sound";
import type {
  CloneProgress,
  CommitFileDTO,
  FilePatch,
  FolderInfo,
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
import { InitRepoPrompt } from "./components/InitRepoPrompt";
import { ConfirmDialog } from "./components/ConfirmDialog";
import type { ConfirmSpec } from "./components/ConfirmDialog";
import { ContextMenu } from "./components/ContextMenu";
import type { MenuSpec } from "./components/ContextMenu";
import { CommandPalette } from "./components/CommandPalette";
import type { Command } from "./components/CommandPalette";
import { DiscardTrash } from "./components/DiscardTrash";
import {
  IconArchive,
  IconBranch,
  IconCherry,
  IconCloud,
  IconCommit,
  IconFetch,
  IconFolder,
  IconMerge,
  IconPull,
  IconPush,
  IconRefresh,
  IconRewind,
  IconSearch,
  IconTag,
  IconTrash,
  IconUndo,
  IconUpload,
} from "./components/icons";

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
  // 打开了一个「还不是仓库」的文件夹时，先弹窗问要不要初始化
  const [pendingFolder, setPendingFolder] = useState<FolderInfo | null>(null);
  // 远端操作的实时进度（push/pull/fetch）
  const [syncProgress, setSyncProgress] = useState<CloneProgress | null>(null);

  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");

  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  // 一闪而过的提示（复制成功之类），1.6 秒后自动消失
  const [toast, setToast] = useState<string | null>(null);
  // 是否打开「Git 操作」面板（全命令入口）
  const [showOps, setShowOps] = useState(false);
  // 是否打开「上传到托管平台」对话框
  const [showPublish, setShowPublish] = useState(false);
  // 音效开关
  const [sound, setSound] = useState(isSoundEnabled());

  // 自绘确认框 / 右键菜单 / 命令面板 / 回收站
  const [confirmSpec, setConfirmSpec] = useState<ConfirmSpec | null>(null);
  const [menuSpec, setMenuSpec] = useState<MenuSpec | null>(null);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [showTrash, setShowTrash] = useState(false);
  const [trashCount, setTrashCount] = useState(0);

  // 从历史面板跳到侧栏做某件事时，把参数带过去
  const [branchStart, setBranchStart] = useState<string | null>(null);
  const [tagRef, setTagRef] = useState<string | null>(null);

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
        setSyncProgress(null);
        // 丢弃类操作会往回收站里加东西，顺手更新计数
        void api
          .discardedFiles()
          .then((r) => setTrashCount(r.length))
          .catch(() => {});
      }
    },
    [reloadRepoFiles],
  );

  // 打开一个文件夹。它会先看看这个目录到底是什么情况：
  //   · 本身就是仓库          -> 直接打开
  //   · 是某个仓库的子目录    -> 打开那个仓库，并提示一下
  //   · 完全不受 git 管理     -> 弹窗问「要不要在这里建仓库」
  const openFolder = useCallback(
    async (path: string) => {
      try {
        const info = await api.inspectFolder(path);

        if (info.isRepo) {
          await run("打开仓库", () => api.openRepo(path));
          return;
        }
        if (info.parentRepo) {
          // 拖进来的是子目录：打开它所属的仓库更符合预期
          await run("打开仓库", async () => {
            const snap = await api.openRepo(info.parentRepo);
            setSelectedPath(null);
            return snap;
          });
          setError(`「${path}」在仓库 ${info.parentRepo} 里面，已打开上级仓库。`);
          return;
        }
        setPendingFolder(info);
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      }
    },
    [run],
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
      case "修补提交":
        sfx.commit();
        break;
      case "Push":
      case "Pull":
      case "Fetch":
      case "上传":
      case "推送标签":
        sfx.sync();
        break;
      case "丢弃改动":
      case "回退":
      case "删除分支":
        sfx.danger();
        break;
      default:
        sfx.success();
    }
  };

  const refresh = useCallback(async () => {
    await run("刷新", () => api.snapshot());
  }, [run]);

  /** 静默刷新：自动刷新用它，不显示忙碌条（否则每次外部改动都闪一下）。 */
  const refreshQuiet = useCallback(async () => {
    try {
      const next = await api.snapshot();
      setSnapshot(next);
      void reloadRepoFiles();
    } catch {
      // 自动刷新失败不打扰用户：可能仓库正在被 git 命令改动的中间状态
    }
  }, [reloadRepoFiles]);

  // busy 的最新值放进 ref：自动刷新的回调是长期订阅的，
  // 不能靠闭包捕获 busy（那样永远读到第一次渲染的值）
  const busyRef = useRef(busy);
  busyRef.current = busy;

  // 订阅远端操作进度。推送大仓库时能看到「上传对象 45%」，
  // 而不是盯着一个转圈等着。
  useEffect(() => {
    return onSyncProgress((p) => setSyncProgress(p));
  }, []);

  // 订阅「仓库被外部改动」。
  //
  // 这就是自动刷新：在编辑器里改完文件、在终端里跑完 git 命令，
  // 切回来时列表已经是新的。自己触发的操作会走 run() 那条路，
  // 所以这里跳过正在忙碌的时刻，避免重复刷新。
  useEffect(() => {
    return onRepoChanged(() => {
      if (busyRef.current !== null) return;
      void refreshQuiet();
    });
  }, [refreshQuiet]);

  // 提示条自动消失
  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 1600);
    return () => clearTimeout(t);
  }, [toast]);

  // 启动时如果所在目录不是 git 仓库，直接问「要不要在这里建仓库」。
  // 这样 `cd 某个项目 && bingit` 就能一步到位。
  useEffect(() => {
    void (async () => {
      try {
        const info = await api.pendingStartupFolder();
        if (info) setPendingFolder(info);
      } catch {
        /* 忽略 */
      }
    })();
  }, []);

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
      // 可能是仓库、仓库的子目录、或者一个普通文件夹 —— 交给 openFolder 判断
      void openFolder(dir);
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
  }, [openFolder]);

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

  // ---------------------------------------------------------------- 小工具

  const copy = useCallback(async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setToast(`已复制${label}`);
    } catch {
      setError("复制失败：剪贴板不可用");
    }
  }, []);

  const askConfirm = useCallback((spec: ConfirmSpec) => {
    setConfirmSpec(spec);
  }, []);

  // ---------------------------------------------------------------- 动作
  //
  // 会改历史 / 丢内容的操作都先过一遍自绘确认框，并且把即将执行的命令
  // 显示出来。这样「硬回退」「删除分支」这类动作点下去之前心里有底。

  const doDiscard = useCallback(
    (path: string) => {
      askConfirm({
        title: "丢弃改动",
        body: `确定丢弃「${path}」上的改动吗？`,
        command: `git checkout -- ${path}`,
        note: "内容会先存进回收站，之后还能从这里恢复。",
        confirmLabel: "丢弃",
        danger: true,
        onConfirm: () => {
          setConfirmSpec(null);
          void run("丢弃改动", () => api.discardFile(path));
        },
      });
    },
    [askConfirm, run],
  );

  const doDeleteBranch = useCallback(
    (name: string) => {
      askConfirm({
        title: "删除分支",
        body: `确定删除本地分支「${name}」吗？`,
        command: `git branch -d ${name}`,
        note: "还没合并进其他分支的提交会被 git 拒绝删除；确认要删可以改用强制删除。",
        confirmLabel: "删除",
        danger: true,
        onConfirm: () => {
          setConfirmSpec(null);
          void run("删除分支", () => api.deleteBranch(name, false));
        },
      });
    },
    [askConfirm, run],
  );

  const doMerge = useCallback(
    (name: string) => {
      askConfirm({
        title: "合并分支",
        body: `把「${name}」合并进当前分支（${snapshot?.branch ?? "?"}）？`,
        command: `git merge --no-edit ${name}`,
        note: "如果两边改了同一个地方会停下等你解决冲突，届时顶部会出现「继续 / 中止」。",
        confirmLabel: "合并",
        onConfirm: () => {
          setConfirmSpec(null);
          void run("合并", () => api.mergeBranch(name, false, false));
        },
      });
    },
    [askConfirm, run, snapshot?.branch],
  );

  const doRebase = useCallback(
    (name: string) => {
      askConfirm({
        title: "变基",
        body: `把当前分支的提交重新应用到「${name}」之上？`,
        command: `git rebase ${name}`,
        note: "这会改写当前分支的提交记录。已经推送过的分支变基后需要强制推送。",
        confirmLabel: "变基",
        onConfirm: () => {
          setConfirmSpec(null);
          void run("变基", () => api.rebaseOnto(name));
        },
      });
    },
    [askConfirm, run],
  );

  const doCherryPick = useCallback(
    (hash: string) => {
      askConfirm({
        title: "拣选提交",
        body: `把提交 ${hash.slice(0, 8)} 的改动应用到当前分支？`,
        command: `git cherry-pick ${hash.slice(0, 8)}`,
        confirmLabel: "拣选",
        onConfirm: () => {
          setConfirmSpec(null);
          void run("拣选", () => api.cherryPick(hash));
        },
      });
    },
    [askConfirm, run],
  );

  const doRevert = useCallback(
    (hash: string) => {
      askConfirm({
        title: "撤销这次提交",
        body: `生成一个反向提交来抵消 ${hash.slice(0, 8)} 的改动？`,
        command: `git revert --no-edit ${hash.slice(0, 8)}`,
        note: "不会改写历史，适合已经推送出去的提交。",
        confirmLabel: "生成反向提交",
        onConfirm: () => {
          setConfirmSpec(null);
          void run("撤销提交", () => api.revertCommit(hash));
        },
      });
    },
    [askConfirm, run],
  );

  const doResetTo = useCallback(
    (hash: string, mode: "soft" | "mixed" | "hard") => {
      const hard = mode === "hard";
      askConfirm({
        title: hard ? "硬回退（会丢弃改动）" : "回退到这次提交",
        body: `把当前分支回退到 ${hash.slice(0, 8)}？`,
        command: `git reset --${mode} ${hash.slice(0, 8)}`,
        note: hard
          ? "工作区和暂存区里所有未提交的改动都会消失。回退之后仍然可以用「撤销」把指针移回来，但文件内容不会回来。"
          : mode === "soft"
            ? "改动会完整保留在暂存区，只是提交记录退回去了。"
            : "改动会保留在工作区，但会取消暂存。",
        confirmLabel: hard ? "硬回退" : "回退",
        danger: hard,
        onConfirm: () => {
          setConfirmSpec(null);
          void run("回退", () => api.resetTo(hash, mode));
        },
      });
    },
    [askConfirm, run],
  );

  const doDeleteTag = useCallback(
    (name: string) => {
      askConfirm({
        title: "删除标签",
        body: `删除本地标签「${name}」？`,
        command: `git tag -d ${name}`,
        note: "只删本地这一个。远端上的同名标签需要另外推送删除。",
        confirmLabel: "删除",
        danger: true,
        onConfirm: () => {
          setConfirmSpec(null);
          void run("删除标签", () => api.deleteTag(name));
        },
      });
    },
    [askConfirm, run],
  );

  const doRemoveRemote = useCallback(
    (name: string) => {
      askConfirm({
        title: "删除远端",
        body: `删除远端「${name}」？`,
        command: `git remote remove ${name}`,
        note: "只影响本地配置，远端仓库本身不动。它对应的远端分支引用也会一起删掉。",
        confirmLabel: "删除",
        danger: true,
        onConfirm: () => {
          setConfirmSpec(null);
          void run("删除远端", () => api.removeRemote(name));
        },
      });
    },
    [askConfirm, run],
  );

  const doAbort = useCallback(() => {
    const state = snapshot?.state ?? "";
    askConfirm({
      title: "中止操作",
      body: `放弃当前${state}的操作，回到它开始之前的状态？`,
      command: "git rebase --abort / git merge --abort",
      note: "中止后会回到操作前的提交和文件状态。",
      confirmLabel: "中止",
      danger: true,
      onConfirm: () => {
        setConfirmSpec(null);
        void run("中止操作", () => api.abortOperation());
      },
    });
  }, [askConfirm, run, snapshot?.state]);

  const doUndo = useCallback(() => {
    if (!snapshot?.canUndo) return;
    askConfirm({
      title: "撤销上一步",
      body: snapshot.undoHint || "撤销上一步操作？",
      command: "git reset --soft HEAD@{1}",
      note: "只把分支指针移回上一个位置，工作区内容一点都不会丢，暂存区里的改动会回到工作区。",
      confirmLabel: "撤销",
      onConfirm: () => {
        setConfirmSpec(null);
        void run("撤销", () => api.undoLast());
      },
    });
  }, [askConfirm, run, snapshot?.canUndo, snapshot?.undoHint]);

  // ---------------------------------------------------------------- 快捷键

  // 正在输入框里打字时不抢按键
  const isTyping = () => {
    const el = document.activeElement as HTMLElement | null;
    if (!el) return false;
    const tag = el.tagName.toLowerCase();
    return tag === "input" || tag === "textarea" || el.isContentEditable;
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const mod = e.ctrlKey || e.metaKey;

      // 命令面板：任何地方都能唤起，包括正在输入框里打字时。
      //
      // 但不能叠在别的浮层上 —— 之前这里写在「有浮层就返回」的守卫之前，
      // 于是在确认框开着的时候按 Ctrl+K，会又叠出一个命令面板（两层浮层）。
      // 现在的规则是：面板已开 -> 关掉；别的浮层开着 -> 不响应；否则打开。
      if (mod && e.key.toLowerCase() === "k") {
        e.preventDefault();
        if (paletteOpen) {
          setPaletteOpen(false);
          return;
        }
        if (confirmSpec || menuSpec || showPublish || showOps) return;
        setPaletteOpen(true);
        return;
      }

      // 其余浮层打开时，按键交给浮层自己处理
      if (paletteOpen || confirmSpec || menuSpec || showPublish || showOps) return;
      if (isTyping()) return;

      if (mod && e.key.toLowerCase() === "z") {
        e.preventDefault();
        doUndo();
        return;
      }
      if (mod && e.key.toLowerCase() === "r") {
        e.preventDefault();
        void refresh();
        return;
      }
      if (e.key === "Escape") {
        setSelectedPath(null);
        setSelectedCommit(null);
        return;
      }
      if (!snapshot) return;

      // 空格：暂存 / 取消暂存当前选中的文件（git GUI 里最顺手的那个键）
      if (e.key === " " && selectedPath) {
        e.preventDefault();
        if (selectedStaged) {
          void run("取消暂存", () => api.unstageFile(selectedPath));
        } else {
          void run("暂存", () => api.stageFile(selectedPath));
        }
        return;
      }

      // 上下键在文件列表里移动选中项。
      // 只对文件列表生效 —— 在分支列表上按方向键就切分支太危险了。
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        const delta = e.key === "ArrowDown" ? 1 : -1;
        if (tab === "changes" || tab === "staged") {
          const staged = tab === "staged";
          const list = snapshot.files
            .filter((f) => f.isStaged === staged)
            .map((f) => f.path);
          move(list, delta, selectedPath, (p) => selectFile(p, staged));
        }
        return;
      }
    };

    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    snapshot,
    tab,
    selectedPath,
    selectedStaged,
    paletteOpen,
    confirmSpec,
    menuSpec,
    showPublish,
    showOps,
    doUndo,
    refresh,
    run,
  ]);

  /** 在列表里按 delta 移动一位并选中。 */
  function move<T>(
    list: T[],
    delta: number,
    current: T | null,
    select: (item: T) => void,
  ) {
    if (list.length === 0) return;
    const i = current === null ? -1 : list.indexOf(current);
    const next = i === -1 ? (delta > 0 ? 0 : list.length - 1) : i + delta;
    if (next < 0 || next >= list.length) return;
    select(list[next]);
  }

  // ---------------------------------------------------------------- 命令面板

  const commands = useMemo<Command[]>(() => {
    if (!snapshot) return [];
    const cmds: Command[] = [];

    const add = (
      group: string,
      label: string,
      icon: ReactNode,
      run: () => void,
      hint?: string,
    ) => cmds.push({ id: group + ":" + label, group, label, icon, run, hint });

    // 同步
    add("同步", "抓取远端更新（Fetch）", <IconFetch />, () =>
      void run("Fetch", () => api.fetch()),
    );
    add("同步", "拉取并合并（Pull）", <IconPull />, () =>
      void run("Pull", () => api.pull()),
    );
    add("同步", "推送到远端（Push）", <IconPush />, () =>
      void run("Push", () => api.push()),
    );
    add("同步", "上传到 GitHub / Gitee", <IconUpload />, () => setShowPublish(true));

    // 提交与撤销
    add("提交", "修补上一次提交（把暂存的改动合进去）", <IconCommit />, () =>
      void run("修补提交", () => api.amendCommit(summary, description)),
      "Ctrl+Enter 提交",
    );
    if (snapshot.canUndo) {
      add("提交", snapshot.undoHint || "撤销上一步", <IconUndo />, doUndo, "Ctrl+Z");
    }
    add("提交", "丢弃过的文件（回收站）", <IconTrash />, () => setShowTrash(true));

    // 分支
    add("分支", "新建分支…", <IconBranch />, () => {
      setTab("branches");
      setBranchStart("");
    });
    for (const b of snapshot.branches) {
      if (!b.isHead) {
        add("分支", `切换到 ${b.name}`, <IconBranch />, () =>
          void run("切换分支", () => api.checkoutBranch(b.name)),
        );
        add("分支", `把 ${b.name} 合并进当前分支`, <IconMerge />, () => doMerge(b.name));
        add("分支", `把当前分支变基到 ${b.name}`, <IconCherry />, () => doRebase(b.name));
      }
    }
    for (const rb of snapshot.remoteBranches) {
      add(
        "分支",
        rb.hasLocal
          ? `切换到本地分支 ${rb.short}`
          : `检出远端分支 ${rb.name}`,
        <IconCloud />,
        () =>
          void run("检出分支", () =>
            api.checkoutRemoteBranch(rb.name, rb.hasLocal ? rb.short : ""),
          ),
      );
    }

    // 标签
    add("标签", "新建标签…", <IconTag />, () => {
      setTab("tags");
      setTagRef(null);
    });
    for (const t of snapshot.tags) {
      add("标签", `推送标签 ${t.name}`, <IconUpload />, () =>
        void run("推送标签", () =>
          api.pushTag(t.name, snapshot.remotes[0]?.name || "origin"),
        ),
      );
    }

    // 储藏
    add("储藏", "储藏当前改动…", <IconArchive />, () => setTab("stashes"));

    // 提交历史里最近的那些提交
    for (const c of snapshot.commits.slice(0, 20)) {
      const label = `${c.shortHash} ${c.subject}`;
      add("提交历史", `拣选：${label}`, <IconCherry />, () => doCherryPick(c.hash));
      add("提交历史", `回退到这里（保留改动）：${label}`, <IconRewind />, () =>
        doResetTo(c.hash, "soft"),
      );
    }

    // 其它入口
    add("其它", "打开 git 操作面板（全部命令）", <IconSearch />, () =>
      setShowOps(true),
    );
    add("其它", "打开本地仓库…", <IconFolder />, () =>
      void run("打开仓库", () => api.chooseAndOpenRepo()),
    );
    add("其它", "刷新", <IconRefresh />, () => void refresh(), "Ctrl+R");

    return cmds;
  }, [
    snapshot,
    run,
    refresh,
    doUndo,
    doMerge,
    doRebase,
    doCherryPick,
    doResetTo,
    summary,
    description,
  ]);

  // 回收站计数：打开时同步一次，之后每次操作结束也会更新
  useEffect(() => {
    void api
      .discardedFiles()
      .then((r) => setTrashCount(r.length))
      .catch(() => {});
  }, [snapshot?.repoPath]);

  // ---------------------------------------------------------------- 渲染

  // 询问框要在「欢迎页」和「主界面」两种状态下都能显示。
  // 之前只写在主界面的 return 里，导致没打开仓库时（也就是最该问的情况）
  // setPendingFolder 执行了但界面什么都不弹 —— 看起来像"没反应"。
  const initPrompt = pendingFolder ? (
    <InitRepoPrompt
      info={pendingFolder}
      busy={busy !== null}
      onCancel={() => setPendingFolder(null)}
      onConfirm={(branch) => {
        const dir = pendingFolder.path;
        setPendingFolder(null);
        void run("建立仓库", async () => {
          const snap = await api.initRepoHere(dir, branch);
          setTab("files");
          return snap;
        });
      }}
    />
  ) : null;

  // 没有打开任何仓库时，显示启动界面：
  // 新建仓库 / 打开本地仓库 / 从网址下载仓库 三个入口都在那里。
  if (!snapshot) {
    return (
      <div className="app" data-drop-target>
        <TitleBar />
        <Welcome onOpened={setSnapshot} onOpenFolder={(p) => void openFolder(p)} />
        {initPrompt}
        {confirmSpec && (
          <ConfirmDialog {...confirmSpec} onCancel={() => setConfirmSpec(null)} />
        )}
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

  const handleCommit = () => {
    if (summary.trim().length === 0 || stagedCount === 0) return;
    void run("提交", async () => {
      const next = await api.commit(summary, description);
      setSummary("");
      setDescription("");
      return next;
    });
  };

  const handleAmend = () => {
    void run("修补提交", async () => {
      const next = await api.amendCommit(summary, description);
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
        onPublish={() => {
          sfx.open();
          setShowPublish(true);
        }}
        soundOn={sound}
        onToggleSound={() => {
          const next = !sound;
          setSound(next);
          setSoundEnabled(next);
        }}
        onUndo={doUndo}
        onShowTrash={() => setShowTrash(true)}
        trashCount={trashCount}
        onPalette={() => setPaletteOpen(true)}
        onContinue={() => void run("继续操作", () => api.continueOperation())}
        onAbort={doAbort}
      />

      {(busy || error || toast || !isDesktop()) && (
        <div className={"banner" + (error ? " error" : "")}>
          {busy && <span className="spinner" />}
          {error
            ? error
            : toast
              ? toast
              : busy
                ? syncProgress && syncProgress.phase
                  ? `正在${busy}… ${syncProgress.phase}${
                      syncProgress.percent >= 0 ? ` ${syncProgress.percent}%` : ""
                    }`
                  : `正在${busy}…`
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
          onDiscard={doDiscard}
          onCheckoutBranch={(name) =>
            void run("切换分支", () => api.checkoutBranch(name))
          }
          onCreateBranch={(name, start, checkout) =>
            void run("新建分支", () => api.createBranchFrom(name, start, checkout))
          }
          onDeleteBranch={doDeleteBranch}
          onRenameBranch={(oldName, newName) =>
            void run("重命名分支", () => api.renameBranch(oldName, newName))
          }
          onMergeBranch={doMerge}
          onRebaseOnto={doRebase}
          onCheckoutRemoteBranch={(remoteBranch, local) =>
            void run("检出分支", () => api.checkoutRemoteBranch(remoteBranch, local))
          }
          onTagCreate={(name, ref, message) =>
            void run("创建标签", () => api.createTag(name, ref, message))
          }
          onTagDelete={doDeleteTag}
          onTagPush={(name, remote) =>
            void run("推送标签", () => api.pushTag(name, remote))
          }
          onTagPushAll={(remote) =>
            void run("推送标签", () => api.pushAllTags(remote))
          }
          onRemoteAdd={(name, url) =>
            void run("添加远端", () => api.addRemote(name, url))
          }
          onRemoteRemove={doRemoveRemote}
          onRemoteSetUrl={(name, url) =>
            void run("修改远端地址", () => api.setRemoteUrl(name, url))
          }
          onStashSave={(message, includeUntracked) =>
            void run("储藏", () => api.stashSave(message, includeUntracked))
          }
          onStashPop={(index) => void run("弹出储藏", () => api.stashPop(index))}
          onStashApply={(index) => void run("应用储藏", () => api.stashApply(index))}
          onStashDrop={(index) => void run("删除储藏", () => api.stashDrop(index))}
          onStashShow={(index) => api.stashShow(index)}
          onMenu={setMenuSpec}
          onCopy={(text, label) => void copy(text, label)}
          branchStart={branchStart}
          onBranchStartUsed={() => setBranchStart(null)}
          tagRef={tagRef}
          onTagRefUsed={() => setTagRef(null)}
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
              doDiscard(selectedPath);
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
            lastCommitSubject={snapshot.commits[0]?.subject ?? ""}
            onSummaryChange={setSummary}
            onDescriptionChange={setDescription}
            onCommit={handleCommit}
            onAmend={handleAmend}
            onSaveIdentity={(n, e) =>
              void run("保存身份", () => api.setIdentity(n, e, true))
            }
          />
          <HistoryPanel
            commits={snapshot.commits}
            selectedHash={selectedCommit}
            onSelect={selectCommit}
            busy={busy}
            onCherryPick={doCherryPick}
            onRevert={doRevert}
            onResetTo={doResetTo}
            onTagCommit={(hash) => {
              setTab("tags");
              setTagRef(hash);
            }}
            onCreateBranchFrom={(hash) => {
              setTab("branches");
              setBranchStart(hash);
            }}
            onMenu={setMenuSpec}
            onCopy={(text, label) => void copy(text, label)}
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

      {initPrompt}

      {confirmSpec && (
        <ConfirmDialog {...confirmSpec} onCancel={() => setConfirmSpec(null)} />
      )}

      {menuSpec && (
        <ContextMenu spec={menuSpec} onClose={() => setMenuSpec(null)} />
      )}

      {paletteOpen && (
        <CommandPalette
          commands={commands}
          onClose={() => setPaletteOpen(false)}
        />
      )}

      {showTrash && (
        <DiscardTrash
          onClose={() => {
            setShowTrash(false);
            void api
              .discardedFiles()
              .then((r) => setTrashCount(r.length))
              .catch(() => {});
            void refreshQuiet();
          }}
        />
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
          onConfirm={askConfirm}
        />
      )}
    </div>
  );
}
