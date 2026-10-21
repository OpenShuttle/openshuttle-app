import { ControlDeskView, MatchView } from '@bindings/apps/desktop';

import { FakeBackend, backendFailure } from '../../testing/backend';
import { buttons, render, text } from '../../testing/render';
import desk from '../../testing/fixtures/progress-desk.json';
import rounds from '../../testing/fixtures/rounds-desk.json';
import { ScorePanel } from './score-panel';

const pending = (desk as unknown as ControlDeskView).pending![0] as MatchView;

async function fill(f: Awaited<ReturnType<typeof render<ScorePanel>>>, values: number[]): Promise<void> {
  const inputs = [...f.nativeElement.querySelectorAll('input[inputmode], input[type=number], .set-input input, input.score')] as HTMLInputElement[];
  values.forEach((v, i) => {
    if (inputs[i]) {
      inputs[i].value = String(v);
      inputs[i].dispatchEvent(new Event('input'));
    }
  });
  f.detectChanges();
}

describe('ScorePanel', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend();
  });

  it('records a score', async () => {
    const f = await render(ScorePanel, { inputs: { match: pending } });
    expect(text(f)).toContain(pending.a.names![0]);
    await fill(f, [15, 15, 12, 9]);
    f.nativeElement.querySelector('form').dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    const [id] = go.argsOf('TournamentService.RecordScore')[0];
    expect(id).toBe(pending.id);
  });

  it('shows a refused score on its set', async () => {
    go.on('TournamentService.RecordScore', () => {
      throw backendFailure('INVALID_SET_SCORE', 1);
    });
    const f = await render(ScorePanel, { inputs: { match: pending } });
    await fill(f, [15, 15, 3, 3]);
    f.nativeElement.querySelector('form').dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r, 10));
    f.detectChanges();
    expect(text(f)).toContain('Set 1');
  });

  it('declares a forfeit with a second click', async () => {
    const f = await render(ScorePanel, { inputs: { match: pending } });
    const wo = buttons(f).find((b) => b.textContent?.includes('WO'))!;
    wo.click();
    f.detectChanges();
    expect(go.called('TournamentService.WithdrawEntrant')).toBe(false);
    wo.click();
    await new Promise((r) => setTimeout(r));
    expect(go.called('TournamentService.WithdrawEntrant')).toBe(true);
  });

  it('enters a timed score with a golden point', async () => {
    const timed = (rounds as unknown as ControlDeskView).rounds.current!.matches![0] as MatchView;
    const f = await render(ScorePanel, { inputs: { match: timed } });
    await fill(f, [11, 11]);
    const golden = f.nativeElement.querySelector('input[type=checkbox]') as HTMLInputElement | null;
    golden?.click();
    f.nativeElement.querySelector('form').dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    expect(go.called('TournamentService.RecordScore')).toBe(true);
  });
});
