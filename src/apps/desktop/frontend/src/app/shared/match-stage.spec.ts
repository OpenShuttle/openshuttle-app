import { roundLabel } from './match-stage';

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
