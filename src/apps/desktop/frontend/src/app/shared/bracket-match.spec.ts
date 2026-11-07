import { MatchView, PhasesView, Settings } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { render, text } from '../../testing/render';
import finished from '../../testing/fixtures/finished-phases.json';
import settings from '../../testing/fixtures/settings.json';
import { TranslocoService } from '@jsverse/transloco';

import { matchWinner } from './bracket-labels';
import { cardWidth } from './card-width';
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
    const f = await render(BracketMatch, { lang: 'fr', inputs: { match: quarter, origins: bracket.origins, size: 8 } });
    expect(text(f)).toContain('1/4 A');
    expect(text(f)).toContain(quarter.a.clubs![0]);
    const won = f.nativeElement.querySelector('.side.won app-result-icon .won');
    const lost = f.nativeElement.querySelector('.side.lost app-result-icon .lost');
    expect(won).not.toBeNull();
    expect(lost).not.toBeNull();
  });
});

describe('BracketMatch in hall mode', () => {
  beforeEach(() => {
    new FakeBackend().on('SettingsService.GetSettings', settings as unknown as Settings);
  });

  it('puts the winner on top, with the place won and no score', async () => {
    const final = match('MD-S2-9');
    const f = await render(BracketMatch, { inputs: { match: { ...final, a: final.b, b: final.a, score: { sets: final.score!.sets!.map((x) => ({ a: x.b, b: x.a })) } }, hall: true } });
    const sides = [...f.nativeElement.querySelectorAll('.side')] as HTMLElement[];
    expect(sides[0].classList).toContain('won');
    expect(sides[0].textContent).toContain('1st');
    expect(sides[1].textContent).toContain('2nd');
    expect(text(f)).not.toContain(`${final.score!.sets![0].a}-`);
    expect(f.nativeElement.querySelector('.club')).toBeNull();
  });

  it('puts the team facing a bye on top', async () => {
    const quarter = { ...match('MD-S2-2'), a: unknown, byeA: true, score: null };
    const f = await render(BracketMatch, { inputs: { match: quarter, hall: true } });
    const sides = [...f.nativeElement.querySelectorAll('.side')] as HTMLElement[];
    expect(sides[0].textContent).toContain(quarter.b.names![0]);
    expect(sides[1].textContent).toContain('Bye');
  });

  it('leaves forfeits out', async () => {
    const quarter = { ...match('MD-S2-2'), walkover: true, walkoverB: true, score: null };
    const f = await render(BracketMatch, { inputs: { match: quarter, hall: true } });
    expect(f.nativeElement.querySelector('.wo')).toBeNull();
    const g = await render(BracketMatch, { inputs: { match: quarter } });
    expect(g.nativeElement.querySelector('.wo')).not.toBeNull();
  });

  it('shows the pool and rank only where a team enters the bracket', async () => {
    const final = match('MD-S2-9');
    const f = await render(BracketMatch, { inputs: { match: final, origins: bracket.origins } });
    expect(f.nativeElement.querySelector('.origin')).toBeNull();
    const g = await render(BracketMatch, { inputs: { match: match('MD-S2-2'), origins: bracket.origins, size: 8 } });
    expect(g.nativeElement.querySelector('.origin')).not.toBeNull();
    // A team after a bye: no pool and rank in its first match played.
    const semi = match('MD-S2-5');
    const h = await render(BracketMatch, { inputs: { match: semi, origins: bracket.origins, size: 8 } });
    expect(semi.fromA).toBeNull();
    expect(h.nativeElement.querySelector('.origin')).toBeNull();
  });
});

describe('cardWidth', () => {
  it('fits the longest line, within bounds', async () => {
    const f = await render(BracketMatch, { inputs: { match: match('MD-S2-2') } });
    const t = f.debugElement.injector.get(TranslocoService);
    const all = bracket.rounds!.flatMap((r) => r.matches!);
    const options = { bibs: false, clubs: false, scores: false, origins: true };
    const narrow = cardWidth(t, all, options);
    expect(narrow).toBeGreaterThanOrEqual(150);
    expect(narrow).toBeLessThanOrEqual(260);
    const long = { ...all[0], a: { ...all[0].a, names: ['A very long first player name', 'and an even longer partner'] } };
    expect(cardWidth(t, [long], options)).toBe(260);
    expect(cardWidth(t, [], options)).toBe(150);
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
