import { ScheduleView, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { choose, click, clickAll, drag, find, render, text } from '../../testing/render';
import before from '../../testing/fixtures/before-schedule.json';
import schedule from '../../testing/fixtures/progress-schedule.json';
import validated from '../../testing/fixtures/progress-schedule-validated.json';
import tournament from '../../testing/fixtures/progress-tournament.json';
import { Schedule } from './schedule';

describe('Schedule', () => {
  let go: FakeBackend;
  const view = (v: unknown) => go.on('TournamentService.GetScheduleView', v as ScheduleView);

  beforeEach(() => {
    go = new FakeBackend().on('TournamentService.GetTournamentView', tournament as unknown as TournamentView);
    view(schedule);
  });

  it('lists the rounds and draws the courts', async () => {
    const f = await render(Schedule);
    expect(text(f)).toContain('Order of rounds');
    expect(f.nativeElement.querySelectorAll('.block-row').length).toBe(schedule.blocks.length);
    expect(f.nativeElement.querySelectorAll('.slot').length).toBe(schedule.slots.length);
  });

  it('validates, and asks twice before recomputing or going back', async () => {
    const f = await render(Schedule);
    await click(f, 'Validate the schedule');
    expect(go.called('TournamentService.ValidateSchedule')).toBe(true);
    view(validated);
    go.emit('tournament.updated', 2);
    await new Promise((r) => setTimeout(r, 10));
    f.detectChanges();
    await click(f, 'Recompute');
    expect(go.argsOf('TournamentService.ValidateSchedule')).toHaveLength(1);
    await click(f, 'Recompute');
    expect(go.argsOf('TournamentService.ValidateSchedule')).toHaveLength(2);
    await click(f, 'Back to blocks');
    await click(f, 'Back to blocks');
    expect(go.called('TournamentService.UnvalidateSchedule')).toBe(true);
  });

  it('shares the courts between two categories', async () => {
    const f = await render(Schedule);
    await click(f, 'Reserved courts');
    expect(go.argsOf('TournamentService.SetScheduleMode')).toEqual([['reserved']]);
    await click(f, "Women's doubles");
    expect(go.argsOf('TournamentService.SetScheduleFirst')).toEqual([['WD']]);
  });

  it('reserves a court', async () => {
    view({ ...schedule, mode: 'reserved', reserved: [{ number: 1, category: 'MD', manual: true }] });
    const f = await render(Schedule);
    await choose(f, find(f, 'select') as HTMLSelectElement, 'WD');
    expect(go.argsOf('TournamentService.SetCourtCategory')).toEqual([[1, 'WD']]);
  });

  it('shows an empty schedule before the format', async () => {
    view({ ...before, slots: [], blocks: [] });
    const f = await render(Schedule);
    expect(text(f)).toContain('Nothing to schedule');
    view(before);
    await clickAll(await render(Schedule));
  });

  it('runs every button', async () => {
    await clickAll(await render(Schedule));
  });

  it('previews then moves a round dragged in the list', async () => {
    go.on('TournamentService.PreviewScheduledBlockMove', schedule);
    const f = await render(Schedule);
    const rows = [...f.nativeElement.querySelectorAll('[data-block-index]')] as HTMLElement[];
    await drag(f, rows[3], rows[0]);
    expect(go.called('TournamentService.PreviewScheduledBlockMove')).toBe(true);
    expect(go.argsOf('TournamentService.MoveScheduledBlock')[0]?.[0]).toBe(schedule.blocks[3].key);
  });

  it('previews then moves a match dragged on the board once validated', async () => {
    view(validated);
    go.on('TournamentService.PreviewScheduledMatchMove', validated);
    const f = await render(Schedule);
    const movable = [...f.nativeElement.querySelectorAll('.slot.movable')] as HTMLElement[];
    expect(movable.length).toBeGreaterThan(2);
    await drag(f, movable[movable.length - 1], movable[0]);
    expect(go.called('TournamentService.PreviewScheduledMatchMove')).toBe(true);
    expect(go.called('TournamentService.MoveScheduledMatch')).toBe(true);
    const lane = f.nativeElement.querySelector('[data-lane]') as HTMLElement;
    await drag(f, movable[0], lane, { x: 10, y: 99999 });
    const out = document.createElement('div');
    await drag(f, movable[0], out);
  });
});
