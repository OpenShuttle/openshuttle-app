import { ControlDeskView, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { buttons, click, clickAll, render, text } from '../../testing/render';
import before from '../../testing/fixtures/before-desk.json';
import finished from '../../testing/fixtures/finished-desk.json';
import desk from '../../testing/fixtures/progress-desk.json';
import tournament from '../../testing/fixtures/progress-tournament.json';
import rounds from '../../testing/fixtures/rounds-desk.json';
import { ControlDesk } from './control-desk';

describe('ControlDesk', () => {
  let go: FakeBackend;
  const view = (v: unknown) => go.on('TournamentService.GetControlDeskView', v as ControlDeskView);

  beforeEach(() => {
    go = new FakeBackend().on('TournamentService.GetTournamentView', tournament as unknown as TournamentView);
    view(desk);
  });

  it('shows the courts, the pending score and the queue', async () => {
    const f = await render(ControlDesk);
    const page = text(f);
    expect(page).toContain(desk.courts[1].match!.a.names[0]);
    expect(page).toContain(desk.queue[0].match.a.names[0]);
    expect(page).toContain('1 score pending');
  });

  it('calls the proposed match on a finished court', async () => {
    const f = await render(ControlDesk);
    await click(f, 'Start next');
    expect(go.argsOf('TournamentService.CallMatch')).toEqual([[desk.courts[0].proposal!.id, 1]]);
  });

  it('ends a match on court', async () => {
    const f = await render(ControlDesk);
    await click(f, 'End');
    expect(go.argsOf('TournamentService.EndMatch')).toEqual([[desk.courts[1].match!.id]]);
  });

  it('opens the score panel of a pending score', async () => {
    const f = await render(ControlDesk);
    await click(f, 'Enter');
    expect(text(f)).toContain('Enter the score');
  });

  it('filters the queue and calls a match from it', async () => {
    const f = await render(ControlDesk);
    await click(f, 'WD');
    expect(text(f)).toContain(desk.queue[0].match.b.names[0]);
    await click(f, 'Call');
    const courts = buttons(f).filter((b) => /^\s*\d+\s*$/.test(b.textContent ?? ''));
    courts[0]?.click();
    expect(go.called('TournamentService.CallMatch')).toBe(true);
  });

  it('runs every button', async () => {
    const f = await render(ControlDesk);
    expect(await clickAll(f)).toBeGreaterThan(5);
  });

  it('starts and ends timed rounds', async () => {
    view(rounds);
    const f = await render(ControlDesk);
    expect(text(f)).toContain('Round 1');
    await click(f, 'Whistle the end');
    expect(go.called('TournamentService.EndRound')).toBe(true);
    await clickAll(f);
  });

  it('says what to do before the draw, and when everything is played', async () => {
    view(before);
    let f = await render(ControlDesk);
    expect(text(f)).toContain('Go to the draw');
    view(finished);
    f = await render(ControlDesk);
    await clickAll(f);
  });
});
