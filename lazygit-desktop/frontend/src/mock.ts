// 纯前端的演示数据。
//
// 作用：不启动 Go、不编译，直接 `npm run dev` 就能看到并操作完整界面。
// 当运行在 Wails 里时，这一整套会被真实引擎替换掉（见 api.ts）。

import type { FileDTO, RepoSnapshot } from "./types";

function file(partial: Partial<FileDTO> & { path: string }): FileDTO {
  return {
    previousPath: "",
    status: "M",
    statusLabel: "修改",
    kind: "modified",
    isStaged: false,
    isUnstaged: true,
    isTracked: true,
    hasConflicts: false,
    linesAdded: 0,
    linesDeleted: 0,
    ...partial,
  };
}

function build(): RepoSnapshot {
  return {
    repoPath: "/home/user/projects/demo",
    repoName: "demo",
    branch: "feature/rounded-ui",
    isDetached: false,
    state: "",
    identityName: "you",
    identityEmail: "you@example.com",
    files: [
      file({
        path: "pkg/engine/session.go",
        status: "M",
        statusLabel: "修改",
        kind: "modified",
        isStaged: true,
        isUnstaged: false,
        linesAdded: 24,
        linesDeleted: 6,
      }),
      file({
        path: "frontend/src/components/DiffPanel.tsx",
        status: "A",
        statusLabel: "新增",
        kind: "new",
        isStaged: true,
        isUnstaged: false,
        isTracked: true,
        linesAdded: 96,
        linesDeleted: 0,
      }),
      file({
        path: "frontend/src/styles.css",
        status: "M",
        statusLabel: "修改",
        kind: "modified",
        isStaged: false,
        isUnstaged: true,
        linesAdded: 41,
        linesDeleted: 12,
      }),
      file({
        path: "README.md",
        status: "M",
        statusLabel: "修改",
        kind: "modified",
        isStaged: false,
        isUnstaged: true,
        linesAdded: 5,
        linesDeleted: 1,
      }),
      file({
        path: "docs/screenshots/sidebar.png",
        status: "??",
        statusLabel: "未跟踪",
        kind: "untracked",
        isStaged: false,
        isUnstaged: true,
        isTracked: false,
      }),
    ],
    commits: [
      {
        hash: "9f2c41b7a8e5d3c1f0a9b8c7d6e5f4a3b2c1d0e9",
        shortHash: "9f2c41b",
        subject: "抽出无界面引擎，与 UI 解耦",
        author: "you",
        when: "12 分钟前",
        tags: [],
        extraInfo: "HEAD -> feature/rounded-ui",
      },
      {
        hash: "3a7d19c4b2e6f8a1c3d5e7f9b0a2c4d6e8f0a1b3",
        shortHash: "3a7d19c",
        subject: "文件面板支持按状态分组",
        author: "you",
        when: "2 小时前",
        tags: [],
        extraInfo: "",
      },
      {
        hash: "b1e4f7a0c3d6b9e2f5a8c1d4b7e0f3a6c9d2b5e8",
        shortHash: "b1e4f7a",
        subject: "圆角面板与胶囊按钮的基础样式",
        author: "you",
        when: "昨天",
        tags: ["v0.1.0"],
        extraInfo: "tag: v0.1.0",
      },
      {
        hash: "c8f2a5d1e4b7c0a3f6d9e2b5c8a1f4d7e0b3c6a9",
        shortHash: "c8f2a5d",
        subject: "接入 lazygit 的 git_commands 层",
        author: "you",
        when: "3 天前",
        tags: [],
        extraInfo: "",
      },
    ],
    branches: [
      {
        name: "feature/rounded-ui",
        isHead: true,
        ahead: "2",
        behind: "",
        upstream: "origin/feature/rounded-ui",
        subject: "抽出无界面引擎，与 UI 解耦",
      },
      {
        name: "main",
        isHead: false,
        ahead: "",
        behind: "1",
        upstream: "origin/main",
        subject: "Merge pull request #128",
      },
      {
        name: "fix/diff-scroll",
        isHead: false,
        ahead: "",
        behind: "",
        upstream: "",
        subject: "修正 diff 滚动位置",
      },
    ],
  };
}

let state: RepoSnapshot = build();

export function mockSnapshot(): RepoSnapshot {
  return structuredClone(state);
}

export function mockStage(path: string): RepoSnapshot {
  state.files = state.files.map((f) =>
    f.path === path ? { ...f, isStaged: true, isUnstaged: false } : f,
  );
  return mockSnapshot();
}

export function mockUnstage(path: string): RepoSnapshot {
  state.files = state.files.map((f) =>
    f.path === path ? { ...f, isStaged: false, isUnstaged: true } : f,
  );
  return mockSnapshot();
}

export function mockStageAll(): RepoSnapshot {
  state.files = state.files.map((f) => ({ ...f, isStaged: true, isUnstaged: false }));
  return mockSnapshot();
}

export function mockUnstageAll(): RepoSnapshot {
  state.files = state.files.map((f) => ({ ...f, isStaged: false, isUnstaged: true }));
  return mockSnapshot();
}

export function mockDiscard(path: string): RepoSnapshot {
  state.files = state.files.filter((f) => f.path !== path);
  return mockSnapshot();
}

export function mockCommit(summary: string): RepoSnapshot {
  const staged = state.files.filter((f) => f.isStaged);
  const rest = state.files.filter((f) => !f.isStaged);

  if (staged.length > 0) {
    state.files = rest;
    state.commits = [
      {
        hash: Math.random().toString(16).slice(2).padEnd(40, "0"),
        shortHash: Math.random().toString(16).slice(2, 10),
        subject: summary,
        author: "you",
        when: "刚刚",
        tags: [],
        extraInfo: "HEAD -> " + state.branch,
      },
      ...state.commits,
    ];
  }
  return mockSnapshot();
}

export function mockCheckout(name: string): RepoSnapshot {
  state.branch = name;
  state.branches = state.branches.map((b) => ({ ...b, isHead: b.name === name }));
  return mockSnapshot();
}

export function mockDiff(path: string, staged: boolean): string {
  const target = path || "file.go";
  return [
    `diff --git a/${target} b/${target}`,
    staged ? "index 4a1b2c3..9f8e7d6 100644" : "index 4a1b2c3..7c6d5e4 100644",
    `--- a/${target}`,
    `+++ b/${target}`,
    "@@ -1,7 +1,12 @@",
    " package main",
    " ",
    ' import (',
    '-\t"fmt"',
    '+\t"fmt"',
    '+\t"os"',
    '+\t"strings"',
    " ",
    '-\t"example.com/legacy"',
    '+\t"example.com/core"',
    " )",
    "@@ -14,10 +19,18 @@ func main() {",
    "-func run() error {",
    "+// run 启动主流程。",
    "+// 现在会在出错时打印更完整的上下文。",
    "+func run() error {",
    " \tcfg, err := loadConfig()",
    " \tif err != nil {",
    "-\t\treturn err",
    "+\t\tfmt.Fprintf(os.Stderr, \"加载配置失败: %v\\n\", err)",
    "+\t\treturn fmt.Errorf(\"loadConfig: %w\", err)",
    " \t}",
    " ",
    "-\treturn serve(cfg)",
    "+\tif strings.TrimSpace(cfg.Addr) == \"\" {",
    "+\t\tcfg.Addr = \":8080\"",
    "+\t}",
    "+\treturn serve(cfg)",
    " }",
    "",
  ].join("\n");
}
