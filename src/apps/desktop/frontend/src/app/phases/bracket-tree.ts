import { MatchView, RoundView } from '@bindings/apps/desktop';

/** A match card of the bracket drawing: its column (round of the tree) and row. */
export interface TreeMatch {
  match: MatchView;
  /** Column, 0 for the first round. */
  col: number;
  /** Row, in card heights (fractional when centred between the matches feeding it). */
  row: number;
}

/** A line from a match to the one its winner or its loser plays next. */
export interface TreeLink {
  from: string;
  to: string;
  kind: 'win' | 'lose';
}

/** A column of the drawing: the entrants a round of the tree starts with. */
export interface TreeColumn {
  col: number;
  size: number;
  /** First match of the column, to scroll to. */
  first: string;
}

/** A part of the bracket playing for places `from`..`to` (a sub-bracket to jump to). */
export interface TreeBlock {
  from: number;
  to: number;
  first: string;
}

/** A sub-bracket of the drawing: a titled block of cards, set apart from the next. */
export interface TreeNode {
  from: number;
  to: number;
  col: number;
  /** Row of its first match. */
  row: number;
}

/** Space opened before a row where a block starts, in the drawing's gap units. */
export interface TreeGap {
  row: number;
  /** How coarse the block is: 1 for a final, 2 for 4 entrants, 3 for 8… */
  level: number;
}

export interface BracketTree {
  /** Blocks (sub-brackets), titled in the drawing when the bracket has several per column. */
  nodes: TreeNode[];
  titled: boolean;
  gaps: TreeGap[];
  matches: TreeMatch[];
  links: TreeLink[];
  columns: TreeColumn[];
  blocks: TreeBlock[];
  rows: number;
}

/** Entrants of the sub-brackets offered as jump buttons (places 1–4, 5–8…). */
const BLOCK_SIZE = 4;

/**
 * Lays a bracket out as a tree, left to right: one column per round (32, 16,
 * 8… entrants), the winner of a match going up and its loser going down.
 *
 * It only draws the shape Go generates (`bracketplay.go`): a sub-bracket
 * (Start, Size) holds Size/2 matches; match i sends its winner to match i/2 of
 * (Start, Size/2) and, with full placement, its loser to match i/2 of
 * (Start+Size/2, Size/2). Who plays whom and who wins stays Go's.
 */
export function bracketTree(source: RoundView[]): BracketTree {
  const rounds = source.map((r) => ({ ...r, matches: (r.matches ?? []).filter((m): m is MatchView => !!m) }));
  const sizes = [...new Set(rounds.map((r) => r.size))].sort((a, b) => b - a);
  const col = (size: number) => sizes.indexOf(size);
  const byNode = new Map(rounds.map((r) => [`${r.from}/${r.size}`, r]));

  const links: TreeLink[] = [];
  for (const r of rounds) {
    const half = r.size / 2;
    const win = byNode.get(`${r.from}/${half}`);
    const lose = byNode.get(`${r.from + half}/${half}`);
    r.matches.forEach((m, i) => {
      const to = Math.floor(i / 2);
      if (win?.matches[to]) {
        links.push({ from: m.id, to: win.matches[to].id, kind: 'win' });
      }
      if (lose?.matches[to]) {
        links.push({ from: m.id, to: lose.matches[to].id, kind: 'lose' });
      }
    });
  }

  const columns: TreeColumn[] = [];
  const matches: TreeMatch[] = [];
  const rowOf = new Map<string, number>();
  let rows = 0;
  sizes.forEach((size, c) => {
    const column = rounds
      .filter((r) => r.size === size)
      .sort((a, b) => a.from - b.from)
      .flatMap((r) => r.matches);
    if (column.length === 0) {
      return;
    }
    columns.push({ col: c, size, first: column[0].id });
    // A column as full as the previous one (full placement) is a grid; a
    // narrower one (winners only) sits between the matches feeding it.
    const previous = matches.filter((t) => t.col === c - 1).length;
    column.forEach((m, i) => {
      const feeders = links.filter((l) => l.to === m.id).map((l) => rowOf.get(l.from) ?? i);
      const grid = c === 0 || column.length === previous || feeders.length === 0;
      const row = grid ? i : feeders.reduce((a, b) => a + b, 0) / feeders.length;
      rowOf.set(m.id, row);
      matches.push({ match: m, col: c, row });
      rows = Math.max(rows, row + 1);
    });
  });

  const nodes: TreeNode[] = [];
  for (const r of rounds) {
    const first = matches.find((t) => t.match.id === r.matches[0]?.id);
    if (first) {
      nodes.push({ from: r.from, to: r.to, col: first.col, row: first.row });
    }
  }
  nodes.sort((a, b) => a.col - b.col || a.row - b.row);
  const titled = nodes.some((n, i) => nodes.findIndex((o) => o.col === n.col) !== i);
  // A block opens a gap before its first row, wider for coarser blocks; the
  // first row needs none. Every column shares the gaps so its cards stay level.
  const gaps: TreeGap[] = [];
  if (titled) {
    for (const r of rounds) {
      const first = matches.find((t) => t.match.id === r.matches[0]?.id);
      if (first && first.row > 0) {
        const level = Math.log2(r.size);
        const known = gaps.find((g) => g.row === first.row);
        if (known) {
          known.level = Math.max(known.level, level);
        } else {
          gaps.push({ row: first.row, level });
        }
      }
    }
    gaps.sort((a, b) => a.row - b.row);
  }

  const blocks = rounds
    .filter((r) => r.size === BLOCK_SIZE && r.matches.length > 0)
    .sort((a, b) => a.from - b.from)
    .map((r) => ({ from: r.from, to: r.to, first: r.matches[0].id }));

  return { nodes, titled, gaps, matches, links, columns, blocks, rows };
}

/** Geometry of the drawing, in px (the cards' height changes in correction mode). */
export interface TreeGeometry {
  cardW: number;
  cardH: number;
  colGap: number;
  rowGap: number;
  /** Space above the cards for the column titles (and the first block's). */
  head: number;
  /** Height of a block's title, opened above it. */
  title: number;
  /** Extra space per level of a block starting a row. */
  sep: number;
}

export function treeGeometry(correction: boolean, titled = false): TreeGeometry {
  return { cardW: 250, cardH: correction ? 152 : 110, colGap: 64, rowGap: 18, head: titled ? 58 : 34, title: 22, sep: 12 };
}

export function cardLeft(g: TreeGeometry, col: number): number {
  return col * (g.cardW + g.colGap);
}

export function cardTop(g: TreeGeometry, gaps: TreeGap[], row: number): number {
  const extra = gaps.filter((x) => x.row <= row).reduce((sum, x) => sum + g.title + x.level * g.sep, 0);
  return g.head + row * (g.cardH + g.rowGap) + extra;
}

/** SVG path of a link: out of the right middle of a card, into the left middle of the next. */
export function linkPath(g: TreeGeometry, gaps: TreeGap[], from: TreeMatch, to: TreeMatch): string {
  const x1 = cardLeft(g, from.col) + g.cardW;
  const y1 = cardTop(g, gaps, from.row) + g.cardH / 2;
  const x2 = cardLeft(g, to.col);
  const y2 = cardTop(g, gaps, to.row) + g.cardH / 2;
  const mid = (x1 + x2) / 2;
  return `M${x1} ${y1} C${mid} ${y1} ${mid} ${y2} ${x2} ${y2}`;
}
