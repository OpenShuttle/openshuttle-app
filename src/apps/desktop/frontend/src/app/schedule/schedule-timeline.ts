import { ScheduleSlotView, ScheduleView } from '@bindings/apps/desktop';

/** Pixels per minute on the time axis. */
export const PX_PER_MIN = 2.4;

/** A slot drawn on a court, with its index in the view and the key that follows it from one view to the next. */
export interface CourtSlot {
  slot: ScheduleSlotView;
  index: number;
  key: string;
}

/** The time axis starts with the first match on court, or now (ms). */
export function axisStart(v: ScheduleView): number {
  return Math.min(new Date(v.now).getTime(), ...(v.slots ?? []).map((s) => new Date(s.start).getTime()));
}

/** Minutes from the start of the axis. */
export function minutesFrom(start: number, value: string | Date): number {
  return (new Date(value).getTime() - start) / 60_000;
}

/** Where a slot starts on the axis, in pixels. */
export function slotTop(start: number, s: ScheduleSlotView): number {
  return minutesFrom(start, s.start) * PX_PER_MIN;
}

/** How tall a slot is drawn: its duration, at least 18 minutes' worth, minus the gap. */
export function slotHeight(s: ScheduleSlotView): number {
  return Math.max(18, (new Date(s.end).getTime() - new Date(s.start).getTime()) / 60_000) * PX_PER_MIN - 3;
}

/** The axis height: down to the last end, at least 200 px. */
export function axisHeight(v: ScheduleView, start: number): number {
  const ends = (v.slots ?? []).map((s) => minutesFrom(start, s.end));
  return Math.max(200, Math.max(0, ...ends) * PX_PER_MIN + 40);
}

/** Half-hour marks of the axis, labelled by format. */
export function halfHours(start: number, height: number, format: (d: Date) => string): { top: number; label: string }[] {
  const end = start + (height / PX_PER_MIN) * 60_000;
  const out: { top: number; label: string }[] = [];
  for (let t = Math.ceil(start / 1_800_000) * 1_800_000; t <= end; t += 1_800_000) {
    out.push({ top: ((t - start) / 60_000) * PX_PER_MIN, label: format(new Date(t)) });
  }
  return out;
}

/**
 * The slots of each court. A slot's key follows it from one view to the
 * next, so a moved match glides to its new place: its match, or for a
 * stand-in its block, round and rank.
 */
export function slotsByCourt(v: ScheduleView): Map<number, CourtSlot[]> {
  const out = new Map<number, CourtSlot[]>();
  const seen = new Map<string, number>();
  (v.slots ?? []).forEach((slot, index) => {
    let key = slot.match?.id as string | undefined;
    if (!key) {
      const block = `${slot.key}#${slot.round}`;
      seen.set(block, (seen.get(block) ?? 0) + 1);
      key = `${block}#${seen.get(block)}`;
    }
    out.set(slot.court, [...(out.get(slot.court) ?? []), { slot, index, key }]);
  });
  return out;
}
