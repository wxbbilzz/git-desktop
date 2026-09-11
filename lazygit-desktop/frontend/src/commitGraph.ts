import type { CommitDTO } from "./types";

// 提交图的泳道计算。
//
// 思路（和常见的 git 图形客户端一致）：
//   - 维护一组「泳道」，每个泳道记录它在等待哪个提交
//   - 按时间从新到旧遍历提交，找到它所在的泳道作为节点位置
//   - 该泳道接着等待它的第一个父提交（于是竖线连下去）
//   - 合并提交的其余父提交各占一个新泳道（于是从节点往右下方连线）
//   - 等待的提交已经全部出现过的泳道就释放掉

export const LANE_WIDTH = 14;
export const ROW_HEIGHT = 46;

const COLORS = [
  "#7c6cff",
  "#3ddc97",
  "#5aa9ff",
  "#ffcf5c",
  "#ff6b81",
  "#22d3ee",
  "#f472b6",
  "#a3e635",
  "#fb923c",
  "#c084fc",
];

export interface GraphLink {
  /** 从哪个泳道出发（本提交所在泳道） */
  from: number;
  /** 连到哪个泳道 */
  to: number;
  color: string;
}

export interface GraphRow {
  /** 本提交的泳道 */
  lane: number;
  /** 本行需要用到的泳道总数（决定左边留多宽） */
  laneCount: number;
  /** 本行的颜色 */
  color: string;
  /** 这些泳道有竖线穿过本行（上下贯通） */
  through: number[];
  /** 到父提交的斜线（合并提交会有多条） */
  links: GraphLink[];
  /** 是否是合并提交 */
  isMerge: boolean;
}

export function laneColor(lane: number): string {
  return COLORS[lane % COLORS.length];
}

export function computeGraph(commits: CommitDTO[]): GraphRow[] {
  // lanes[i] = 第 i 条泳道正在等待的提交哈希；null 表示空闲
  const lanes: (string | null)[] = [];
  const rows: GraphRow[] = [];

  for (const c of commits) {
    // 找自己所在的泳道；没有就占一个空闲的
    let lane = lanes.indexOf(c.hash);
    if (lane === -1) {
      lane = lanes.indexOf(null);
      if (lane === -1) {
        lane = lanes.length;
        lanes.push(null);
      }
    }

    const parents = c.parents ?? [];

    // 先把当前泳道指向第一个父提交
    lanes[lane] = parents[0] ?? null;

    // 合并提交的其余父提交各占一条泳道
    const links: GraphLink[] = [];
    for (let p = 1; p < parents.length; p++) {
      let slot = lanes.indexOf(null);
      if (slot === -1) {
        slot = lanes.length;
        lanes.push(null);
      }
      lanes[slot] = parents[p];
      links.push({ from: lane, to: slot, color: laneColor(slot) });
    }

    // 本行结束后哪些泳道仍有线穿过
    const through: number[] = [];
    for (let i = 0; i < lanes.length; i++) {
      if (lanes[i] !== null) through.push(i);
    }

    rows.push({
      lane,
      laneCount: Math.max(lanes.length, lane + 1),
      color: laneColor(lane),
      through,
      links,
      isMerge: parents.length > 1,
    });

    // 收尾：砍掉尾部连续的空泳道，避免图越来越宽
    while (lanes.length > 0 && lanes[lanes.length - 1] === null) {
      lanes.pop();
    }
  }

  // 统一用最大泳道数，保证每行左侧宽度一致
  const maxLanes = rows.reduce((m, r) => Math.max(m, r.laneCount), 1);
  for (const r of rows) r.laneCount = maxLanes;

  return rows;
}

/** 提交图需要占用的像素宽度（留一点右边距）。 */
export function graphWidth(rows: GraphRow[]): number {
  const max = rows.reduce((m, r) => Math.max(m, r.laneCount), 1);
  return Math.min(max, 8) * LANE_WIDTH + 8;
}
