import type { RepoSnapshot } from "../types";
import { PillButton } from "./PillButton";
import {
  IconBranch,
  IconCommit,
  IconFetch,
  IconFolder,
  IconHome,
  IconPull,
  IconPush,
  IconRefresh,
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
}: Props) {
  const disabled = busy !== null;

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

      <span className="spacer" />

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
        title="打开 git 操作面板"
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
