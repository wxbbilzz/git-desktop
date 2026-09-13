import type { RepoSnapshot } from "../types";
import { PillButton } from "./PillButton";
import {
  IconAlert,
  IconBranch,
  IconCheck,
  IconCommit,
  IconFetch,
  IconFolder,
  IconHome,
  IconPull,
  IconPush,
  IconRefresh,
  IconSearch,
  IconTrash,
  IconUndo,
  IconUpload,
  IconVolumeOn,
  IconVolumeOff,
} from "./icons";

interface Props {
  snapshot: RepoSnapshot;
  busy: string | null;
  onFetch: () => void;
  onPull: () => void;
  onPush: () => void;
  onRefresh: () => void;
  onOpenRepo: () => void;
  onHome: () => void;
  onOperations: () => void;
  onPublish: () => void;
  soundOn: boolean;
  onToggleSound: () => void;
  // 撤销上一步 / 回收站 / 命令面板
  onUndo: () => void;
  onShowTrash: () => void;
  trashCount: number;
  onPalette: () => void;
  // 处于变基 / 合并等被中断的状态时，这两个按钮才有意义
  onContinue: () => void;
  onAbort: () => void;
}

export function TopBar({
  snapshot,
  busy,
  onFetch,
  onPull,
  onPush,
  onRefresh,
  onOpenRepo,
  onHome,
  onOperations,
  onPublish,
  soundOn,
  onToggleSound,
  onUndo,
  onShowTrash,
  trashCount,
  onPalette,
  onContinue,
  onAbort,
}: Props) {
  const disabled = busy !== null;
  const conflicted = snapshot.state !== "";

  return (
    <header className="topbar">
      <div className="brand">
        <span className="brand-dot" />
        <span className="repo-name">{snapshot.repoName || "Lazygit Desktop"}</span>
      </div>

      <span className="repo-path" title={snapshot.repoPath}>
        {snapshot.repoPath}
      </span>

      {snapshot.branch && (
        <span className="chip accent">
          <IconBranch />
          {snapshot.branch}
          {snapshot.isDetached ? " · 游离 HEAD" : ""}
        </span>
      )}

      {snapshot.state && <span className="chip warn">{snapshot.state}</span>}

      {/* 卡在变基 / 合并冲突里的时候，最要紧的就是「继续」和「中止」这两个出口，
          所以把它们放在顶栏最显眼的位置，而不是藏在命令目录里 */}
      {conflicted && (
        <>
          <PillButton
            variant="success"
            icon={<IconCheck />}
            onClick={onContinue}
            disabled={disabled}
            title="解决冲突并暂存后，继续被中断的操作"
          >
            继续
          </PillButton>
          <PillButton
            variant="danger"
            icon={<IconAlert />}
            onClick={onAbort}
            disabled={disabled}
            title="放弃这次操作，回到它开始之前的状态"
          >
            中止
          </PillButton>
        </>
      )}

      <span className="spacer" />

      <PillButton
        icon={<IconSearch />}
        onClick={onPalette}
        disabled={disabled}
        title="命令面板（Ctrl/⌘ + K）"
      />

      <PillButton
        icon={<IconUndo />}
        onClick={onUndo}
        disabled={disabled || !snapshot.canUndo}
        title={snapshot.canUndo ? snapshot.undoHint : "没有可撤销的操作"}
      />

      {trashCount > 0 && (
        <PillButton
          icon={<IconTrash />}
          onClick={onShowTrash}
          title={`回收站里有 ${trashCount} 个丢弃过的文件`}
        >
          {trashCount}
        </PillButton>
      )}

      <PillButton
        icon={<IconHome />}
        onClick={onHome}
        disabled={disabled}
        title="回到启动页"
      />
      <PillButton
        icon={<IconFolder />}
        onClick={onOpenRepo}
        disabled={disabled}
        title="打开本地仓库"
      />
      <PillButton
        variant="success"
        icon={<IconCommit />}
        onClick={onOperations}
        disabled={disabled}
        title="打开 git 操作面板（全部命令）"
      >
        Git 操作
      </PillButton>
      <PillButton icon={<IconFetch />} onClick={onFetch} disabled={disabled}>
        Fetch
      </PillButton>
      <PillButton icon={<IconPull />} onClick={onPull} disabled={disabled}>
        Pull
      </PillButton>
      <PillButton
        icon={<IconUpload />}
        onClick={onPublish}
        disabled={disabled}
        title="上传到 GitHub / Gitee"
      >
        上传
      </PillButton>
      <PillButton
        variant="primary"
        icon={<IconPush />}
        onClick={onPush}
        disabled={disabled}
      >
        Push
      </PillButton>
      <PillButton
        icon={soundOn ? <IconVolumeOn /> : <IconVolumeOff />}
        onClick={onToggleSound}
        title={soundOn ? "关闭音效" : "开启音效"}
      />
      <PillButton
        icon={<IconRefresh />}
        onClick={onRefresh}
        disabled={disabled}
        title="刷新"
      />
    </header>
  );
}
