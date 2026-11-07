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
  /** The side of `to` it feeds: match i of a round feeds side a when i is even. */
  side: 'a' | 'b';
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
        links.push({ from: m.id, to: win.matches[to].id, kind: 'win', side: i % 2 === 0 ? 'a' : 'b' });
      }
      if (lose?.matches[to]) {
        links.push({ from: m.id, to: lose.matches[to].id, kind: 'lose', side: i % 2 === 0 ? 'a' : 'b' });
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
    // Roots: the parts fed by no other part shown (the whole bracket, or the
    // places left once the main draw is drawn apart).
    rounds
      .filter((r) => !byNode.has(`${r.from}/${r.size * 2}`) && !byNode.has(`${r.from - r.size}/${r.size * 2}`))
      .forEach(place);
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
  /** Height of a card's head (its name and result), and of its correction buttons below the sides. */
  headH: number;
  actionsH: number;
}

/** Height of a card's head and of its correction buttons, as `bracket-match.ts` and `phases.scss` set them. */
export const CARD_HEAD = 27;
export const CARD_ACTIONS = 39;

export function treeGeometry(correction: boolean, titled = false, compact = false, cardW = 250): TreeGeometry {
  // Compact: cards without clubs (display screen), a single line per side.
  const cardH = compact ? 88 : correction ? 166 : 124;
  return {
    cardW,
    cardH,
    colGap: compact ? 48 : 64,
    rowGap: compact ? 12 : 18,
    head: titled ? 58 : compact ? 8 : 34,
    title: 22,
    headH: CARD_HEAD,
    actionsH: correction ? CARD_ACTIONS : 0,
  };
}

export function cardLeft(g: TreeGeometry, col: number): number {
  return col * (g.cardW + g.colGap);
}

export function cardTop(g: TreeGeometry, row: number): number {
  return g.head + row * (g.cardH + g.rowGap);
}

/** A square run from (x1, y1) to (x2, y2), going vertical at xv, corners rounded. */
function run(x1: number, y1: number, x2: number, y2: number, xv: number): string {
  const dy = y2 - y1;
  if (Math.abs(dy) < 1) {
    return `M${x1} ${y1} H${x2}`;
  }
  const dx = Math.sign(x2 - x1) || 1;
  const r = Math.min(CORNER, Math.abs(dy) / 2, Math.abs(xv - x1), Math.abs(x2 - xv));
  const s = Math.sign(dy);
  return `M${x1} ${y1} H${xv - dx * r} Q${xv} ${y1} ${xv} ${y1 + s * r} V${y2 - s * r} Q${xv} ${y2} ${xv + dx * r} ${y2} H${x2}`;
}

const CORNER = 10;

/** A link of the drawing with its SVG path. */
export interface LinkRoute {
  link: TreeLink;
  d: string;
}

/** Side of a match that won it: a bye loses at once, then forfeits, then sets. */
function winnerSide(m: MatchView): 'a' | 'b' | '' {
  if (m.byeA !== m.byeB) {
    return m.byeA ? 'b' : 'a';
  }
  if (m.walkoverA !== m.walkoverB) {
    return m.walkoverA ? 'b' : 'a';
  }
  let a = 0;
  let b = 0;
  for (const set of m.score?.sets ?? []) {
    if (set.a > set.b) a++;
    else if (set.b > set.a) b++;
  }
  return a > b ? 'a' : b > a ? 'b' : '';
}

/**
 * Routes the links of a drawing. A link leaves its match between the two
 * teams while it has no result, then from the team it carries (the winner
 * for a winner's link, the loser for a loser's), and enters the next match at
 * the team it feeds. Links crossing the same gap between two columns go
 * vertical on separate lanes whenever their vertical runs would overlap.
 * The right half of a folded bracket runs right to left. With `winnerFirst`
 * (the display screen), a decided match shows its winner on top.
 */
export function routeLinks(g: TreeGeometry, t: BracketTree, links: TreeLink[], winnerFirst = false): LinkRoute[] {
  const at = new Map(t.matches.map((m) => [m.match.id, m]));
  const sideH = (g.cardH - g.headH - g.actionsH) / 2;
  // The row a side is drawn on: the winner moves on top with winnerFirst.
  const shown = (m: TreeMatch, side: 'a' | 'b' | '') =>
    winnerFirst && side && winnerSide(m.match) === 'b' ? (side === 'a' ? 'b' : 'a') : side;
  const sideY = (m: TreeMatch, side: 'a' | 'b' | '') => {
    const row = shown(m, side);
    return cardTop(g, m.row) + g.headH + (row === 'a' ? sideH / 2 : row === 'b' ? sideH * 1.5 : sideH);
  };
  const ends = links.flatMap((link) => {
    const from = at.get(link.from);
    const to = at.get(link.to);
    if (!from || !to || from.col === to.col) {
      return [];
    }
    const won = winnerSide(from.match);
    const carried = !won ? '' : link.kind === 'win' ? won : won === 'a' ? 'b' : 'a';
    const right = from.col < to.col;
    return [
      {
        link,
        gap: Math.min(from.col, to.col),
        x1: right ? cardLeft(g, from.col) + g.cardW : cardLeft(g, from.col),
        y1: sideY(from, carried),
        x2: right ? cardLeft(g, to.col) : cardLeft(g, to.col) + g.cardW,
        y2: sideY(to, link.side),
      },
    ];
  });
  // Lanes per gap, counted from the side the links leave (the outside of
  // the drawing): the lower a link leaves, the further out it runs. A run
  // takes the outermost lane free over its whole height.
  const lane = new Map<(typeof ends)[number], number>();
  const lanes = new Map<number, number>();
  const byGap = new Map<number, typeof ends>();
  for (const e of ends) {
    byGap.set(e.gap, [...(byGap.get(e.gap) ?? []), e]);
  }
  for (const [gap, list] of byGap) {
    const taken: [number, number][][] = [];
    for (const e of [...list].sort((a, b) => b.y1 - a.y1)) {
      if (Math.abs(e.y2 - e.y1) < 1) {
        continue;
      }
      const top = Math.min(e.y1, e.y2);
      const bottom = Math.max(e.y1, e.y2);
      const free = (runs: [number, number][]) =>
        runs.every(([t, b]) => b + LANE_CLEARANCE < top || bottom + LANE_CLEARANCE < t);
      let i = taken.findIndex(free);
      if (i < 0) {
        i = taken.length;
        taken.push([]);
      }
      taken[i].push([top, bottom]);
      lane.set(e, i);
    }
    lanes.set(gap, taken.length);
  }
  return ends.map((e) => {
    const n = lanes.get(e.gap) ?? 1;
    const step = (g.colGap * ((lane.get(e) ?? 0) + 1)) / (n + 1);
    // Lane 0 next to the card the link leaves.
    const xv = e.x1 < e.x2 ? e.x1 + step : e.x1 - step;
    return { link: e.link, d: run(e.x1, e.y1, e.x2, e.y2, xv) };
  });
}

/** Vertical room between two runs sharing a lane. */
const LANE_CLEARANCE = 6;
