import { TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
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
});
