import type { FileDTO } from "./types";

// 把平铺的文件列表构建成目录树。
//
// 算法复刻自 lazygit 的 pkg/gui/filetree/build_tree.go：
//   1. 按路径逐段插入节点
//   2. 排序：目录在前、文件在后，各自按名称
//   3. Compress：把只有单个子目录的链条合并成 "a/b/c" 一项
//
// 第 3 步是文件树好不好用的关键 —— 否则 src/components/Foo.tsx
// 会硬生生变成三层缩进，浪费屏幕。

export interface TreeLeaf {
  type: "file";
  name: string;
  path: string;
  file: FileDTO;
}

export interface TreeDir {
  type: "dir";
  name: string;
  path: string;
  children: TreeNode[];
}

export type TreeNode = TreeLeaf | TreeDir;

export function buildFileTree(files: FileDTO[]): TreeNode[] {
  const root: TreeDir = { type: "dir", name: "", path: "", children: [] };
  const dirs = new Map<string, TreeDir>([["", root]]);

  for (const f of files) {
    const parts = f.path.split("/");
    let cur = root;
    let acc = "";

    for (let i = 0; i < parts.length - 1; i++) {
      acc = acc ? `${acc}/${parts[i]}` : parts[i];
      let next = dirs.get(acc);
      if (!next) {
        next = { type: "dir", name: parts[i], path: acc, children: [] };
        dirs.set(acc, next);
        cur.children.push(next);
      }
      cur = next;
    }

    cur.children.push({
      type: "file",
      name: parts[parts.length - 1],
      path: f.path,
      file: f,
    });
  }

  sortNodes(root);
  compress(root);
  return root.children;
}

function sortNodes(dir: TreeDir): void {
  dir.children.sort((a, b) => {
    // 目录排在文件前面，同类按名称
    if (a.type !== b.type) return a.type === "dir" ? -1 : 1;
    return a.name.localeCompare(b.name, "zh");
  });
  for (const c of dir.children) {
    if (c.type === "dir") sortNodes(c);
  }
}

// compress 把「只有一个子目录」的链条合并：
//   src → components → Foo.tsx
// 变成
//   src/components → Foo.tsx
function compress(dir: TreeDir): void {
  dir.children = dir.children.map((child) => {
    if (child.type !== "dir") return child;

    let node = child;
    // 一路向下合并，直到遇到分叉或文件
    while (
      node.children.length === 1 &&
      node.children[0].type === "dir"
    ) {
      const only = node.children[0] as TreeDir;
      node = {
        type: "dir",
        name: `${node.name}/${only.name}`,
        path: only.path,
        children: only.children,
      };
    }
    node.children = node.children.map((c) => (c.type === "dir" ? compressInto(c) : c));
    return node;
  });
}

// compressInto 对子目录做同样的处理并返回它自己
function compressInto(dir: TreeDir): TreeDir {
  compress(dir);
  return dir;
}

/** 收集树里所有目录的 path，用于「全部展开」。 */
export function allDirPaths(nodes: TreeNode[]): string[] {
  const out: string[] = [];
  const walk = (list: TreeNode[]) => {
    for (const n of list) {
      if (n.type === "dir") {
        out.push(n.path);
        walk(n.children);
      }
    }
  };
  walk(nodes);
  return out;
}
