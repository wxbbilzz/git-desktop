import { useMemo, useState } from "react";
import type { FileDTO } from "../types";
import { buildFileTree, type TreeDir, type TreeNode } from "../fileTree";
import { IconChevron, IconFolder } from "./icons";
import { PillButton } from "./PillButton";
import { IconMinus, IconPlus, IconTrash } from "./icons";

// 文件树视图。相比平铺列表，目录结构一目了然，也更省屏幕。

interface Props {
  files: FileDTO[];
  staged: boolean;
  selectedPath: string | null;
  selectedStaged: boolean;
  busy: string | null;
  onSelectFile: (path: string, staged: boolean) => void;
  onStage: (path: string) => void;
  onUnstage: (path: string) => void;
  onDiscard: (path: string) => void;
}

function splitName(name: string): { base: string; dir: string } {
  const i = name.lastIndexOf("/");
  if (i === -1) return { base: name, dir: "" };
  return { base: name.slice(i + 1), dir: name.slice(0, i) };
}

export function FileTree({
  files,
  staged,
  selectedPath,
  selectedStaged,
  busy,
  onSelectFile,
  onStage,
  onUnstage,
  onDiscard,
}: Props) {
  // 同 RepoFileTree：构建结果要缓存，否则每次渲染都重算
  const tree = useMemo(() => buildFileTree(files), [files]);

  // 默认全部展开：文件少的时候展开更好用，多了再手动收
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  const toggle = (path: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const renderNodes = (nodes: TreeNode[], depth: number) =>
    nodes.map((node) => {
      if (node.type === "dir") {
        return (
          <div key={node.path}>
            <button
              className="tree-dir"
              style={{ paddingLeft: 8 + depth * 14 }}
              onClick={() => toggle(node.path)}
            >
              <span className="tree-chevron">
                <IconChevron open={!collapsed.has(node.path)} />
              </span>
              <span className="tree-dir-icon">
                <IconFolder />
              </span>
              <span className="tree-dir-name">{node.name}</span>
              <span className="tree-dir-count">{countFiles(node)}</span>
            </button>
            {!collapsed.has(node.path) && renderNodes(node.children, depth + 1)}
          </div>
        );
      }

      const f = node.file;
      const selected = selectedPath === f.path && selectedStaged === staged;
      const { base, dir } = splitName(node.name);

      return (
        <div
          key={f.path}
          className={"row tree-file" + (selected ? " selected" : "")}
          style={{ paddingLeft: 8 + depth * 14 }}
          onClick={() => onSelectFile(f.path, staged)}
        >
          <span className={"dot " + f.kind} />
          <div className="row-main">
            <div className="row-name" title={f.path}>
              {base}
            </div>
            {dir && <div className="row-sub">{dir}</div>}
          </div>

          {(f.linesAdded > 0 || f.linesDeleted > 0) && (
            <span className="stat">
              <span className="add">+{f.linesAdded}</span>{" "}
              <span className="del">-{f.linesDeleted}</span>
            </span>
          )}

          <span className={"badge " + f.kind}>{f.statusLabel}</span>

          <div className="row-actions" onClick={(e) => e.stopPropagation()}>
            {staged ? (
              <PillButton
                size="sm"
                variant="ghost"
                icon={<IconMinus />}
                title="取消暂存"
                disabled={busy !== null}
                onClick={() => onUnstage(f.path)}
              />
            ) : (
              <PillButton
                size="sm"
                variant="success"
                icon={<IconPlus />}
                title="暂存"
                disabled={busy !== null}
                onClick={() => onStage(f.path)}
              />
            )}
            {!staged && (
              <PillButton
                size="sm"
                variant="danger"
                icon={<IconTrash />}
                title="丢弃改动"
                disabled={busy !== null}
                onClick={() => onDiscard(f.path)}
              />
            )}
          </div>
        </div>
      );
    });

  // 没有改动时给出明确提示，而不是留一片空白
  if (files.length === 0) {
    return (
      <div className="empty">
        <div className="empty-title">
          {staged ? "暂存区是空的" : "工作区是干净的"}
        </div>
        <div className="empty-text">
          {staged
            ? "在工作区里点 + 号把改动放进暂存区。"
            : "没有未提交的改动。想看仓库里都有什么，切到「文件」标签。"}
        </div>
      </div>
    );
  }

  return <div className="tree">{renderNodes(tree, 0)}</div>;
}

function countFiles(dir: TreeDir): number {
  let n = 0;
  for (const c of dir.children) {
    if (c.type === "file") n++;
    else n += countFiles(c);
  }
  return n;
}
