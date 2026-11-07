import { MatchView, RoundView } from '@bindings/apps/desktop';

import { foldBracket } from './split-bracket';

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

describe('foldBracket', () => {
  const at = (t: ReturnType<typeof foldBracket>['tree'], id: string) => t.matches.find((m) => m.match.id === id)!;

  it('folds a bracket of 8 around its final', () => {
    const { tree, parts } = foldBracket(rounds(8, false));
    expect(parts).toHaveLength(1);
    // Quarters 1-2 left, 3-4 right; semis around the final.
    expect([at(tree, 'M1').col, at(tree, 'M2').col, at(tree, 'M3').col, at(tree, 'M4').col]).toEqual([0, 0, 4, 4]);
    expect([at(tree, 'M3').row, at(tree, 'M4').row]).toEqual([at(tree, 'M1').row, at(tree, 'M2').row]);
    expect([at(tree, 'M5').col, at(tree, 'M6').col, at(tree, 'M7').col]).toEqual([1, 3, 2]);
    expect(at(tree, 'M5').row).toBe(at(tree, 'M6').row);
    expect(at(tree, 'M7').row).toBe(at(tree, 'M5').row);
    expect(tree.columns).toHaveLength(5);
    expect(tree.links.filter((l) => l.to === 'M7').map((l) => l.from).sort()).toEqual(['M5', 'M6']);
  });

  it('packs the places under the main draw, losers on their side', () => {
    const { tree, parts } = foldBracket(rounds(8, true));
    expect(parts.map((p) => [p.from, p.to])).toEqual([
      [1, 8],
      [5, 8],
    ]);
    // Go's order: QF 1-4, SF 5-6, 5-8 semis 7-8, final 9, 3rd 10, 5th 11, 7th 12.
    const final = at(tree, 'M9');
    const third = at(tree, 'M10');
    // The final and the 3rd place stacked between the semis, like the 5th and 7th places.
    expect(third.col).toBe(final.col);
    expect(third.row).toBe(final.row + 1);
    expect((final.row + third.row) / 2).toBe(at(tree, 'M5').row);
    expect(at(tree, 'M12').row).toBe(at(tree, 'M11').row + 1);
    // Places 5 to 8 under the semis.
    expect([at(tree, 'M7').col, at(tree, 'M8').col]).toEqual([at(tree, 'M5').col, at(tree, 'M6').col]);
    expect([at(tree, 'M11').col, at(tree, 'M12').col]).toEqual([final.col, final.col]);
    expect(at(tree, 'M11').row).toBe(third.row + 1.25);
    expect(tree.links.filter((l) => l.to === 'M12').every((l) => l.kind === 'lose')).toBe(true);
    // The losers of the main draw go on to the places below.
    expect(tree.links).toContainEqual({ from: 'M1', to: 'M7', kind: 'lose', side: 'a' });
    expect(tree.links).toContainEqual({ from: 'M6', to: 'M10', kind: 'lose', side: 'b' });
    expect(tree.links.filter((l) => l.kind === 'lose')).toHaveLength(8);
  });

  it.each([16, 32])('never overlaps two cards of a full placement bracket of %i', (size) => {
    const { tree } = foldBracket(rounds(size, true));
    expect(Math.min(...tree.matches.map((m) => m.row))).toBe(0);
    for (const a of tree.matches) {
      for (const b of tree.matches) {
        if (a !== b && a.col === b.col) {
          expect(Math.abs(a.row - b.row)).toBeGreaterThanOrEqual(1);
        }
      }
    }
    expect(tree.matches).toHaveLength((size / 2) * Math.log2(size));
  });

  it('sends the team of a match with a bye on, without a loser link', () => {
    const r = rounds(8, true);
    const quarter = r[0].matches![0]!;
    r[0].matches![0] = { ...quarter, byeB: true } as MatchView;
    const { tree } = foldBracket(r);
    expect(tree.links.filter((l) => l.from === quarter.id).map((l) => l.kind)).toEqual(['win']);
  });

  it('leaves out a match between two byes', () => {
    const r = rounds(8, true);
    // Go's order: QF 1-4, SF 5-6, 5-8 semis 7-8, final 9, 3rd 10, 5th 11, 7th 12.
    r[6].matches![0] = { ...r[6].matches![0]!, byeA: true, byeB: true } as MatchView;
    const { tree } = foldBracket(r);
    expect(tree.matches.some((m) => m.match.id === 'M12')).toBe(false);
    expect(tree.links.some((l) => l.to === 'M12' || l.from === 'M12')).toBe(false);
  });

  it('keeps a lone final as it is', () => {
    const { tree } = foldBracket(rounds(2, false));
    expect(tree.matches).toHaveLength(1);
    expect(tree.columns).toHaveLength(1);
  });
});
