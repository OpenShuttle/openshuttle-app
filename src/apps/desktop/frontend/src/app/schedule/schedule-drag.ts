import { Signal, computed, effect, signal, untracked } from '@angular/core';
import { ScheduleBlockView, ScheduleSlotView, ScheduleView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { MatchID } from '@bindings/internal/tournament';

import { TournamentStore } from '../core/tournament.service';

/** Pointer travel before a press becomes a drag, in pixels. */
const DRAG_THRESHOLD = 5;

/** Time the screen takes to settle after a preview, before the pointer moves anything again, in ms. */
const SETTLE_MS = 320;

/** A move the drop would make: a block before another one, or a match before another one ("" at the end). */
export type Move = { block: string; before: string } | { match: MatchID; before: MatchID | '' };

export function sameMove(a: Move | null, b: Move | null): boolean {
  if (!a || !b) {
    return a === b;
  }
  return 'block' in a
    ? 'block' in b && a.block === b.block && a.before === b.before
    : 'match' in b && a.match === b.match && a.before === b.before;
}

/**
 * What the pointer is over: a place to drop (before a match or a block, ""
 * at the end, with the words saying it), out of the board or list (null: no
 * move), or nothing that changes the move.
 */
export type Hit = { before: string; text: string } | 'keep' | null;

/** What is dragged: a match of the validated schedule, or a block (a round of a pool, a bracket round) of the list. */
export interface DragItem {
  kind: 'match' | 'block';
  key: string;
  match: MatchID | null;
  category: Category;
  slot: number;
  label: string;
}

/** What the drag needs from the schedule screen. */
export interface DragHost {
  /** The schedule from Go, without preview. */
  live: Signal<ScheduleView>;
  /** The schedule shown (the preview while dragging). */
  view: () => ScheduleView;
  store: TournamentStore;
  label(b: ScheduleSlotView | ScheduleBlockView): string;
  time(value: string | Date): string;
  translate(key: string, params?: Record<string, unknown>): string;
  slotTop(s: ScheduleSlotView): number;
  slotHeight(s: ScheduleSlotView): number;
}

/**
 * Drag and drop on the schedule, with pointer events (HTML5 drag and drop is
 * unreliable in webviews). Before validation, blocks are reordered in the
 * block list; once validated, matches move on the board. While dragging,
 * the screen shows the preview of the drop (Go projects the move, nothing is
 * saved) and the pointer works on what is shown, like a sortable list: over
 * an item, the dragged one goes before it (in a list, after it from its
 * lower half); over the dragged one, nothing changes. After each preview the
 * screen settles before the pointer moves anything again, so items glide
 * instead of bouncing.
 */
export class ScheduleDrag {
  /** What is dragged, once the pointer moved enough. */
  readonly dragging = signal<DragItem | null>(null);
  /** While dragging, what the drop does, said above the timeline. */
  readonly info = signal<{ item: DragItem; text: string } | null>(null);
  /** While dragging, the schedule as it would be after the drop. */
  readonly preview = signal<ScheduleView | null>(null);
  /** Where the preview puts the dragged match: its court and start. */
  readonly landing = computed(() => {
    const d = this.dragging();
    const p = this.preview();
    return d?.match && p ? ((p.slots ?? []).find((s) => s.state === 'planned' && s.match?.id === d.match) ?? null) : null;
  });

  /** Start of each waiting match and block when the drag began, to show how far the preview shifts it. */
  private baseStarts = new Map<string, number>();
  private drag: { item: DragItem; x: number; y: number; moved: boolean; move: Move | null } | null = null;
  private seq = 0;
  private settleUntil = 0;
  private settleTimer: ReturnType<typeof setTimeout> | undefined;

  /** Must be built in an injection context (it runs an effect). */
  constructor(private readonly host: DragHost) {
    // A new schedule from Go replaces the preview of the last drop.
    effect(() => {
      host.live();
      untracked(() => !this.drag && this.preview.set(null));
    });
  }

  /** A press on a match of the board: it moves once the schedule is validated. */
  slotDown(event: PointerEvent, slot: ScheduleSlotView): void {
    if (slot.movable && slot.match && event.button === 0) {
      const item: DragItem = { kind: 'match', key: slot.key, match: slot.match.id, category: slot.category, slot: slot.slot, label: this.host.label(slot) };
      this.start(event, item);
    }
  }

  /** A press on a block of the list. */
  blockDown(event: PointerEvent, b: ScheduleBlockView): void {
    if (event.button === 0) {
      this.start(event, { kind: 'block', key: b.key, match: null, category: b.category, slot: b.slot, label: this.host.label(b) });
    }
  }

  private start(event: PointerEvent, item: DragItem): void {
    const base = this.host.live();
    this.drag = { item, x: event.clientX, y: event.clientY, moved: false, move: null };
    this.baseStarts = new Map([
      ...(base.slots ?? []).filter((s) => s.match).map((s): [string, number] => [s.match!.id, new Date(s.start).getTime()]),
      ...(base.blocks ?? []).map((b): [string, number] => ['b:' + b.key, new Date(b.start).getTime()]),
    ]);
  }

  /** The pointer moved anywhere in the document. */
  move(event: PointerEvent): void {
    const drag = this.drag;
    if (!drag || (!drag.moved && Math.hypot(event.clientX - drag.x, event.clientY - drag.y) < DRAG_THRESHOLD)) {
      return;
    }
    event.preventDefault();
    drag.x = event.clientX;
    drag.y = event.clientY;
    if (!drag.moved) {
      drag.moved = true;
      this.dragging.set(drag.item);
      this.info.set({ item: drag.item, text: this.host.translate('schedule.dropNone') });
    }
    this.retarget();
  }

  /** The pointer was released: the move the preview showed is applied. */
  up(): void {
    const drag = this.drag;
    this.drag = null;
    clearTimeout(this.settleTimer);
    this.dragging.set(null);
    this.info.set(null);
    this.seq++;
    if (drag?.moved && drag.move) {
      void this.drop(drag.move);
    } else {
      this.preview.set(null);
    }
  }

  /** Matches carried by the drag: the dragged match, or the matches of the dragged block. */
  carried(s: ScheduleSlotView): boolean {
    const d = this.dragging();
    if (!d || s.state !== 'planned') {
      return false;
    }
    return d.match ? s.match?.id === d.match : s.key === d.key;
  }

  /** How far the preview moves a match, or a block ("b:<key>"), from where it was when the drag began, in minutes (0: not moved). */
  shift(key: string | undefined, start: string): number {
    const before = this.dragging() && this.preview() && key ? this.baseStarts.get(key) : undefined;
    return before === undefined ? 0 : Math.round((new Date(start).getTime() - before) / 60_000);
  }

  /** Follows the pointer once the screen has settled: a new move only when what is under it calls for one. */
  private retarget(): void {
    const drag = this.drag;
    if (!drag?.moved) {
      return;
    }
    const wait = this.settleUntil - performance.now();
    if (wait > 0) {
      clearTimeout(this.settleTimer);
      this.settleTimer = setTimeout(() => this.retarget(), wait);
      return;
    }
    const item = drag.item;
    const hit = item.kind === 'block' ? this.hitList(item, drag.x, drag.y) : this.hitBoard(drag.x, drag.y);
    if (hit === 'keep') {
      return;
    }
    let move: Move | null = null;
    if (hit) {
      move = item.kind === 'block' ? { block: item.key, before: hit.before } : { match: item.match!, before: hit.before as MatchID | '' };
    }
    if (sameMove(move, drag.move)) {
      return;
    }
    drag.move = move;
    this.info.set({ item, text: hit ? hit.text : this.host.translate('schedule.dropNone') });
    void this.showPreview(move);
  }

  /**
   * What the pointer is over, on the board as shown: a waiting match takes
   * the dragged one before it, the space below the last match of a court is
   * the end. The dragged match, a match on court or played, a gap, the court
   * names change nothing; out of the timeline, no move.
   */
  private hitBoard(x: number, y: number): Hit {
    const h = this.host;
    const el = document.elementFromPoint(x, y);
    if (!el?.closest('[data-timeline]')) {
      return null;
    }
    const lane = el.closest<HTMLElement>('[data-lane]');
    if (!lane) {
      return 'keep';
    }
    const slots = h.view().slots ?? [];
    const slotEl = el.closest<HTMLElement>('[data-slot-index]');
    if (slotEl) {
      const on = slots[Number(slotEl.dataset['slotIndex'])];
      if (!on?.match || on.state !== 'planned' || this.carried(on)) {
        return 'keep';
      }
      const label = this.withCategory(on.category, h.label(on));
      return { before: on.match.id, text: h.translate('schedule.dropBefore', { label, court: on.court, time: h.time(on.start) }) };
    }
    const court = Number(lane.dataset['lane']);
    const at = y - lane.getBoundingClientRect().top;
    const below = slots.filter((s) => s.court === court).every((s) => h.slotTop(s) + h.slotHeight(s) <= at);
    return below ? { before: '', text: h.translate('schedule.dropLast') } : 'keep';
  }

  /**
   * What the pointer is over, in the block list as shown: the upper half of
   * a block takes the dragged one before it, the lower half after it; below
   * the last one, the end. The dragged block changes nothing; out of the
   * list, no move.
   */
  private hitList(item: DragItem, x: number, y: number): Hit {
    const h = this.host;
    const el = document.elementFromPoint(x, y);
    const list = el?.closest<HTMLElement>('[data-blocks]');
    if (!list) {
      return null;
    }
    const blocks = h.view().blocks ?? [];
    const label = this.withCategory(item.category, item.label);
    const before = (i: number): Hit => {
      const b = blocks[i];
      if (!b) {
        return { before: '', text: h.translate('schedule.dropBlockLast', { label }) };
      }
      if (b.key === item.key) {
        return 'keep';
      }
      return { before: b.key, text: h.translate('schedule.dropBlockBefore', { label, before: this.withCategory(b.category, h.label(b)) }) };
    };
    const row = el?.closest<HTMLElement>('[data-block-index]');
    if (!row) {
      const rows = list.querySelectorAll<HTMLElement>('[data-block-index]');
      const last = rows.item(rows.length - 1);
      return last && y > last.getBoundingClientRect().bottom ? before(blocks.length) : 'keep';
    }
    const i = Number(row.dataset['blockIndex']);
    if (blocks[i]?.key === item.key) {
      return 'keep';
    }
    const r = row.getBoundingClientRect();
    return before(y < r.top + r.height / 2 ? i : i + 1);
  }

  /** With two categories, "MD Pool A": both have a pool A. */
  private withCategory(category: Category, label: string): string {
    return (this.host.view().categories ?? []).length > 1 ? `${this.host.translate('category.short.' + category)} ${label}` : label;
  }

  private async showPreview(move: Move | null): Promise<void> {
    const seq = ++this.seq;
    if (!move) {
      this.preview.set(null);
      this.settleUntil = performance.now() + SETTLE_MS;
      return;
    }
    try {
      const v = await ('block' in move
        ? TournamentService.PreviewScheduledBlockMove(move.block, move.before)
        : TournamentService.PreviewScheduledMatchMove(move.match, move.before as MatchID));
      if (seq === this.seq) {
        this.preview.set(v);
        this.settleUntil = performance.now() + SETTLE_MS;
      }
    } catch (err) {
      console.error('schedule preview', err);
      if (seq === this.seq) {
        this.preview.set(null);
      }
    }
  }

  private async drop(move: Move): Promise<void> {
    const store = this.host.store;
    const ok = await ('block' in move ? store.moveScheduledBlock(move.block, move.before) : store.moveScheduledMatch(move.match, move.before));
    // Done: the preview stays until the new schedule from Go replaces it (never the old order in between).
    if (!ok) {
      this.preview.set(null);
    } else {
      setTimeout(() => !this.drag && this.preview.set(null), 2000);
    }
  }
}
