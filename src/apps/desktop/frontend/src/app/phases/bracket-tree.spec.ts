import { MatchView, RoundView } from '@bindings/apps/desktop';

import { bracketTree, cardLeft, cardTop, linkPath, treeGeometry } from './bracket-tree';

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

  it('links the middle of two cards', () => {
    const g = treeGeometry(false);
    const t = bracketTree(rounds(4, true));
    const [a, b] = [t.matches[0], t.matches[2]];
    expect(linkPath(g, a, b)).toContain(
      `M${cardLeft(g, 0) + g.cardW} ${cardTop(g, a.row) + g.cardH / 2}`,
    );
    expect(treeGeometry(true).cardH).toBeGreaterThan(g.cardH);
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
});
