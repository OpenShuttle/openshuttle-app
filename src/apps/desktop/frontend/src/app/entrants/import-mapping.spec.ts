import { ImportFile } from '@bindings/apps/desktop';
import { ImportMapping as Mapping, ImportPreview } from '@bindings/internal/tournament';

import { FakeBackend } from '../../testing/backend';
import { buttons, choose, click, find, render, text } from '../../testing/render';
import { ImportMapping, columnLetter } from './import-mapping';

const helloAsso: ImportFile = {
  path: 'C:\\Users\\julie\\Downloads\\export-helloasso.csv',
  lines: [
    { line: 1, cells: ['Nom participant', 'Prénom participant', 'Tarif', 'Club', 'Nom et prénom du partenaire'] },
    { line: 2, cells: ['Zola', 'Paul', 'Double hommes', 'BC Lyon', 'Hugo Écuyer'] },
    { line: 3, cells: ['Roux', 'Léa', 'Double dames', 'ASPTT', 'Zoé Petit'] },
    { line: 4, cells: ['Durand', 'Jo', 'Repas', '', ''] },
  ],
  mapping: {
    header: true,
    category: 2,
    categories: {},
    fallback: 'MD',
    players: [
      { name: -1, first: 1, last: 0, club: 3 },
      { name: 4, first: -1, last: -1, club: -1 },
    ],
  } as Mapping,
};

const preview: ImportPreview = {
  report: { added: 2, skipped: [{ line: 4, code: 'CATEGORY_NOT_FOUND' }], ignored: 0 },
  entrants: [
    { id: 'e9.1', category: 'MD', players: [{ id: 'p9.1.1', name: 'Paul Zola', club: 'BC Lyon' }, { id: 'p9.1.2', name: 'Hugo Écuyer', club: '' }] },
    { id: 'e9.2', category: 'WD', players: [{ id: 'p9.2.1', name: 'Léa Roux', club: 'ASPTT' }, { id: 'p9.2.2', name: 'Zoé Petit', club: '' }] },
  ],
  values: [
    { value: 'Double hommes', category: 'MD', lines: 1 },
    { value: 'Double dames', category: 'WD', lines: 1 },
    { value: 'Repas', category: '', lines: 1 },
  ],
} as ImportPreview;

describe('ImportMapping', () => {
  let go: FakeBackend;

  const mappingOf = (call = -1): Mapping => go.argsOf('TournamentService.PreviewImport').at(call)![1] as Mapping;

  const show = () =>
    render(ImportMapping, {
      inputs: {
        file: helloAsso,
        categories: [
          { category: 'MD', slot: 1 },
          { category: 'WD', slot: 2 },
        ],
      },
    });

  beforeEach(() => {
    go = new FakeBackend().on('TournamentService.PreviewImport', preview);
  });

  it('shows the guess, the first lines and what Go would import', async () => {
    const f = await show();
    expect(text(f)).toContain('Import export-helloasso.csv');
    expect(text(f)).toContain('3 lines');
    expect(text(f)).toContain('C · Tarif');
    expect(text(f)).toContain('Paul Zola');
    expect(text(f)).toContain('2 entrants to import');
    expect(text(f)).toContain('Line 4 · category not played');
    expect(mappingOf()).toEqual(helloAsso.mapping);
    // Values Go could not read wait for a choice.
    const repas = find(f, 'select[aria-label="Repas"]') as HTMLSelectElement;
    expect(repas.value).toBe('?');
  });

  it('sends every change of the mapping to the preview', async () => {
    const f = await show();
    await choose(f, find(f, 'select[aria-label="Repas"]') as HTMLSelectElement, '');
    expect(mappingOf().categories).toEqual({ Repas: '' });

    const header = find(f, '.head input[type=checkbox]') as HTMLInputElement;
    header.checked = false;
    header.dispatchEvent(new Event('change'));
    await new Promise((r) => setTimeout(r));
    expect(mappingOf().header).toBe(false);
    expect(text(f)).toContain('Column A');

    await click(f, 'Full name');
    expect(mappingOf().players![0]).toEqual({ name: -1, first: -1, last: -1, club: 3 });
    await click(f, 'First and last name');

    const selects = () => buttons(f, 'select') as HTMLSelectElement[];
    await choose(f, selects()[0], '-1'); // no category column
    expect(mappingOf().category).toBe(-1);
    expect(text(f)).toContain('Every line goes to');
    await choose(f, selects()[1], 'WD');
    expect(mappingOf().fallback).toBe('WD');

    const club2 = selects().at(-1)!;
    await choose(f, club2, '-2');
    expect(mappingOf().players![1].club).toBe(-2);

    const pairs = buttons(f, '.group input[type=checkbox]')[0] as HTMLInputElement;
    pairs.checked = false;
    pairs.dispatchEvent(new Event('change'));
    await new Promise((r) => setTimeout(r));
    expect(mappingOf().players!.length).toBe(1);
  });

  it('imports with the mapping shown, or cancels', async () => {
    go.on('TournamentService.ImportEntrants', { added: 2, skipped: [], ignored: 1 });
    const f = await show();
    const done: unknown[] = [];
    f.componentInstance.done.subscribe((r) => done.push(r));
    await click(f, 'Import 2 entrants');
    expect(go.argsOf('TournamentService.ImportEntrants')[0]).toEqual([helloAsso.path, helloAsso.mapping]);
    await click(f, 'Cancel');
    expect(done).toEqual([{ added: 2, skipped: [], ignored: 1 }, null]);
  });

  it('names columns like a spreadsheet', () => {
    expect([0, 25, 26, 27, 701, 702].map(columnLetter)).toEqual(['A', 'Z', 'AA', 'AB', 'ZZ', 'AAA']);
  });
});
