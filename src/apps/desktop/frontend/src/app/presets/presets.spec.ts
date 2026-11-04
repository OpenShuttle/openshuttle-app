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

  it('lists clubs with their acronym, adds and edits one', async () => {
    const f = await render(PresetsPage);
    expect(text(f)).toContain('BC Rive DroiteBCRD');
    const clubs = () =>
      ([...f.nativeElement.querySelectorAll('h2.list-title')] as HTMLElement[]).find((h) => h.textContent!.trim() === 'Clubs')!.closest('section') as HTMLElement;
    (clubs().querySelector('.list-head button') as HTMLButtonElement).click();
    await new Promise((r) => setTimeout(r));
    f.detectChanges();
    let [name, acronym] = [...clubs().querySelectorAll('form input')] as HTMLInputElement[];
    await type(f, name, 'Rillieux');
    await type(f, acronym, 'AUVR');
    clubs().querySelector('form')!.dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('PresetService.SaveClub')).toEqual([['', { name: 'Rillieux', acronym: 'AUVR' }]]);

    const edit = [...clubs().querySelectorAll('.row')].find((r) => r.textContent!.includes('BCRD'))!.querySelector('button') as HTMLButtonElement;
    edit.click();
    await new Promise((r) => setTimeout(r));
    f.detectChanges();
    [name, acronym] = [...clubs().querySelectorAll('form input')] as HTMLInputElement[];
    expect([name.value, acronym.value]).toEqual(['BC Rive Droite', 'BCRD']);
    await type(f, acronym, '');
    clubs().querySelector('form')!.dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('PresetService.SaveClub')[1]).toEqual(['BC Rive Droite', { name: 'BC Rive Droite', acronym: '' }]);
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
