import { Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import {
  CheckInView,
  ControlDeskView,
  EntrantsView,
  FormatView,
  PhasesView,
  Presets,
  ResultsView,
  Settings,
  TournamentView,
} from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { FakeBackend } from '../../testing/backend';
import { buttons, render } from '../../testing/render';
import checkIn from '../../testing/fixtures/before-checkin.json';
import entrants from '../../testing/fixtures/before-entrants.json';
import format from '../../testing/fixtures/before-format.json';
import tournament from '../../testing/fixtures/before-tournament.json';
import desk from '../../testing/fixtures/progress-desk.json';
import phases from '../../testing/fixtures/finished-phases.json';
import presets from '../../testing/fixtures/presets.json';
import results from '../../testing/fixtures/finished-results.json';
import settings from '../../testing/fixtures/settings.json';
import { CheckIn } from '../check-in/check-in';
import { ControlDesk } from '../control-desk/control-desk';
import { Entrants } from '../entrants/entrants';
import { Format } from '../format/format';
import { Phases } from '../phases/phases';
import { Results } from '../results/results';
import { CATEGORY_SHOWN_KEY, CategoryShown } from './category-shown.service';

const MD = 'MD' as Category;
const WD = 'WD' as Category;
const XD = 'XD' as Category;
const both = [{ category: MD }, { category: WD }];

describe('CategoryShown', () => {
  beforeEach(() => {
    localStorage.removeItem(CATEGORY_SHOWN_KEY);
    TestBed.resetTestingModule();
  });

  it('starts on all and the first category', () => {
    const s = TestBed.inject(CategoryShown);
    expect(s.orAll(both)).toBe('all');
    expect(s.pick(both)?.category).toBe(MD);
    expect(s.pick([])).toBeNull();
    expect(s.orAll(undefined)).toBe('all');
  });

  it('keeps the last value and the last category, and remembers them', () => {
    const s = TestBed.inject(CategoryShown);
    s.show(WD);
    expect(s.orAll(both)).toBe(WD);
    expect(s.pick(both)?.category).toBe(WD);
    s.show('all');
    expect(s.orAll(both)).toBe('all');
    expect(s.pick(both)?.category).toBe(WD);
    s.showOnly(MD);
    expect(s.orAll(both)).toBe('all');
    expect(s.pick(both)?.category).toBe(MD);
    expect(JSON.parse(localStorage.getItem(CATEGORY_SHOWN_KEY)!)).toEqual({ last: 'all', category: MD });

    TestBed.resetTestingModule();
    expect(TestBed.inject(CategoryShown).pick(both)?.category).toBe(MD);
  });

  it('falls back to the default for a category the tournament does not have', () => {
    localStorage.setItem(CATEGORY_SHOWN_KEY, JSON.stringify({ last: XD, category: XD }));
    const s = TestBed.inject(CategoryShown);
    expect(s.orAll(both)).toBe('all');
    expect(s.pick(both)?.category).toBe(MD);
  });

  it('survives an unreadable storage', () => {
    localStorage.setItem(CATEGORY_SHOWN_KEY, '{');
    const s = TestBed.inject(CategoryShown);
    expect(s.orAll(both)).toBe('all');
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('full');
    });
    s.show(WD);
    expect(s.orAll(both)).toBe(WD);
    vi.restoreAllMocks();
  });
});

describe('category shown across screens', () => {
  beforeEach(() => {
    new FakeBackend()
      .on('TournamentService.GetTournamentView', tournament as unknown as TournamentView)
      .on('TournamentService.GetEntrantsView', entrants as unknown as EntrantsView)
      .on('TournamentService.GetCheckInView', checkIn as unknown as CheckInView)
      .on('TournamentService.GetControlDeskView', desk as unknown as ControlDeskView)
      .on('TournamentService.GetFormatView', format as unknown as FormatView)
      .on('TournamentService.GetPhasesView', phases as unknown as PhasesView)
      .on('TournamentService.GetResultsView', results as unknown as ResultsView)
      .on('TournamentService.GetHistoryView', { entries: [] })
      .on('PresetService.GetPresets', presets as unknown as Presets)
      .on('SettingsService.GetSettings', settings as unknown as Settings)
      .on('PresetService.EstimateMinutes', 22);
  });

  /**
   * The category a screen shows: the one of its active buttons (short code
   * or name, the Format screen adds the preview count), else "all".
   */
  async function shown(screen: Type<unknown>, selector = '.seg-option.active'): Promise<string> {
    const f = await render(screen, { keepCategory: true });
    const texts = buttons(f, selector).map((b) => b.textContent!.trim());
    for (const [code, name] of [['MD', "Men's doubles"], ['WD', "Women's doubles"]]) {
      if (texts.some((t) => t === code || t.startsWith(name))) {
        return code;
      }
    }
    return 'all';
  }

  async function choose(screen: Type<unknown>, selector: string, label: string): Promise<void> {
    const f = await render(screen, { keepCategory: true });
    buttons(f, selector)
      .find((b) => b.textContent!.trim() === label)!
      .click();
  }

  it('shows on every screen the category chosen on another', async () => {
    await choose(Entrants, '.filter .seg-option', "Women's doubles");
    expect(JSON.parse(localStorage.getItem(CATEGORY_SHOWN_KEY)!)).toEqual({ last: WD, category: WD });

    expect(await shown(CheckIn)).toBe('WD');
    expect(await shown(ControlDesk)).toBe('WD');
    expect(await shown(Format)).toBe('WD');
    expect(await shown(Phases)).toBe('WD');
    expect(await shown(Results)).toBe('WD');
    expect(await shown(Entrants, '.filter .seg-option.active')).toBe('WD');

    // "All" on the queue: the screens with "all" follow, the others keep WD.
    await choose(ControlDesk, '.queue-filter .seg-option', 'All');
    expect(await shown(CheckIn)).toBe('all');
    expect(await shown(Entrants, '.filter .seg-option.active')).toBe('all');
    expect(await shown(Results)).toBe('WD');

    // A category picked on a screen without "all" shows on the others too.
    await choose(Results, '.seg-option', "Men's doubles");
    expect(await shown(CheckIn)).toBe('MD');
    expect(await shown(Format)).toBe('MD');
  });

  it('opens every screen on its default without a stored category', async () => {
    localStorage.setItem(CATEGORY_SHOWN_KEY, JSON.stringify({ last: XD, category: XD }));
    expect(await shown(CheckIn)).toBe('all');
    expect(await shown(ControlDesk)).toBe('all');
    expect(await shown(Format)).toBe('MD');
    expect(await shown(Phases)).toBe('MD');
    expect(await shown(Results)).toBe('MD');
  });
});
