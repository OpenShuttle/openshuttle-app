import { roundLabel, shortRoundLabel } from './match-stage';

describe('shortRoundLabel', () => {
  it.each([
    [1, 2, 2, 'round.short.final', {}],
    [1, 4, 4, 'round.short.semi', {}],
    [1, 8, 8, 'round.short.quarter', {}],
    [1, 16, 16, 'round.short.eighth', {}],
    [1, 32, 32, 'round.short.ofN', { n: 32 }],
    [3, 4, 2, 'round.short.place', { place: 3 }],
    [5, 6, 2, 'round.short.place', { place: 5 }],
    [5, 8, 4, 'round.short.places', { from: 5, to: 8 }],
  ])('places %i–%i, round of %i: %s', (placeFrom, placeTo, roundSize, key, params) => {
    expect(shortRoundLabel({ placeFrom, placeTo, roundSize })).toEqual({ key, params });
  });
});

describe('roundLabel', () => {
  it('names the main rounds', () => {
    expect(roundLabel({ placeFrom: 1, placeTo: 2, roundSize: 2 }).key).toBe('round.final');
    expect(roundLabel({ placeFrom: 1, placeTo: 4, roundSize: 4 }).key).toBe('round.semi');
    expect(roundLabel({ placeFrom: 1, placeTo: 8, roundSize: 8 }).key).toBe('round.quarter');
    expect(roundLabel({ placeFrom: 1, placeTo: 16, roundSize: 16 }).key).toBe('round.eighth');
    expect(roundLabel({ placeFrom: 1, placeTo: 32, roundSize: 32 })).toEqual({ key: 'round.ofN', params: { n: 32 } });
  });

  it('names placement matches', () => {
    expect(roundLabel({ placeFrom: 3, placeTo: 4, roundSize: 2 })).toEqual({ key: 'round.placeMatch', params: { place: 3 } });
    expect(roundLabel({ placeFrom: 5, placeTo: 8, roundSize: 4 })).toEqual({ key: 'round.places', params: { from: 5, to: 8 } });
  });
});
