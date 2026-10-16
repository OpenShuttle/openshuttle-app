import { scoreFrom } from './score-panel';

describe('scoreFrom', () => {
  it('keeps the sets typed, in order', () => {
    expect(scoreFrom([['21', '19', ''], ['10', '21', '']], false)).toEqual({
      sets: [
        { a: 21, b: 10 },
        { a: 19, b: 21 },
      ],
      goldenPoint: false,
    });
  });

  it('reads a half-filled set as 0', () => {
    expect(scoreFrom([['21'], ['']], false).sets).toEqual([{ a: 21, b: 0 }]);
  });

  it('carries the golden point', () => {
    expect(scoreFrom([['16'], ['15']], true)).toEqual({ sets: [{ a: 16, b: 15 }], goldenPoint: true });
  });
});
