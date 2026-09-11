import { LANE_WIDTH, ROW_HEIGHT, laneColor, type GraphRow } from "../commitGraph";

// 单个提交行的图形。用 SVG 画，才能有平滑的合并连线。

export function CommitGraph({ row }: { row: GraphRow }) {
  const width = Math.min(row.laneCount, 8) * LANE_WIDTH + 6;
  const cy = ROW_HEIGHT / 2;
  const x = (lane: number) => Math.min(lane, 7) * LANE_WIDTH + LANE_WIDTH / 2;

  return (
    <svg
      className="commit-graph"
      width={width}
      height={ROW_HEIGHT}
      viewBox={`0 0 ${width} ${ROW_HEIGHT}`}
    >
      {/* 穿透本行的竖线 */}
      {row.through.map((lane) => (
        <line
          key={`t${lane}`}
          x1={x(lane)}
          y1={0}
          x2={x(lane)}
          y2={ROW_HEIGHT}
          stroke={lane === row.lane ? row.color : laneColor(lane)}
          strokeWidth={1.6}
          opacity={lane === row.lane ? 0.9 : 0.5}
        />
      ))}

      {/* 合并提交：从节点向右下连到各个父泳道 */}
      {row.links.map((l, i) => {
        const x1 = x(l.from);
        const x2 = x(l.to);
        return (
          <path
            key={`l${i}`}
            d={`M ${x1} ${cy} C ${x1} ${cy + 18}, ${x2} ${cy + 8}, ${x2} ${ROW_HEIGHT}`}
            fill="none"
            stroke={l.color}
            strokeWidth={1.6}
            opacity={0.75}
          />
        );
      })}

      {/* 当前提交的节点 */}
      <circle cx={x(row.lane)} cy={cy} r={row.isMerge ? 4.2 : 3.4} fill={row.color} />
      <circle cx={x(row.lane)} cy={cy} r={row.isMerge ? 8 : 7} fill={row.color} opacity={0.18} />
    </svg>
  );
}
