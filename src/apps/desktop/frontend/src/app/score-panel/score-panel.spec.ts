import { ControlDeskView, MatchView } from '@bindings/apps/desktop';

import { FakeBackend, backendFailure } from '../../testing/backend';
import { buttons, render, text } from '../../testing/render';
import desk from '../../testing/fixtures/progress-desk.json';
import rounds from '../../testing/fixtures/rounds-desk.json';
import { ScorePanel } from './score-panel';

const pending = (desk as unknown as ControlDeskView).pending![0] as MatchView;

function inputs(f: Awaited<ReturnType<typeof render<ScorePanel>>>): HTMLInputElement[] {
  return [...f.nativeElement.querySelectorAll('input.point')] as HTMLInputElement[];
}

/** Types the points in Tab order: set 1 side A, set 1 side B, set 2… */
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
    await fill(f, [15, 12, 15, 9]);
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
    await fill(f, [15, 3, 15, 3]);
    f.nativeElement.querySelector('form').dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r, 10));
    f.detectChanges();
    expect(text(f)).toContain('Set 1');
  });

  it('tabs down to the other side, then to the next set', async () => {
    const f = await render(ScorePanel, { inputs: { match: pending } });
    const labels = inputs(f).map((i) => i.getAttribute('aria-label'));
    expect(labels[0]).toContain('Set 1');
    expect(labels[1]).toContain('Set 1');
    expect(labels[1]).toContain(pending.b.names![0]);
    expect(labels[2]).toContain('Set 2');
  });

  it('leaves out the third set and the forfeit once two sets decide the match', async () => {
    go.on('TournamentService.ScoreDecidedAfter', (_win: unknown, score: { sets: unknown[] }) => (score.sets.length >= 2 ? 2 : 0));
    const f = await render(ScorePanel, { inputs: { match: pending } });
    await fill(f, [15, 3, 15, 9, 1, 0]);
    await new Promise((r) => setTimeout(r));
    f.detectChanges();
    const fields = inputs(f);
    expect(fields.slice(0, 4).every((i) => !i.disabled)).toBe(true);
    expect(fields.slice(4).every((i) => i.disabled)).toBe(true);
    expect(buttons(f).filter((b) => b.textContent?.includes('WO')).every((b) => (b as HTMLButtonElement).disabled)).toBe(true);
    f.nativeElement.querySelector('form').dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    const [, score] = go.argsOf('TournamentService.RecordScore')[0] as [string, { sets: unknown[] }];
    expect(score.sets).toEqual([
      { a: 15, b: 3 },
      { a: 15, b: 9 },
    ]);
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
