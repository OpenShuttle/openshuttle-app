import { EntrantsView, Presets, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { click, clickAll, find, render, text, type } from '../../testing/render';
import entrants from '../../testing/fixtures/before-entrants.json';
import tournament from '../../testing/fixtures/before-tournament.json';
import presets from '../../testing/fixtures/presets.json';
import { Entrants } from './entrants';

describe('Entrants', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', tournament as unknown as TournamentView)
      .on('TournamentService.GetEntrantsView', entrants as unknown as EntrantsView)
      .on('PresetService.GetPresets', presets as unknown as Presets);
  });

  it('lists the pairs with their clubs', async () => {
    const f = await render(Entrants);
    expect(text(f)).toContain(entrants.categories[0].entrants![0].players![0].name);
  });

  it('registers a pair', async () => {
    const f = await render(Entrants);
    const names = [...f.nativeElement.querySelectorAll('form input')] as HTMLInputElement[];
    await type(f, names[0], 'Anne Martin');
    await type(f, names[1], 'Rose Petit');
    (find(f, 'form') as HTMLFormElement).dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    const [category, players] = go.argsOf('TournamentService.RegisterEntrant')[0] as [string, { name: string }[]];
    expect(category).toBeTruthy();
    expect(players.map((p) => p.name)).toEqual(['Anne Martin', 'Rose Petit']);
  });

  it('edits and removes a pair', async () => {
    const f = await render(Entrants);
    await click(f, 'Edit');
    expect(text(f)).toContain('Save');
    await click(f, 'Remove');
    expect(go.called('TournamentService.RemoveEntrant')).toBe(true);
  });

  it('imports and exports the list', async () => {
    go.on('TournamentService.ImportEntrants', { added: 3, skipped: [{ line: 4, code: 'ENTRANT_EXISTS' }] });
    go.on('TournamentService.ExportCSV', '/home/julie/joueurs.csv');
    const f = await render(Entrants);
    await click(f, 'Import…');
    await click(f, 'Export');
    expect(go.called('TournamentService.ImportEntrants')).toBe(true);
    expect(text(f)).toContain('/home/julie/joueurs.csv');
  });

  it('runs every button', async () => {
    await clickAll(await render(Entrants), ['Remove']);
  });
});
