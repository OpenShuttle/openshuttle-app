import { Presets, Settings } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { click, clickAll, find, render, text, type } from '../../testing/render';
import presets from '../../testing/fixtures/presets.json';
import settings from '../../testing/fixtures/settings.json';
import { PresetsPage } from './presets';

describe('Presets', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('PresetService.GetPresets', presets as unknown as Presets)
      .on('SettingsService.GetSettings', settings as unknown as Settings)
      .on('PresetService.EstimateMinutes', 22);
  });

  it('lists the formats, win conditions, tiebreak orders and clubs', async () => {
    const f = await render(PresetsPage);
    expect(text(f)).toContain('2 sets to win of 15');
  });

  it('creates a win condition preset', async () => {
    const f = await render(PresetsPage);
    const buttons = [...f.nativeElement.querySelectorAll('button')] as HTMLButtonElement[];
    buttons.find((b) => b.textContent?.trim() === '+ New')!.click();
    await new Promise((r) => setTimeout(r));
    f.detectChanges();
    const form = find(f, 'form') as HTMLFormElement;
    await type(f, form.querySelector('input') as HTMLInputElement, 'Rapide');
    form.dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('PresetService.SaveWinPreset')[0]?.[1]).toBe('Rapide');
  });

  it('runs every button', async () => {
    const f = await render(PresetsPage);
    await clickAll(f);
    for (const box of [...f.nativeElement.querySelectorAll('input[type=checkbox]')] as HTMLInputElement[]) {
      box.click();
    }
    expect(go.called('PresetService.DuplicatePreset')).toBe(true);
  });
});
