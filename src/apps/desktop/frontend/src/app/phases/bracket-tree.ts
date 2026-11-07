import { MatchView, RoundView } from '@bindings/apps/desktop';

/** A match card of the bracket drawing: its column (round of the tree) and row. */
export interface TreeMatch {
  match: MatchView;
  /** Column, 0 for the first round. */
  col: number;
  /** Row, in card pitches (fractional when centred on what it feeds or is fed by). */
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
  /** Row of its topmost block, where its title goes. */
  row: number;
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

export interface BracketTree {
  /** Blocks (sub-brackets), titled in the drawing when the bracket has several per column. */
  nodes: TreeNode[];
  titled: boolean;
  matches: TreeMatch[];
  links: TreeLink[];
  columns: TreeColumn[];
  blocks: TreeBlock[];
  rows: number;
}

/** Entrants of the sub-brackets offered as jump buttons (places 1–4, 5–8…). */
const BLOCK_SIZE = 4;

/** Rows between the smallest blocks (finals) of a titled drawing: room for a title. */
const LEAF_PITCH = 1.75;

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

  // With full placement the blocks (sub-brackets) are laid out as a tree of
  // their own, from the right: the smallest ones evenly spaced, each larger one
  // centred on the two it feeds, its matches stacked and titled. Without it the
  // drawing is a plain bracket: a match sits between the two feeding it.
  const titled = sizes.some((size) => rounds.filter((r) => r.size === size).length > 1);
  const rowOf = new Map<string, number>();
  if (titled) {
    let leaf = 0;
    const place = (r: (typeof rounds)[number]): number => {
      const half = r.size / 2;
      const kids = [byNode.get(`${r.from}/${half}`), byNode.get(`${r.from + half}/${half}`)].filter((k) => !!k);
      const centre = kids.length > 0 ? kids.reduce((sum, k) => sum + place(k), 0) / kids.length : leaf++ * LEAF_PITCH;
      r.matches.forEach((m, i) => rowOf.set(m.id, centre - (r.matches.length - 1) / 2 + i));
      return centre;
    };
    rounds.filter((r) => r.size === sizes[0]).forEach(place);
    const top = Math.min(...rowOf.values());
    rowOf.forEach((row, id) => rowOf.set(id, row - top));
  }

  const columns: TreeColumn[] = [];
  const matches: TreeMatch[] = [];
  let rows = 0;
  sizes.forEach((size, c) => {
    const column = rounds
      .filter((r) => r.size === size)
      .sort((a, b) => a.from - b.from)
      .flatMap((r) => r.matches);
    if (column.length === 0) {
      return;
    }
    const previous = matches.filter((t) => t.col === c - 1).length;
    column.forEach((m, i) => {
      if (!rowOf.has(m.id)) {
        const feeders = links.filter((l) => l.to === m.id).map((l) => rowOf.get(l.from) ?? i);
        const grid = c === 0 || column.length === previous || feeders.length === 0;
        rowOf.set(m.id, grid ? i : feeders.reduce((a, b) => a + b, 0) / feeders.length);
      }
      const row = rowOf.get(m.id) ?? i;
      matches.push({ match: m, col: c, row });
      rows = Math.max(rows, row + 1);
    });
    columns.push({ col: c, size, first: column[0].id, row: Math.min(...column.map((m) => rowOf.get(m.id) ?? 0)) });
  });

  const nodes: TreeNode[] = [];
  for (const r of rounds) {
    const first = matches.find((t) => t.match.id === r.matches[0]?.id);
    if (first) {
      nodes.push({ from: r.from, to: r.to, col: first.col, row: first.row });
    }
  }
  nodes.sort((a, b) => a.col - b.col || a.row - b.row);

  const blocks = rounds
    .filter((r) => r.size === BLOCK_SIZE && r.matches.length > 0)
    .sort((a, b) => a.from - b.from)
    .map((r) => ({ from: r.from, to: r.to, first: r.matches[0].id }));

  return { nodes, titled, matches, links, columns, blocks, rows };
}

/** Geometry of the drawing, in px (the cards' height changes in correction mode). */
export interface TreeGeometry {
  cardW: number;
  cardH: number;
  colGap: number;
  rowGap: number;
  /** Space above the cards for the column titles (and the first block's). */
  head: number;
  /** Height of a block's title, above its first card. */
  title: number;
}

export function treeGeometry(correction: boolean, titled = false): TreeGeometry {
  return { cardW: 250, cardH: correction ? 166 : 124, colGap: 64, rowGap: 18, head: titled ? 58 : 34, title: 22 };
}

export function cardLeft(g: TreeGeometry, col: number): number {
  return col * (g.cardW + g.colGap);
}

export function cardTop(g: TreeGeometry, row: number): number {
  return g.head + row * (g.cardH + g.rowGap);
}

/** Which links of the drawing are shown. */
export type LinkFilter = 'all' | 'win' | 'lose' | 'none';

export function showsLink(filter: LinkFilter, kind: TreeLink['kind']): boolean {
  return filter === 'all' || filter === kind;
}

/**
 * SVG path of a link: out of the right middle of a card, a vertical run with
 * rounded corners, into the left middle of the next. Winner and loser runs
 * keep to either side of the middle so they never share a line.
 */
export function linkPath(g: TreeGeometry, kind: TreeLink['kind'], from: TreeMatch, to: TreeMatch): string {
  const x1 = cardLeft(g, from.col) + g.cardW;
  const y1 = cardTop(g, from.row) + g.cardH / 2;
  const x2 = cardLeft(g, to.col);
  const y2 = cardTop(g, to.row) + g.cardH / 2;
  const dy = y2 - y1;
  if (Math.abs(dy) < 1) {
    return `M${x1} ${y1} H${x2}`;
  }
  const xv = (x1 + x2) / 2 + (kind === 'win' ? -RUN_OFFSET : RUN_OFFSET);
  const r = Math.min(CORNER, Math.abs(dy) / 2);
  const s = Math.sign(dy);
  return `M${x1} ${y1} H${xv - r} Q${xv} ${y1} ${xv} ${y1 + s * r} V${y2 - s * r} Q${xv} ${y2} ${xv + r} ${y2} H${x2}`;
}

const RUN_OFFSET = 7;
const CORNER = 10;
