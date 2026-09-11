import { useMemo, useState } from "react";
import type { RepoFileDTO } from "../types";
import { buildPathTree, type PathDir, type PathNode } from "../fileTree";
import { IconChevron, IconFolder } from "./icons";

// 完整仓库文件树。和「工作区」的文件树共用同一套构建算法与样式，
// 区别是这里列的是仓库里的**所有**文件，没有改动也能看到。

interface Props {
  files: RepoFileDTO[];
  selectedPath: string | null;
  onSelectFile: (path: string) => void;
}

export function RepoFileTree({ files, selectedPath, onSelectFile }: Props) {
  // 必须缓存：否则每次点选文件都会把整棵树重建一遍
  // （大仓库几千个文件时会明显卡顿）
  const tree = useMemo(() => buildPathTree(files.map((f) => f.path)), [files]);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  const toggle = (path: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });

  const render = (nodes: PathNode[], depth: number): React.ReactNode =>
    nodes.map((node) => {
      if (node.type === "dir") {
        const closed = collapsed.has(node.path);
        return (
          <div key={node.path}>
            <button
              className="tree-dir"
              style={{ paddingLeft: 8 + depth * 14 }}
              onClick={() => toggle(node.path)}
            >
              <span className="tree-chevron">
                <IconChevron open={!closed} />
              </span>
              <span className="tree-dir-icon">
                <IconFolder />
              </span>
              <span className="tree-dir-name">{node.name}</span>
              <span className="tree-dir-count">{countFiles(node)}</span>
            </button>
            {!closed && render(node.children, depth + 1)}
          </div>
        );
      }

      const base = node.name;
      return (
        <div
          key={node.path}
          className={"row tree-file" + (selectedPath === node.path ? " selected" : "")}
          style={{ paddingLeft: 8 + depth * 14 }}
          onClick={() => onSelectFile(node.path)}
          title={node.path}
        >
          <span className="dot modified" style={{ opacity: 0.5 }} />
          <div className="row-main">
            <div className="row-name">{base}</div>
          </div>
        </div>
      );
    });

  return <div className="tree">{render(tree, 0)}</div>;
}

function countFiles(dir: PathDir): number {
  let n = 0;
  for (const c of dir.children) {
    if (c.type === "file") n++;
    else n += countFiles(c);
  }
  return n;
}
