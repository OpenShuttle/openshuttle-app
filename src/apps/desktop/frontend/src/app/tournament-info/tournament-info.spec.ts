import { TournamentView } from '@bindings/apps/desktop';

import { FakeBackend, backendFailure } from '../../testing/backend';
import { clickAll, find, render, text } from '../../testing/render';
import tournament from '../../testing/fixtures/before-tournament.json';
import { TournamentStore } from '../core/tournament.service';
import { TournamentInfo } from './tournament-info';

describe('TournamentInfo', () => {
  let go: FakeBackend;

  beforeEach(() => {
    localStorage.clear();
    go = new FakeBackend().on('TournamentService.GetTournamentView', tournament as unknown as TournamentView);
  });

  it('renames the tournament and changes courts, rest and categories', async () => {
    const f = await render(TournamentInfo);
    await f.debugElement.injector.get(TournamentStore).refresh();
    f.detectChanges();
    expect(text(f)).toContain('Courts');
    const name = find(f, 'input') as HTMLInputElement;
    name.value = 'Plume 2027';
    name.dispatchEvent(new Event('input'));
    name.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('TournamentService.RenameTournament')).toEqual([['Plume 2027']]);
    name.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await clickAll(f, ['Close tournament']);
    expect(go.called('TournamentService.SetCourts')).toBe(true);
    expect(go.called('TournamentService.SetMinRest')).toBe(true);
  });

  it('sets the date and hours, and puts back a refused one', async () => {
    go.on('TournamentService.GetTournamentView', { ...tournament, day: { date: '2026-11-14', start: '09:00' } } as unknown as TournamentView);
    const f = await render(TournamentInfo);
    await f.debugElement.injector.get(TournamentStore).refresh();
    f.detectChanges();
    const input = (name: string) => find(f, `[data-day] input[name=${name}]`) as HTMLInputElement;
    expect(input('date').value).toBe('2026-11-14');
    input('end').value = '19:00';
    input('end').dispatchEvent(new Event('change'));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('TournamentService.SetDay')).toEqual([['2026-11-14', '09:00', '19:00']]);
    go.on('TournamentService.SetDay', () => {
      throw backendFailure('INVALID_TIME');
    });
    input('start').value = '20:00';
    input('start').dispatchEvent(new Event('change'));
    await new Promise((r) => setTimeout(r, 10));
    expect(input('start').value).toBe('09:00');
  });
});
