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
    expect(t.rows).toBe(16);
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
    expect(linkPath(g, t.gaps, a, b)).toContain(
      `M${cardLeft(g, 0) + g.cardW} ${cardTop(g, t.gaps, a.row) + g.cardH / 2}`,
    );
    expect(treeGeometry(true).cardH).toBeGreaterThan(g.cardH);
  });

  it('sets blocks apart, the coarser the wider, and titles them', () => {
    const t = bracketTree(rounds(8, true));
    expect(t.titled).toBe(true);
    // Last column: finals of 1–2, 3–4, 5–6, 7–8, each opening a gap; the 5–8 block's is wider.
    expect(t.gaps.map((g) => [g.row, g.level])).toEqual([
      [1, 1],
      [2, 2],
      [3, 1],
    ]);
    const g = treeGeometry(false, true);
    expect(cardTop(g, t.gaps, 2) - cardTop(g, t.gaps, 1)).toBeGreaterThan(cardTop(g, t.gaps, 1) - cardTop(g, t.gaps, 0));
    expect(t.nodes.filter((n) => n.col === 2).map((n) => [n.from, n.to])).toEqual([
      [1, 2],
      [3, 4],
      [5, 6],
      [7, 8],
    ]);
  });

  it('has no gap or title without full placement', () => {
    const t = bracketTree(rounds(8, false));
    expect(t.titled).toBe(false);
    expect(t.gaps).toEqual([]);
  });
});
