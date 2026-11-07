import { MatchView, RoundView } from '@bindings/apps/desktop';

import { MatchView as Match } from '@bindings/apps/desktop';

import { bracketTree, cardLeft, cardTop, routeLinks, treeGeometry } from './bracket-tree';

/** Rounds of a bracket of `size`, as Go lists them: parents before children. */
function rounds(size: number, full: boolean): RoundView[] {
  const out: RoundView[] = [];
  let level = [{ from: 1, size }];
  let n = 0;
  while (level.length > 0) {
    const next: { from: number; size: number }[] = [];
    for (const node of level) {
      if (node.size < 2) continue;
      const matches = Array.from({ length: node.size / 2 }, () => ({ id: `M${++n}` }) as MatchView);
      out.push({ from: node.from, to: node.from + node.size - 1, size: node.size, matches });
      next.push({ from: node.from, size: node.size / 2 });
      if (full) next.push({ from: node.from + node.size / 2, size: node.size / 2 });
    }
    level = next;
  }
  return out;
}

describe('bracketTree', () => {
  it('draws a full placement bracket of 32 on 5 columns of 16 matches', () => {
    const t = bracketTree(rounds(32, true));
    expect(t.columns.map((c) => c.size)).toEqual([32, 16, 8, 4, 2]);
    for (let c = 0; c < 5; c++) {
      expect(t.matches.filter((m) => m.col === c)).toHaveLength(16);
    }
    expect(t.rows).toBeCloseTo(15 * 1.75 + 1);
    // Every match but the finals sends its winner and its loser on.
    expect(t.links.filter((l) => l.kind === 'win')).toHaveLength(64);
    expect(t.links.filter((l) => l.kind === 'lose')).toHaveLength(64);
    expect(t.blocks.map((b) => [b.from, b.to])).toEqual(
      [1, 5, 9, 13, 17, 21, 25, 29].map((f) => [f, f + 3]),
    );
  });

  it('sends the winner up and the loser down', () => {
    const t = bracketTree(rounds(32, true));
    const row = (id: string) => t.matches.find((m) => m.match.id === id)!.row;
    for (const l of t.links.filter((x) => x.from === 'M1')) {
      expect(l.kind === 'win' ? row(l.to) < 8 : row(l.to) >= 8).toBe(true);
    }
    for (const l of t.links) {
      const from = t.matches.find((m) => m.match.id === l.from)!;
      const to = t.matches.find((m) => m.match.id === l.to)!;
      expect(to.col).toBe(from.col + 1);
    }
  });

  it('centres a match between the two feeding it without full placement', () => {
    const t = bracketTree(rounds(8, false));
    expect(t.columns.map((c) => c.size)).toEqual([8, 4, 2]);
    expect(t.links.every((l) => l.kind === 'win')).toBe(true);
    const rowOf = (c: number) => t.matches.filter((m) => m.col === c).map((m) => m.row);
    expect(rowOf(0)).toEqual([0, 1, 2, 3]);
    expect(rowOf(1)).toEqual([0.5, 2.5]);
    expect(rowOf(2)).toEqual([1.5]);
  });

  it('handles a bracket with no round', () => {
    const t = bracketTree([]);
    expect(t.matches).toEqual([]);
    expect(t.rows).toBe(0);
  });

  it('links the middle of a match without result to the team it feeds', () => {
    const g = treeGeometry(false);
    const t = bracketTree(rounds(4, false));
    const [semi1, semi2, final] = t.matches;
    const routes = routeLinks(g, t, t.links);
    const sideH = (g.cardH - g.headH) / 2;
    const one = routes.find((r) => r.link.from === semi1.match.id)!;
    const two = routes.find((r) => r.link.from === semi2.match.id)!;
    expect(one.d).toMatch(new RegExp(`^M${cardLeft(g, 0) + g.cardW} ${cardTop(g, semi1.row) + g.headH + sideH} `));
    // The first semi feeds the final's top team, the second its bottom one.
    expect(one.d).toMatch(new RegExp(` H${cardLeft(g, 1)}$`));
    expect(one.d).toContain(`${cardTop(g, final.row) + g.headH + sideH / 2} H`);
    expect(two.d).toContain(`${cardTop(g, final.row) + g.headH + sideH * 1.5} H`);
    expect(treeGeometry(true).cardH).toBeGreaterThan(g.cardH);
  });

  it('links a decided match from the team it carries', () => {
    const g = treeGeometry(false);
    const r = rounds(4, true);
    const semi = { ...r[0].matches![0]!, score: { sets: [{ a: 10, b: 21 }] } } as unknown as Match;
    r[0].matches![0] = semi;
    const t = bracketTree(r);
    const sideH = (g.cardH - g.headH) / 2;
    const top = cardTop(g, t.matches.find((m) => m.match.id === semi.id)!.row) + g.headH;
    const from = (kind: string) => routeLinks(g, t, t.links).find((x) => x.link.from === semi.id && x.link.kind === kind)!.d;
    // B won: the winner's link leaves from B, the loser's from A.
    expect(from('win')).toMatch(new RegExp(`^M[\\d.]+ ${top + sideH * 1.5} `));
    expect(from('lose')).toMatch(new RegExp(`^M[\\d.]+ ${top + sideH / 2} `));
  });

  it('runs a link further out the lower it leaves', () => {
    const g = treeGeometry(false);
    const t = bracketTree(rounds(8, false));
    const firstRound = t.links.filter((l) => t.matches.find((m) => m.match.id === l.from)!.col === 0);
    const routes = routeLinks(g, t, firstRound);
    const start = (d: string) => Number(/^M[\d.]+ ([\d.]+)/.exec(d)![1]);
    const vertical = (d: string) => Number(/Q([\d.]+) /.exec(d)?.[1]);
    // Quarters 1 and 2 meet in semi 1: the lower one runs nearer its card (further left).
    const [q1, q2] = [routes[0], routes[1]].sort((a, b) => start(a.d) - start(b.d));
    expect(vertical(q2.d)).toBeLessThanOrEqual(vertical(q1.d));
  });

  it('runs links crossing the same gap on separate lanes', () => {
    const g = treeGeometry(false);
    const t = bracketTree(rounds(8, true));
    const vertical = (d: string) => Number(/Q([\d.]+) /.exec(d)?.[1]);
    const lanes = new Set(routeLinks(g, t, t.links.filter((l) => t.matches.find((m) => m.match.id === l.from)!.col === 0)).map((r) => vertical(r.d)));
    expect(lanes.size).toBeGreaterThan(1);
  });

  it('lays blocks out as a tree of their own, centred on the two they feed', () => {
    const t = bracketTree(rounds(8, true));
    expect(t.titled).toBe(true);
    const rowOf = (from: number, size: number) => {
      const first = rounds(8, true).find((r) => r.from === from && r.size === size)!.matches![0]!.id;
      return t.matches.find((m) => m.match.id === first)!.row;
    };
    // Finals (1–2, 3–4, 5–6, 7–8) evenly spaced, not on the grid of the first round.
    const finals = [1, 3, 5, 7].map((f) => rowOf(f, 2));
    expect(finals[1] - finals[0]).toBeGreaterThan(1);
    expect(finals[2] - finals[1]).toBeCloseTo(finals[1] - finals[0]);
    // Places 1–4 (two matches) sit between the finals 1–2 and 3–4.
    const semis = t.matches.filter((m) => m.col === 1).slice(0, 2).map((m) => m.row);
    expect((semis[0] + semis[1]) / 2).toBeCloseTo((finals[0] + finals[1]) / 2);
    expect(semis[1] - semis[0]).toBe(1);
    expect(Math.min(...t.matches.map((m) => m.row))).toBe(0);
    expect(t.nodes.filter((n) => n.col === 2).map((n) => [n.from, n.to])).toEqual([
      [1, 2],
      [3, 4],
      [5, 6],
      [7, 8],
    ]);
  });

  it('has no block title without full placement', () => {
    const t = bracketTree(rounds(8, false));
    expect(t.titled).toBe(false);
      });

  it('draws links with square runs', () => {
    const g = treeGeometry(false);
    const t = bracketTree(rounds(4, true));
    const routes = routeLinks(g, t, t.links);
    expect(routes.every((r) => /^M[\d.]+ [\d.]+ H[\d.]+( Q.* V.* Q.*)? H[\d.]+$/.test(r.d) || /^M[\d.]+ [\d.]+ H[\d.]+$/.test(r.d))).toBe(true);
  });
});
