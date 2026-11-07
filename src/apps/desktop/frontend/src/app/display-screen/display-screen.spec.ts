import { DisplayView, Settings } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { clickAll, render, text } from '../../testing/render';
import finished from '../../testing/fixtures/finished-display.json';
import display from '../../testing/fixtures/progress-display.json';
import rounds from '../../testing/fixtures/rounds-display.json';
import settings from '../../testing/fixtures/settings.json';
import { DisplayScreen } from './display-screen';

describe('DisplayScreen', () => {
  let go: FakeBackend;
  const view = (v: unknown) => go.on('TournamentService.GetDisplayView', v as DisplayView);
  const title = (f: Awaited<ReturnType<typeof render>>) =>
    (f.nativeElement.querySelector('.stage-head') as HTMLElement).textContent?.replace(/\s+/g, ' ').trim();

  beforeEach(() => {
    go = new FakeBackend().on('SettingsService.GetSettings', settings as unknown as Settings);
    view(display);
  });

  afterEach(() => vi.useRealTimers());

  it('shows every pool of the stage played, with clubs, and the latest results', async () => {
    const f = await render(DisplayScreen);
    const stage = display.stages[0];
    expect(f.nativeElement.querySelectorAll('.pool').length).toBe(stage.pools.length);
    const row = stage.pools[0].standings[0].entrant;
    expect(text(f)).toContain(row.names[0]);
    expect(text(f)).toContain(row.clubs[0]);
    expect(text(f)).toContain('Latest results');
    expect(f.nativeElement.querySelectorAll('.result').length).toBe(display.results.length);
    // A decided result shows a trophy and a cross.
    expect(f.nativeElement.querySelector('.result app-result-icon .won')).not.toBeNull();
    expect(f.nativeElement.querySelector('.result app-result-icon .lost')).not.toBeNull();
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

  it('draws a bracket with its matches named', async () => {
    view(finished);
    const f = await render(DisplayScreen);
    expect(f.nativeElement.querySelectorAll('.bmatch').length).toBeGreaterThan(0);
    expect(text(f)).toContain('Final');
  });

  it('shows a timed round and its countdown, and waits for a stage', async () => {
    view(rounds);
    expect(text(await render(DisplayScreen))).toContain('Round 1');
    view({ ...display, stages: [], results: [] });
    const f = await render(DisplayScreen);
    expect(text(f)).toContain('No stage to show yet');
    expect(text(f)).toContain('No results yet');
  });
});
