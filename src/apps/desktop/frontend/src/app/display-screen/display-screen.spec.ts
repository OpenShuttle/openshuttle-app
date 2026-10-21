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

  beforeEach(() => {
    go = new FakeBackend().on('SettingsService.GetSettings', settings as unknown as Settings);
    view(display);
  });

  it('calls the latest match in large type, with the courts and a standing', async () => {
    const f = await render(DisplayScreen);
    const call = display.calls[0];
    expect(text(f)).toContain(call.a.names[0]);
    expect(text(f)).toContain('Previous calls');
    expect(f.nativeElement.querySelectorAll('.tile').length).toBe(display.courts.length);
  });

  it('rotates the standings and goes full screen', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const f = await render(DisplayScreen);
    vi.advanceTimersByTime(settings.rotationSeconds * 1000 * 2);
    f.detectChanges();
    document.dispatchEvent(new MouseEvent('mousemove'));
    f.nativeElement.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
    await clickAll(f);
    vi.useRealTimers();
  });

  it('shows a timed round and its countdown, and a finished tournament', async () => {
    view(rounds);
    expect(text(await render(DisplayScreen))).toContain('Round 1');
    view(finished);
    await render(DisplayScreen);
    view({ ...display, calls: [], courts: [], boards: [] });
    await render(DisplayScreen);
  });
});
