import { MatchView, PhasesView, Settings } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { render, text } from '../../testing/render';
import finished from '../../testing/fixtures/finished-phases.json';
import settings from '../../testing/fixtures/settings.json';
import { matchWinner } from './bracket-labels';
import { BracketMatch } from './bracket-match';

const bracket = (finished as unknown as PhasesView).categories![0].stages!.find((s) => s.kind === 'bracket')!;
const match = (id: string): MatchView => bracket.rounds!.flatMap((r) => r.matches!).find((m) => m.id === id)!;
const unknown = { entrant: '', bib: 0, names: [], clubs: [], clubNames: [], withdrawn: false };

describe('BracketMatch', () => {
  beforeEach(() => {
    new FakeBackend().on('SettingsService.GetSettings', settings as unknown as Settings);
  });

  it('names the match and the sides still to come', async () => {
    // The final, before the semi-finals are played.
    const final = { ...match('MD-S2-9'), a: unknown, b: unknown, score: null };
    const f = await render(BracketMatch, { lang: 'fr', inputs: { match: final } });
    expect(text(f)).toContain('Finale');
    expect(text(f)).toContain('Vainqueur 1/2 A');
    expect(text(f)).toContain('Vainqueur 1/2 B');
    expect(f.nativeElement.querySelector('app-result-icon')).toBeNull();

    // The 5th place match: the losers of the quarter-finals, byes passed through.
    const fifth = { ...match('MD-S2-11'), a: unknown, b: unknown, score: null };
    const g = await render(BracketMatch, { inputs: { match: fifth } });
    expect(text(g)).toContain('Loser QF A');
    expect(text(g)).toContain('Loser QF B');
  });

  it('shows the clubs, a trophy for the winner and a cross for the loser', async () => {
    const quarter = match('MD-S2-2');
    const f = await render(BracketMatch, { lang: 'fr', inputs: { match: quarter, origins: bracket.origins } });
    expect(text(f)).toContain('1/4 A');
    expect(text(f)).toContain(quarter.a.clubs![0]);
    const won = f.nativeElement.querySelector('.side.won app-result-icon .won');
    const lost = f.nativeElement.querySelector('.side.lost app-result-icon .lost');
    expect(won).not.toBeNull();
    expect(lost).not.toBeNull();
  });
});

describe('matchWinner', () => {
  const m = (sets: [number, number][], walkoverA = false, walkoverB = false) =>
    ({ score: { sets: sets.map(([a, b]) => ({ a, b })) }, walkoverA, walkoverB }) as unknown as MatchView;

  it('reads the sets, then forfeits', () => {
    expect(matchWinner(m([[21, 10], [10, 21], [21, 19]]))).toBe('a');
    expect(matchWinner(m([[10, 21]]))).toBe('b');
    expect(matchWinner(m([]))).toBe('');
    expect(matchWinner(m([], true))).toBe('b');
    expect(matchWinner(m([], false, true))).toBe('a');
    expect(matchWinner(m([], true, true))).toBe('');
  });
});
