import { EntrantsView, Presets, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { buttons, choose, click, clickAll, find, render, text, type } from '../../testing/render';
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
    const names = [...f.nativeElement.querySelectorAll('form.add input.input')] as HTMLInputElement[];
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

  it('registers a player waiting for a partner', async () => {
    const f = await render(Entrants);
    const alone = find(f, 'form.add input[type=checkbox]') as HTMLInputElement;
    alone.checked = true;
    alone.dispatchEvent(new Event('change'));
    await new Promise((r) => setTimeout(r));
    f.detectChanges();
    const names = [...f.nativeElement.querySelectorAll('form.add input.input')] as HTMLInputElement[];
    expect(names.length).toBe(1);
    await type(f, names[0], 'Julie Morel');
    (find(f, 'form.add') as HTMLFormElement).dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    const [, players] = go.argsOf('TournamentService.RegisterEntrant')[0] as [string, { name: string }[]];
    expect(players.map((p) => p.name)).toEqual(['Julie Morel']);
  });

  it('shows the players waiting for a partner', async () => {
    const f = await render(Entrants);
    expect(text(f)).toContain('2 waiting for a partner');
    expect(text(f)).toContain('Waiting for a partner');
    await click(f, 'Partner…');
    expect(text(f)).toContain('Nobody else is waiting for a partner in this category.');
  });

  it('pairs two players waiting, or adds a partner', async () => {
    const view = structuredClone(entrants) as unknown as EntrantsView;
    const wd = view.categories!.find((c) => c.category === 'WD')!;
    const julie = wd.entrants!.find((e) => e.waiting)!;
    wd.entrants!.push({ ...julie, id: 'e900', players: [{ ...julie.players![0], id: 'p900.1', name: 'Zoé Petit', checkedIn: false }] });
    wd.waiting = 2;
    go.on('TournamentService.GetEntrantsView', view);
    const f = await render(Entrants);
    const row = [...f.nativeElement.querySelectorAll('.row')].find((r) => (r as HTMLElement).textContent?.includes('Julie Morel')) as HTMLElement;
    (row.querySelector('button') as HTMLButtonElement).click();
    await new Promise((r) => setTimeout(r));
    f.detectChanges();
    await choose(f, find(f, '.partner select') as HTMLSelectElement, 'e900');
    await click(f, 'Form the pair');
    expect(go.argsOf('TournamentService.PairEntrants')).toEqual([[julie.id, 'e900']]);

    await click(f, 'Partner…');
    await type(f, find(f, '.partner input') as HTMLInputElement, 'Mia Roux');
    await click(f, 'Add the partner');
    const [id, partner] = go.argsOf('TournamentService.AddPartner')[0] as [string, { name: string }];
    expect(id).toBeTruthy();
    expect(partner.name).toBe('Mia Roux');
  });

  it('splits a pair', async () => {
    const f = await render(Entrants);
    await click(f, 'Edit');
    await click(f, 'Split the pair');
    const pair = entrants.categories[0].entrants![0];
    expect(go.argsOf('TournamentService.SplitEntrant')).toEqual([[pair.id, pair.players![1].id]]);
  });

  it('offers no removal once the pair has matches', async () => {
    const locked = structuredClone(entrants) as unknown as EntrantsView;
    locked.categories![0].entrants![0].locked = true;
    go.on('TournamentService.GetEntrantsView', locked);
    const f = await render(Entrants);
    await click(f, 'Edit');
    expect(text(f)).toContain('can no longer be removed or split');
    expect(buttons(f).map((b) => b.textContent?.trim())).not.toContain('Remove');
    expect(buttons(f).map((b) => b.textContent?.trim())).not.toContain('Split the pair');
    expect(buttons(f).map((b) => b.textContent?.trim())).toContain('Save');
  });

  it('runs every button', async () => {
    await clickAll(await render(Entrants), ['Remove']);
  });
});
