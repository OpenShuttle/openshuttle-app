import { ScheduleSlotView, ScheduleView } from '@bindings/apps/desktop';

import schedule from '../../testing/fixtures/progress-schedule.json';
import { PX_PER_MIN, axisHeight, axisStart, breakBands, halfHours, minutesFrom, slotHeight, slotTop, slotsByCourt } from './schedule-timeline';

const view = schedule as unknown as ScheduleView;

describe('schedule timeline', () => {
  it('starts with the first match on court', () => {
    const start = axisStart(view);
    expect(start).toBeLessThanOrEqual(new Date(view.now).getTime());
    expect(Math.min(...view.slots!.map((s) => slotTop(start, s)))).toBe(0);
  });

  it('starts with the first match before a tournament starting later', () => {
    const first = Math.min(...view.slots!.map((s) => Date.parse(s.start)));
    const later = { ...view, now: '2026-10-16T09:00:00Z', start: view.slots![0].start };
    expect(axisStart(later)).toBe(first);
  });

  it('places the breaks, clipped to the axis, and grows the axis to their end', () => {
    const start = Date.parse('2026-10-17T09:00:00Z');
    const v = {
      ...view,
      slots: [{ start: '2026-10-17T09:00:00Z', end: '2026-10-17T10:00:00Z' } as ScheduleSlotView],
      breaks: [
        { id: 'k1', from: '2026-10-17T08:30:00Z', to: '2026-10-17T09:10:00Z' },
        { id: 'k2', from: '2026-10-17T09:50:00Z', to: '2026-10-17T11:00:00Z' },
        { id: 'k3', from: '2026-10-17T15:00:00Z', to: '2026-10-17T16:00:00Z' },
      ],
    };
    const height = axisHeight(v, start);
    expect(height).toBe(120 * PX_PER_MIN + 40);
    const bands = breakBands(v, start, height);
    expect(bands.map((b) => b.id)).toEqual(['k1', 'k2']);
    expect(bands[0]).toMatchObject({ top: 0, height: 10 * PX_PER_MIN });
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
