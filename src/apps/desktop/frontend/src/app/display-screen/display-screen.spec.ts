import { DisplayView, Settings } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { clickAll, render, text } from '../../testing/render';
import finished from '../../testing/fixtures/finished-display.json';
import display from '../../testing/fixtures/progress-display.json';
import rounds from '../../testing/fixtures/rounds-display.json';
import settings from '../../testing/fixtures/settings.json';
import { DisplayScreen, pageOffsets } from './display-screen';

describe('DisplayScreen', () => {
  let go: FakeBackend;
  const view = (v: unknown) => go.on('TournamentService.GetDisplayView', v as DisplayView);
  const title = (f: Awaited<ReturnType<typeof render>>) =>
    ['.cat', '.stage-title'].map((c) => (f.nativeElement.querySelector(`.stage-head ${c}`) as HTMLElement).textContent?.trim()).join(' ');

  beforeEach(() => {
    go = new FakeBackend().on('SettingsService.GetSettings', settings as unknown as Settings);
    view(display);
  });

  afterEach(() => vi.useRealTimers());

  it('shows every pool of the stage, with clubs, and the time', async () => {
    const f = await render(DisplayScreen);
    const stage = display.stages[0];
    expect(f.nativeElement.querySelectorAll('.pool').length).toBe(stage.pools.length);
    const row = stage.pools[0].standings[0].entrant;
    expect(text(f)).toContain(row.names[0]);
    expect(text(f)).toContain(row.clubs[0]);
    // Played, wins, set and point differences, each under its title.
    const cells = [...f.nativeElement.querySelectorAll('.pool .row')].map((r) => (r as HTMLElement).children.length);
    expect(new Set(cells)).toEqual(new Set([6]));
    expect(f.nativeElement.querySelector('.stage-head .clock')).not.toBeNull();
    // Neither the tournament's name nor a list of results.
    expect(text(f)).not.toContain(display.name);
    expect(f.nativeElement.querySelector('.result')).toBeNull();
  });

  it('takes the stages in turn, then goes full screen', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const f = await render(DisplayScreen);
    const first = title(f);
    vi.advanceTimersByTime(settings.rotationSeconds * 1000 + 200);
    f.detectChanges();
    expect(title(f)).not.toBe(first);
    vi.advanceTimersByTime(settings.rotationSeconds * 1000 + 200);
    f.detectChanges();
    expect(title(f)).toBe(first);
    document.dispatchEvent(new MouseEvent('mousemove'));
    f.nativeElement.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
    await clickAll(f);
  });

  it('draws a bracket with the winners links and the losers from the quarters, without clubs', async () => {
    const brackets = finished.stages.filter((s) => s.kind === 'bracket');
    view({ ...finished, stages: brackets });
    const f = await render(DisplayScreen);
    expect(f.nativeElement.querySelectorAll('.bmatch').length).toBeGreaterThan(0);
    expect(text(f)).toContain('Final');
    expect(f.nativeElement.querySelector('.link-win')).not.toBeNull();
    // A bracket of 8: every loser's link starts from the quarter-finals or later.
    expect(f.nativeElement.querySelector('.link-lose')).not.toBeNull();
    expect(f.nativeElement.querySelector('app-bracket-match .club')).toBeNull();
    // Two brackets in turn: the progress bar shows; alone, it does not.
    expect(f.nativeElement.querySelector('.progress')).not.toBeNull();
    view({ ...finished, stages: brackets.slice(0, 1) });
    const g = await render(DisplayScreen);
    expect(g.nativeElement.querySelector('.progress')).toBeNull();
  });

  it('shows a timed round and its countdown, and waits for a stage', async () => {
    view(rounds);
    expect(text(await render(DisplayScreen))).toContain('Round 1');
    view({ ...display, stages: [] });
    const f = await render(DisplayScreen);
    expect(text(f)).toContain('No stage to show yet');
  });
});

describe('pageOffsets', () => {
  /** A stage of the given height on a screen of 100 px, with pool cards [top, height]. */
  const stage = (height: number, cards: [number, number][] = []) => {
    const el = document.createElement('div');
    Object.defineProperty(el, 'clientHeight', { value: 100 });
    Object.defineProperty(el, 'scrollHeight', { value: height });
    for (const [top, h] of cards) {
      const card = document.createElement('div');
      card.className = 'page-block';
      Object.defineProperty(card, 'offsetTop', { value: top });
      Object.defineProperty(card, 'offsetHeight', { value: h });
      el.appendChild(card);
    }
    return el;
  };

  it('keeps a stage that fits on one page', () => {
    expect(pageOffsets(stage(80))).toEqual([0]);
  });

  it('breaks before the pool card a page would cut', () => {
    expect(pageOffsets(stage(250, [[0, 60], [70, 60], [140, 60], [210, 40]]))).toEqual([0, 70, 140, 150]);
  });

  it('moves by most of the screen without cards', () => {
    expect(pageOffsets(stage(250))).toEqual([0, 90, 150]);
  });
});
