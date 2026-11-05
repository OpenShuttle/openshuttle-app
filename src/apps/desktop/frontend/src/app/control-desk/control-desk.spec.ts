import { ControlDeskView, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { buttons, click, clickAll, render, text } from '../../testing/render';
import before from '../../testing/fixtures/before-desk.json';
import finished from '../../testing/fixtures/finished-desk.json';
import desk from '../../testing/fixtures/progress-desk.json';
import deskRules from '../../testing/fixtures/progress-desk-rules.json';
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

  it('colors each match badge like its pool on the schedule', async () => {
    const f = await render(ControlDesk);
    const styles = [...f.nativeElement.querySelectorAll('.badge')].map((b) => (b as HTMLElement).getAttribute('style'));
    expect(styles).toContainEqual(expect.stringContaining(`--color-group-${desk.courts[1].match!.color}`));
    expect(styles).toContainEqual(expect.stringContaining(`--color-group-${desk.queue[0].match.color}`));
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

  it('warns when the rules of a court keep a queued match off it', async () => {
    // Court 3 free: it never takes WD.
    view({ ...deskRules, courts: deskRules.courts.map((c) => (c.number === 3 ? { ...c, state: 'free', match: null } : c)) });
    const f = await render(ControlDesk);
    const q = deskRules.queue.find((x) => x.offCourts.length > 0 && x.reason !== 'playing')!;
    const row = [...f.nativeElement.querySelectorAll('.launch-btn')] as HTMLButtonElement[];
    row[deskRules.queue.filter((x) => x.reason !== 'playing').indexOf(q)].click();
    f.detectChanges();
    const courts = [...f.nativeElement.querySelectorAll('.launch-court')] as HTMLButtonElement[];
    const off = courts.filter((b) => b.getAttribute('title')?.includes('Court rules keep this match off'));
    expect(off.map((b) => b.textContent?.trim())).toEqual(['3']);
    expect(courts.find((b) => b.textContent?.trim() === '1')?.getAttribute('title')).toBeNull();
    expect(off.every((b) => b.classList.contains('force'))).toBe(true);
    expect(off.map((b) => Number(b.textContent?.trim())).every((n) => q.offCourts.includes(n))).toBe(true);
  });
});
