import { FormatView, Presets, Settings, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { choose, click, clickAll, find, render, text, type } from '../../testing/render';
import format from '../../testing/fixtures/before-format.json';
import presets from '../../testing/fixtures/presets.json';
import progress from '../../testing/fixtures/progress-format.json';
import settings from '../../testing/fixtures/settings.json';
import tournament from '../../testing/fixtures/before-tournament.json';
import { Format } from './format';

describe('Format', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', tournament as unknown as TournamentView)
      .on('TournamentService.GetFormatView', format as unknown as FormatView)
      .on('PresetService.GetPresets', presets as unknown as Presets)
      .on('SettingsService.GetSettings', settings as unknown as Settings)
      .on('PresetService.EstimateMinutes', 22);
  });

  it('draws the stage graph and the preview', async () => {
    const f = await render(Format);
    expect(f.nativeElement.querySelectorAll('.node').length).toBeGreaterThan(1);
    expect(text(f)).toContain('Tableau principal');
  });

  it('adds a stage and saves the format as a preset', async () => {
    const f = await render(Format);
    await click(f, 'Pools');
    expect(go.called('TournamentService.AddStage')).toBe(true);
    await click(f, 'Save as preset');
    const inputs = [...f.nativeElement.querySelectorAll('form input')] as HTMLInputElement[];
    await type(f, inputs[0], 'Mon format');
    (find(f, 'form') as HTMLFormElement).dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('TournamentService.SaveFormatPreset')[0]?.[1]).toBe('Mon format');
  });

  it('applies a preset', async () => {
    const f = await render(Format);
    const select = [...f.nativeElement.querySelectorAll('select')].find((s) => s.textContent?.includes('Start from a preset')) as HTMLSelectElement;
    await choose(f, select, presets.formats[0].id);
    expect(go.argsOf('TournamentService.ApplyFormatPreset')).toEqual([['MD', presets.formats[0].id]]);
  });

  it('edits the selected stage with every control', async () => {
    const f = await render(Format);
    await clickAll(f, ['Remove']);
    expect(go.called('TournamentService.SetFormat')).toBe(true);
    for (const node of [...f.nativeElement.querySelectorAll('.node')] as HTMLElement[]) {
      node.click();
    }
    await clickAll(f, ['Remove']);
  });

  it('previews the format for complete pairs, all entries or present ones', async () => {
    localStorage.removeItem('openshuttle.formatPreviewBase');
    const f = await render(Format);
    // MD is drawn: its pools are known, no base to choose.
    expect(text(f)).not.toContain('Complete pairs');
    await click(f, 'Women\'s doubles · 8');
    expect(text(f)).toContain('Complete pairs (8)');
    expect(text(f)).toContain('All entries (9)');
    expect(text(f)).toContain('Present (6)');
    expect(text(f)).toContain('For 8 entrants');
    await click(f, 'All entries (9)');
    expect(text(f)).toContain('For 9 entrants');
    expect(text(f)).toContain("Women's doubles · 9");
    expect(localStorage.getItem('openshuttle.formatPreviewBase')).toBe('registered');
    await click(f, 'Present (6)');
    expect(text(f)).toContain('For 6 entrants');
    localStorage.removeItem('openshuttle.formatPreviewBase');
  });

  it('freezes a started stage and keeps the later ones editable', async () => {
    go.on('TournamentService.GetFormatView', progress as unknown as FormatView);
    const f = await render(Format);
    const nodes = [...f.nativeElement.querySelectorAll('.node:not(.output)')] as HTMLElement[];
    expect(nodes[0].textContent).toContain('started');
    expect(nodes[1].textContent).not.toContain('started');
    // The draw is confirmed: no preset replaces the format, stages can still be added.
    expect(text(f)).not.toContain('Start from a preset');
    expect(text(f)).toContain('+ Bracket');

    nodes[0].click();
    f.detectChanges();
    expect(text(f)).toContain('Matches of this phase have started');
    const groups = [...f.nativeElement.querySelectorAll('fieldset.group')] as HTMLFieldSetElement[];
    expect(groups.length).toBeGreaterThan(0);
    expect(groups.every((g) => g.disabled)).toBe(true);

    nodes[1].click();
    f.detectChanges();
    expect(text(f)).not.toContain('Matches of this phase have started');
    const win = find(f, 'fieldset.group') as HTMLFieldSetElement;
    expect(win.disabled).toBe(false);
    (win.querySelector('.stepper button:enabled') as HTMLButtonElement).click();
    await new Promise((r) => setTimeout(r));
    expect(go.called('TournamentService.SetFormat')).toBe(true);
  });
});
