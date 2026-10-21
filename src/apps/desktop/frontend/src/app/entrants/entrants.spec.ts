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

  it('maps the columns of a chosen list, then imports it', async () => {
    const file = {
      path: '/home/julie/inscriptions.csv',
      lines: [
        { line: 1, cells: ['Tableau', 'Joueur', 'Partenaire'] },
        { line: 2, cells: ['DH', 'Paul Zola', 'Hugo Écuyer'] },
      ],
      mapping: { header: true, category: 0, categories: {}, fallback: 'MD', players: [{ name: 1, first: -1, last: -1, club: -1 }, { name: 2, first: -1, last: -1, club: -1 }] },
    };
    go.on('TournamentService.ChooseImportFile', file);
    go.on('TournamentService.PreviewImport', { report: { added: 1, skipped: [], ignored: 0 }, entrants: [], values: [] });
    go.on('TournamentService.ImportEntrants', { added: 1, skipped: [{ line: 4, code: 'ALREADY_ENTERED' }], ignored: 2 });
    go.on('TournamentService.ExportCSV', '/home/julie/joueurs.csv');
    const f = await render(Entrants);
    await click(f, 'Import…');
    expect(text(f)).toContain('Import inscriptions.csv');
    await click(f, 'Import 1 entrant');
    expect(go.argsOf('TournamentService.ImportEntrants')[0]).toEqual([file.path, file.mapping]);
    expect(text(f)).toContain('1 entrant imported');
    expect(text(f)).toContain('Line 4 · already entered');
    expect(text(f)).toContain('2 lines ignored');
    expect(text(f)).not.toContain('Import inscriptions.csv');
    await click(f, 'Export');
    expect(text(f)).toContain('/home/julie/joueurs.csv');
  });

  it('cancels a chosen list', async () => {
    go.on('TournamentService.ChooseImportFile', { path: '/x.csv', lines: [], mapping: { header: false, category: -1, categories: {}, fallback: 'MD', players: [] } });
    const f = await render(Entrants);
    await click(f, 'Import…');
    await click(f, 'Cancel');
    expect(text(f)).not.toContain('Import x.csv');
    expect(go.called('TournamentService.ImportEntrants')).toBe(false);
  });

  it('runs every button', async () => {
    await clickAll(await render(Entrants), ['Remove']);
  });
});
