import { ScheduleSlotView, ScheduleView } from '@bindings/apps/desktop';

import schedule from '../../testing/fixtures/progress-schedule.json';
import { PX_PER_MIN, axisHeight, axisStart, halfHours, minutesFrom, slotHeight, slotTop, slotsByCourt } from './schedule-timeline';

const view = schedule as unknown as ScheduleView;

describe('schedule timeline', () => {
  it('starts with the first match on court', () => {
    const start = axisStart(view);
    expect(start).toBeLessThanOrEqual(new Date(view.now).getTime());
    expect(Math.min(...view.slots!.map((s) => slotTop(start, s)))).toBe(0);
  });

  it('measures slots and the axis', () => {
    const s = { start: '2026-10-17T09:00:00Z', end: '2026-10-17T09:30:00Z' } as ScheduleSlotView;
    expect(slotHeight(s)).toBe(30 * PX_PER_MIN - 3);
    expect(slotHeight({ ...s, end: s.start })).toBe(18 * PX_PER_MIN - 3);
    expect(minutesFrom(Date.parse(s.start), s.end)).toBe(30);
    expect(axisHeight({ ...view, slots: [] }, Date.now())).toBe(200);
    expect(axisHeight(view, axisStart(view))).toBeGreaterThan(200);
  });

  it('marks every half hour', () => {
    const start = Date.parse('2026-10-17T09:10:00Z');
    const ticks = halfHours(start, 60 * PX_PER_MIN, (d) => d.toISOString().slice(11, 16));
    expect(ticks.map((t) => t.label)).toEqual(['09:30', '10:00']);
    expect(ticks[0].top).toBe(20 * PX_PER_MIN);
  });

  it('groups slots by court with stable keys', () => {
    const byCourt = slotsByCourt(view);
    const all = [...byCourt.values()].flat();
    expect(all).toHaveLength(view.slots!.length);
    expect(new Set(all.map((s) => s.key)).size).toBe(all.length);
  });
});
