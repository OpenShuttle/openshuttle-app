import { MatchView, RoundView } from '@bindings/apps/desktop';

import { BracketTree, TreeLink, TreeMatch, bracketTree } from './bracket-tree';

/** A part of a bracket: the main draw, or the matches for places from..to. */
export interface BracketPart {
  from: number;
  to: number;
  /** First match, to scroll to. */
  first: string;
}

/** A bracket folded in two and its parts packed under the main draw. */
export interface FoldedBracket {
  tree: BracketTree;
  parts: BracketPart[];
}

/** Rows kept between two parts, beyond the gap between two cards. */
const PART_GAP = 0.25;

type Round = RoundView & { matches: MatchView[] };

/**
 * Lays a bracket out as compact as possible in height: the main draw
 * (places 1 to N) and each part playing for lower places (full placement:
 * the 3rd place, places 5 to 8, 9 to 16…) are folded in two: the matches fed
 * by the top half of the part's first round on the left, those fed by the
 * bottom half on the right, mirrored, and its last matches (a final, a match
 * for a place) in the middle, between the two matches feeding them; the
 * final and the 3rd place stacked between the semi-finals. Every part shares
 * the middle column with the final and comes as high as it can without
 * covering another: places 5 to 8 under the semi-finals, 9 to 16 under the
 * quarter-finals.
 *
 * Layout only, from the shape Go generates (`bracketplay.go`): match i of
 * (Start, Size) sends its winner to match i/2 of (Start, Size/2) and its
 * loser to match i/2 of (Start+Size/2, Size/2).
 */
export function foldBracket(source: RoundView[]): FoldedBracket {
  const rounds: Round[] = source.map((r) => ({ ...r, matches: (r.matches ?? []).filter((m): m is MatchView => !!m) }));
  const byNode = new Map(rounds.map((r) => [key(r.from, r.size), r]));
  const parent = (r: Round) => byNode.get(key(r.from, r.size * 2)) ?? byNode.get(key(r.from - r.size, r.size * 2));
  // The 3rd place sits with the final, stacked under it like the 5th and
  // 7th places: it belongs to the main draw's part.
  const third = byNode.get(key(3, 2));
  // A part starts at the main draw's first round, or where the losers of
  // the main draw go on.
  const roots = rounds
    .filter((r) => {
      const p = parent(r);
      return r !== third && (!p || (r.from > 1 && p.from === 1));
    })
    .sort((a, b) => a.from - b.from);
  const parts: BracketPart[] = [];
  const folded: BracketTree[] = [];
  for (const root of roots) {
    const nodes: Round[] = [];
    const collect = (r: Round | undefined) => {
      if (!r) {
        return;
      }
      nodes.push(r);
      collect(byNode.get(key(r.from, r.size / 2)));
      if (root.from > 1) {
        collect(byNode.get(key(r.from + r.size / 2, r.size / 2)));
      }
    };
    collect(root);
    if (root.from === 1 && third && parent(third)) {
      nodes.push(third);
    }
    const laid = fold(root, nodes);
    // A match between two byes is never anything: left out of the drawing.
    const tree = { ...laid, matches: laid.matches.filter((m) => !(m.match.byeA && m.match.byeB)) };
    if (tree.matches.length > 0) {
      parts.push({ from: root.from, to: root.from + root.size - 1, first: root.matches[0]?.id ?? '' });
      folded.push(tree);
    }
  }
  // Every link, within a part or from one part to another (a loser going on
  // to play for lower places): the parts share one drawing. A match with a
  // bye only sends its team on: no loser's link (it would carry the bye).
  const links: TreeLink[] = [];
  for (const r of rounds) {
    const win = byNode.get(key(r.from, r.size / 2));
    const lose = byNode.get(key(r.from + r.size / 2, r.size / 2));
    r.matches.forEach((m, i) => {
      const to = Math.floor(i / 2);
      const side = i % 2 === 0 ? 'a' : 'b';
      if (win?.matches[to] && !(m.byeA && m.byeB)) {
        links.push({ from: m.id, to: win.matches[to].id, kind: 'win', side });
      }
      if (lose?.matches[to] && !m.byeA && !m.byeB) {
        links.push({ from: m.id, to: lose.matches[to].id, kind: 'lose', side });
      }
    });
  }
  const tree = pack(folded);
  const drawn = new Set(tree.matches.map((m) => m.match.id));
  return { tree: { ...tree, links: links.filter((l) => drawn.has(l.from) && drawn.has(l.to)) }, parts };
}

/** The middle column of a folded part. */
function middleOf(t: BracketTree): number {
  return (t.columns.length - 1) / 2;
}

/**
 * Packs folded parts in one drawing, their middle columns on one column:
 * each part, in order, comes right under the cards already placed in its
 * columns (a card's height and PART_GAP below them).
 */
function pack(trees: BracketTree[]): BracketTree {
  if (trees.length === 0) {
    return { nodes: [], titled: false, matches: [], links: [], columns: [], blocks: [], rows: 0 };
  }
  const middle = Math.max(...trees.map(middleOf));
  const placed: TreeMatch[] = [];
  const links: TreeLink[] = [];
  for (const t of trees) {
    const shift = middle - middleOf(t);
    const cards = t.matches.map((m) => ({ ...m, col: m.col + shift }));
    // Under every card already in its columns, as high as that allows.
    let down = -Infinity;
    for (const c of cards) {
      for (const p of placed) {
        if (p.col === c.col) {
          down = Math.max(down, p.row + 1 + PART_GAP - c.row);
        }
      }
    }
    down = Number.isFinite(down) ? down : 0;
    placed.push(...cards.map((c) => ({ ...c, row: c.row + down })));
    links.push(...t.links);
  }
  const columns = Array.from({ length: 2 * middle + 1 }, (_, col) => {
    const first = placed.filter((m) => m.col === col).sort((a, b) => a.row - b.row)[0];
    return { col, size: 0, first: first?.match.id ?? '', row: first?.row ?? 0 };
  });
  return {
    nodes: [],
    titled: false,
    matches: placed,
    links,
    columns,
    blocks: [],
    rows: Math.max(...placed.map((m) => m.row)) + 1,
  };
}

function key(from: number, size: number): string {
  return `${from}/${size}`;
}

/** Folds a part (its root and every node below) in two halves around its last column. */
function fold(root: Round, nodes: Round[]): BracketTree {
  if (root.size <= 2) {
    return bracketTree(nodes);
  }
  // Each half of the part is laid out as a bracket of its own, half the
  // size: node (f, s) keeps half its matches as node (F + (f-F)/2, s/2).
  const half = (side: 0 | 1): Round[] =>
    nodes
      .filter((r) => r.size >= 4)
      .map((r) => {
        const n = r.matches.length / 2;
        return { ...r, from: root.from + (r.from - root.from) / 2, size: r.size / 2, matches: r.matches.slice(side * n, side * n + n) };
      });
  const left = bracketTree(half(0));
  const right = bracketTree(half(1));
  const middle = left.columns.length;
  const matches: TreeMatch[] = [
    ...left.matches,
    ...right.matches.map((m) => ({ ...m, col: 2 * middle - m.col })),
  ];
  const links: TreeLink[] = [...left.links, ...right.links];

  // The last column: each match between the two matches feeding it, those
  // fed by the same two stacked around them.
  const rowOf = new Map(matches.map((m) => [m.match.id, m.row]));
  const finals = nodes.filter((r) => r.size === 2).sort((a, b) => a.from - b.from);
  const centre = new Map<number, Round[]>();
  for (const r of finals) {
    const loser = (r.from - root.from) % 4 !== 0;
    const feeder = nodes.find((p) => p.size === 4 && p.from === (loser ? r.from - 2 : r.from));
    const row = feeder ? ((rowOf.get(feeder.matches[0]?.id) ?? 0) + (rowOf.get(feeder.matches[1]?.id) ?? 0)) / 2 : 0;
    centre.set(row, [...(centre.get(row) ?? []), r]);
    (feeder?.matches ?? []).forEach((m, i) => {
      if (r.matches[0]) {
        links.push({ from: m.id, to: r.matches[0].id, kind: loser ? 'lose' : 'win', side: i % 2 === 0 ? 'a' : 'b' });
      }
    });
  }
  // Stacked matches never overlap: a group too close to the one above moves down.
  let floor = -Infinity;
  for (const [row, list] of [...centre].sort((a, b) => a[0] - b[0])) {
    const start = Math.max(row - (list.length - 1) / 2, floor);
    list.forEach((r, j) => {
      if (r.matches[0]) {
        matches.push({ match: r.matches[0], col: middle, row: start + j });
      }
    });
    floor = start + list.length;
  }
  const top = Math.min(...matches.map((m) => m.row));
  const placed = matches.map((m) => ({ ...m, row: m.row - top }));
  const columns = Array.from({ length: 2 * middle + 1 }, (_, col) => {
    const first = placed.filter((m) => m.col === col).sort((a, b) => a.row - b.row)[0];
    return { col, size: 0, first: first?.match.id ?? '', row: first?.row ?? 0 };
  });
  return {
    nodes: [],
    titled: false,
    matches: placed,
    links,
    columns,
    blocks: [],
    rows: Math.max(...placed.map((m) => m.row)) + 1,
  };
}
