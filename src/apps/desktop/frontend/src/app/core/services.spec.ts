import { TestBed } from '@angular/core/testing';
import { Presets, Settings } from '@bindings/apps/desktop';

import { FakeBackend, backendFailure } from '../../testing/backend';
import presets from '../../testing/fixtures/presets.json';
import settings from '../../testing/fixtures/settings.json';
import { Clock } from './clock';
import { DisplayScreenService } from './display-screen.service';
import { Panels } from './panels.service';
import { AppPresets } from './presets.service';
import { AppSettings } from './settings.service';

const tick = () => new Promise((r) => setTimeout(r));

describe('core services', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('SettingsService.GetSettings', settings as unknown as Settings)
      .on('PresetService.GetPresets', presets as unknown as Presets);
    TestBed.configureTestingModule({});
  });

  it('AppSettings loads, follows settings.changed and reports errors', async () => {
    const s = TestBed.inject(AppSettings);
    await tick();
    expect(s.value()?.language).toBe(settings.language);
    go.emit('settings.changed', { ...settings, language: 'fr' });
    expect(s.value()?.language).toBe('fr');
    expect(await s.run(() => Promise.resolve())).toBe(true);
    expect(await s.run(() => Promise.reject(backendFailure('INVALID_SETTING')))).toBe(false);
    expect(s.error()).toEqual({ code: 'INVALID_SETTING' });
  });

  it('AppPresets offers the enabled presets and the defaults', async () => {
    const p = TestBed.inject(AppPresets);
    await tick();
    expect(p.value().formats?.length).toBeGreaterThan(0);
    expect(p.defaultWin().kind).toBeTruthy();
    expect(p.defaultTiebreaks().length).toBeGreaterThan(0);
    const first = p.value().formats![0].id;
    go.emit('presets.changed', { ...presets, disabled: [first] });
    expect(p.enabled(first)).toBe(false);
    expect(p.offered().formats?.some((f) => f.id === first)).toBe(false);
    expect(await p.run(() => Promise.reject(backendFailure('PRESET_READ_ONLY')))).toBe(false);
    expect(p.error()).toEqual({ code: 'PRESET_READ_ONLY' });
    expect(await p.run(() => Promise.resolve())).toBe(true);
  });

  it('AppPresets falls back to 2 sets of 15 without presets', async () => {
    go.on('PresetService.GetPresets', { win: [], tiebreaks: [], formats: [], disabled: [], clubs: [] });
    const p = TestBed.inject(AppPresets);
    await tick();
    expect(p.defaultWin()).toEqual({ kind: 'sets', setsToWin: 2, pointsPerSet: 15, cap: 21 });
    expect(p.defaultTiebreaks()).toEqual([]);
  });

  it('DisplayScreenService opens, closes and follows the window', async () => {
    go.on('DisplayScreenService.IsOpen', true).on('DisplayScreenService.ListMonitors', [{ id: 'm1', name: 'HDMI' }]);
    const d = TestBed.inject(DisplayScreenService);
    await d.refresh();
    expect(d.isOpen()).toBe(true);
    expect(d.monitors()).toHaveLength(1);
    go.emit('displayScreen.changed', false);
    expect(d.isOpen()).toBe(false);
    await d.open();
    await d.close();
    expect(go.argsOf('DisplayScreenService.Open')).toEqual([['']]);
    expect(go.called('DisplayScreenService.Close')).toBe(true);
  });

  it('Panels fold and remember', () => {
    localStorage.clear();
    const p = TestBed.inject(Panels);
    expect(p.collapsed('nav')).toBe(false);
    p.toggle('nav');
    expect(p.collapsed('nav')).toBe(true);
    expect(JSON.parse(localStorage.getItem('openshuttle.panels')!)).toEqual({ nav: true });
  });

  it('Clock ticks every second', () => {
    vi.useFakeTimers();
    const c = TestBed.inject(Clock);
    const before = c.now().getTime();
    vi.advanceTimersByTime(2000);
    expect(c.now().getTime()).toBeGreaterThan(before);
    vi.useRealTimers();
  });
});
