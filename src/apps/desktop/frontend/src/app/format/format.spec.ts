import { FormatView, Presets, Settings, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { choose, click, clickAll, find, render, text, type } from '../../testing/render';
import format from '../../testing/fixtures/before-format.json';
import presets from '../../testing/fixtures/presets.json';
import locked from '../../testing/fixtures/progress-format.json';
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

  it('shows a locked format read only', async () => {
    go.on('TournamentService.GetFormatView', locked as unknown as FormatView);
    const f = await render(Format);
    expect(text(f)).toContain('Locked');
  });
});
