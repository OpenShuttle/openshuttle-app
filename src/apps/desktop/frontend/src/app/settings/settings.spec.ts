import { Presets, Settings } from '@bindings/apps/desktop';

import { FakeBackend, backendFailure } from '../../testing/backend';
import { choose, click, clickAll, render, text } from '../../testing/render';
import presets from '../../testing/fixtures/presets.json';
import settings from '../../testing/fixtures/settings.json';
import { SettingsPage } from './settings';

describe('SettingsPage', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('SettingsService.GetSettings', { ...settings, operator: 'Julie' } as unknown as Settings)
      .on('PresetService.GetPresets', presets as unknown as Presets)
      .on('DisplayScreenService.ListMonitors', [{ id: 'm1', name: 'HDMI-1', width: 1920, height: 1080, primary: false }])
      .on('DisplayScreenService.IsOpen', false)
      .on('SettingsService.ChooseTournamentsDir', '/home/julie/Tournois');
  });

  it('shows the settings and changes each one', async () => {
    const f = await render(SettingsPage);
    expect(text(f)).toContain('/home/julie/Documents/OpenShuttle');
    await clickAll(f);
    for (const name of ['SetLanguage', 'SetDefaultCourts', 'SetDefaultMinRest', 'SetRotationSeconds', 'ChooseTournamentsDir']) {
      expect(go.called(`SettingsService.${name}`)).toBe(true);
    }
  });

  it('looks for a new version at startup, or not, and checks on demand', async () => {
    go.on('AppService.GetAppInfo', { version: '1.2.0', compat: 1 });
    const f = await render(SettingsPage);
    expect(text(f)).toContain('Not checked yet.');
    const seg = () => [...f.nativeElement.querySelectorAll('.seg-option')] as HTMLButtonElement[];
    seg().find((b) => b.textContent!.trim() === 'No')!.click();
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('SettingsService.SetCheckUpdates')).toEqual([[false]]);

    go.on('AppService.CheckForUpdate', { checked: true, latest: '1.3.0', available: true });
    await click(f, 'Check now');
    expect(text(f)).toContain('Version 1.3.0 is available.');
    expect(text(f)).toContain('Download');

    go.on('AppService.CheckForUpdate', { checked: true, latest: '1.2.0', available: false });
    await click(f, 'Check now');
    expect(text(f)).toContain('You have the latest version (1.2.0).');

    go.on('AppService.CheckForUpdate', () => {
      throw backendFailure('UPDATE_CHECK_FAILED');
    });
    await click(f, 'Check now');
    expect(text(f)).toContain('no internet connection?');
  });

  it('shows clubs by name or by acronym', async () => {
    const f = await render(SettingsPage);
    const seg = [...f.nativeElement.querySelectorAll('.seg-option')] as HTMLButtonElement[];
    const byAcronym = seg.find((b) => b.textContent!.trim() === 'Acronym')!;
    expect(seg.find((b) => b.textContent!.trim() === 'Name')!.classList).toContain('active');
    byAcronym.click();
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('SettingsService.SetClubDisplay')).toEqual([['acronym']]);
  });

  it('chooses the defaults, the monitor and the operator', async () => {
    const f = await render(SettingsPage);
    for (const select of [...f.nativeElement.querySelectorAll('select')] as HTMLSelectElement[]) {
      const last = select.options[select.options.length - 1];
      await choose(f, select, last.value);
    }
    const operator = f.nativeElement.querySelector('input[type=text], input:not([type])') as HTMLInputElement;
    operator.value = 'Marc';
    operator.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('SettingsService.SetOperator')).toEqual([['Marc']]);
    expect(go.called('SettingsService.SetDefaultPreset')).toBe(true);
  });
});
