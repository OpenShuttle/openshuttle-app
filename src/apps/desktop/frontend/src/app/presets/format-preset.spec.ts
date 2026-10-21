import { Presets, Settings } from '@bindings/apps/desktop';
import { FormatPreview } from '@bindings/internal/tournament';

import { FakeBackend } from '../../testing/backend';
import { click, clickAll, find, render, text, type } from '../../testing/render';
import presets from '../../testing/fixtures/presets.json';
import preview from '../../testing/fixtures/preview.json';
import settings from '../../testing/fixtures/settings.json';
import { FormatPresetPage } from './format-preset';

describe('FormatPresetPage', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('PresetService.GetPresets', presets as unknown as Presets)
      .on('SettingsService.GetSettings', settings as unknown as Settings)
      .on('PresetService.PreviewFormat', preview as unknown as FormatPreview)
      .on('PresetService.EstimateMinutes', 22);
  });

  it('shows a built-in format read only, to duplicate', async () => {
    const f = await render(FormatPresetPage, { inputs: { id: presets.formats[0].id } });
    expect(go.called('PresetService.PreviewFormat')).toBe(true);
    await click(f, 'Duplicate');
    expect(go.called('PresetService.DuplicatePreset')).toBe(true);
  });

  it('creates a new format and simulates it for more teams', async () => {
    const f = await render(FormatPresetPage, { inputs: { id: 'new' } });
    await type(f, find(f, 'input') as HTMLInputElement, 'Poules et tableau');
    await clickAll(f, ['Remove']);
    await click(f, 'Save');
    expect(go.called('PresetService.SaveFormatPreset')).toBe(true);
    expect(text(f)).toBeTruthy();
  });
});
