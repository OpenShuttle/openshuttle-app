import { StageView } from '@bindings/apps/desktop';

/** Canvas geometry of the stage graph, in pixels. */
export const NODE_W = 190;
export const NODE_H = 140;
export const OUTPUT_W = 170;
const COL_GAP = 140;
const ROW_GAP = 26;
const PAD = 16;
/** Height of the lane under the stages that early exits run along. */
const LANE_H = 44;

export interface GraphNode {
  stage: StageView;
  x: number;
  y: number;
}

export interface GraphEdge {
  d: string;
  label: string;
  x: number;
  y: number;
}

/** The stage graph laid out: one column per phase, the output (final places) last. */
export interface FormatGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
  width: number;
  height: number;
  outputX: number;
  outputY: number;
}

/**
 * Lays the stages out: each phase in a column, its stages centred; an edge
 * per stage fed by routing (labelled with how many entrants it sends, by
 * edgeLabel), and one to the output for stages giving final places (along a
 * lane under the graph when they are not in the last phase).
 */
export function layoutGraph(stages: StageView[], edgeLabel: (count: number) => string): FormatGraph {
  const phases = Math.max(1, ...stages.map((s) => s.phase));
  const byPhase = new Map<number, StageView[]>();
  for (const s of stages) {
    byPhase.set(s.phase, [...(byPhase.get(s.phase) ?? []), s]);
  }
  const rows = Math.max(1, ...[...byPhase.values()].map((l) => l.length));
  const rowsHeight = Math.max(220, rows * (NODE_H + ROW_GAP) - ROW_GAP + 2 * PAD);
  const hasLane = stages.some((s) => s.placeFrom > 0 && s.phase < phases);
  const outputX = PAD + phases * (NODE_W + COL_GAP);
  const outputY = rowsHeight / 2 - NODE_H / 2;

  const nodes: GraphNode[] = [];
  for (const [phase, list] of byPhase) {
    const top = rowsHeight / 2 - (list.length * (NODE_H + ROW_GAP) - ROW_GAP) / 2;
    list.forEach((stage, i) => nodes.push({ stage, x: PAD + (phase - 1) * (NODE_W + COL_GAP), y: top + i * (NODE_H + ROW_GAP) }));
  }

  const byId = new Map(nodes.map((n) => [n.stage.id, n]));
  const exit = { x: outputX, y: outputY + NODE_H / 2 };
  const edges: GraphEdge[] = [];
  for (const n of nodes) {
    const from = { x: n.x + NODE_W, y: n.y + NODE_H / 2 };
    const counts = new Map<string, number>();
    (n.stage.routing ?? []).forEach((r, i) => counts.set(r.to, (counts.get(r.to) ?? 0) + (n.stage.rules?.[i] ?? 0)));
    for (const [target, count] of counts) {
      const to = byId.get(target);
      if (to) {
        edges.push(edge(from, { x: to.x, y: to.y + NODE_H / 2 }, edgeLabel(count)));
      }
    }
    if (n.stage.placeFrom > 0) {
      const label = places(n.stage.placeFrom, n.stage.placeTo);
      edges.push(n.stage.phase < phases ? laneEdge(from, exit, rowsHeight + LANE_H / 2 - PAD / 2, label) : edge(from, exit, label));
    }
  }
  return { nodes, edges, width: outputX + OUTPUT_W + PAD, height: rowsHeight + (hasLane ? LANE_H : 0), outputX, outputY };
}

/** "5" or "5–8". */
export function places(from: number, to: number): string {
  return from === to ? `${from}` : `${from}–${to}`;
}

/** An edge that leaves a stage downwards, runs along the lane at y and rises into the output, labelled near its start. */
function laneEdge(a: { x: number; y: number }, b: { x: number; y: number }, y: number, label: string): GraphEdge {
  const d = [
    `M ${a.x} ${a.y}`,
    `C ${a.x + 40} ${a.y}, ${a.x + 40} ${y}, ${a.x + 80} ${y}`,
    `L ${b.x - 80} ${y}`,
    `C ${b.x - 40} ${y}, ${b.x - 40} ${b.y}, ${b.x} ${b.y}`,
  ].join(' ');
  return { d, label, x: a.x + COL_GAP / 2 + 40, y };
}

/** A curved edge between two points, with its label in the middle. */
function edge(a: { x: number; y: number }, b: { x: number; y: number }, label: string): GraphEdge {
  const mid = (a.x + b.x) / 2;
  return { d: `M ${a.x} ${a.y} C ${mid} ${a.y}, ${mid} ${b.y}, ${b.x} ${b.y}`, label, x: mid, y: (a.y + b.y) / 2 };
}
