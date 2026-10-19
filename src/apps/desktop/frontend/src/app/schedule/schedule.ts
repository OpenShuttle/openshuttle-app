import { Component, ElementRef, HostListener, afterEveryRender, computed, effect, inject, signal, untracked } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ScheduleBlockView, ScheduleGroupView, ScheduleSlotView, ScheduleView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { MatchID, ScheduleMode } from '@bindings/internal/tournament';

import { LanguageService } from '../core/language.service';
import { Panels } from '../core/panels.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { fullNames } from '../shared/names';
import { PanelToggle } from '../shared/panel-strip';
import { roundLabel } from '../shared/match-stage';
import { stageLabel } from '../shared/stage-label';

/** Pixels per minute on the time axis. */
const PX_PER_MIN = 2.4;

/** Pointer travel before a press becomes a drag, in pixels. */
const DRAG_THRESHOLD = 5;

/** Time the board takes to settle after a preview, before the pointer moves anything again, in ms. */
const SETTLE_MS = 320;

/** A move the drop would make: a block before another one, or a match before another one ("" at the end). */
type Move = { block: string; before: string } | { match: MatchID; before: MatchID | '' };

function sameMove(a: Move | null, b: Move | null): boolean {
  if (!a || !b) {
    return a === b;
  }
  return 'block' in a ? 'block' in b && a.block === b.block && a.before === b.before : 'match' in b && a.match === b.match && a.before === b.before;
}

/**
 * What the pointer is over: a place to drop (before a match or a block, ""
 * at the end, with the words saying it), out of the board or list (null: no
 * move), or nothing that changes the move.
 */
type Hit = { before: string; text: string } | 'keep' | null;

/** What is dragged: a match of the validated schedule, or a block (a round of a pool, a bracket round) of the list. */
interface DragItem {
  kind: 'match' | 'block';
  key: string;
  match: MatchID | null;
  category: Category;
  slot: number;
  label: string;
}

/** The time axis starts with the first match on court, or now. */
function axisStart(v: ScheduleView): number {
  return Math.min(new Date(v.now).getTime(), ...(v.slots ?? []).map((s) => new Date(s.start).getTime()));
}

const EMPTY: ScheduleView = {
  now: '',
  mode: ScheduleMode.ScheduleAlternate,
  first: '' as Category,
  categories: [],
  courts: 0,
  minRest: 0,
  reserved: [],
  slots: [],
  blocks: [],
  validatedAt: null,
  groups: [],
  end: null,
  unrested: 0,
};

/**
 * Schedule: every match court by court, played ones greyed, the others at
 * their estimated times. Before validation the desk drags a match to move
 * its whole block (pool or bracket round); once validated, matches move one
 * by one. Go computes everything; this screen only lays it out.
 */
@Component({
  selector: 'app-schedule',
  imports: [TranslocoDirective, RouterLink, CategoryBadge, PanelToggle],
  templateUrl: './schedule.html',
  styleUrl: './schedule.scss',
})
export class Schedule {
  protected readonly tournament = inject(TournamentStore);
  protected readonly panels = inject(Panels);
  private readonly transloco = inject(TranslocoService);
  private readonly language = inject(LanguageService);
  private readonly live = liveView<ScheduleView>(() => TournamentService.GetScheduleView(), EMPTY, 60_000);
  /** While dragging, the schedule as it would be after the drop (computed by Go, not saved). */
  private readonly preview = signal<ScheduleView | null>(null);
  protected readonly view = computed(() => this.preview() ?? this.live());
  protected readonly modes = ScheduleMode;
  protected readonly fullNames = fullNames;

  /** A second click confirms an action that drops the matches moved by hand. */
  protected readonly confirming = signal<'recompute' | 'unvalidate' | null>(null);

  /** The time axis starts with the first match on court, or now. */
  protected readonly start = computed(() => axisStart(this.view()));
  protected readonly courts = computed(() => Array.from({ length: this.view().courts }, (_, i) => i + 1));
  protected readonly height = computed(() => {
    const ends = (this.view().slots ?? []).map((s) => this.minutes(s.end));
    return Math.max(200, Math.max(0, ...ends) * PX_PER_MIN + 40);
  });
  /** Matches still to play (stand-ins included). */
  protected readonly planned = computed(() => (this.view().slots ?? []).filter((s) => s.state === 'planned').length);

  /** Where "now" sits on the time axis. */
  protected readonly nowTop = computed(() => this.minutes(this.view().now) * PX_PER_MIN);

  /** Half-hour marks of the time axis. */
  protected readonly ticks = computed(() => {
    const start = this.start();
    const end = start + (this.height() / PX_PER_MIN) * 60_000;
    const out: { top: number; label: string }[] = [];
    for (let t = Math.ceil(start / 1_800_000) * 1_800_000; t <= end; t += 1_800_000) {
      out.push({ top: ((t - start) / 60_000) * PX_PER_MIN, label: this.time(new Date(t)) });
    }
    return out;
  });

  /**
   * Slots per court, each with its index in the view and the key that
   * follows it from one view to the next (its match, or for a stand-in its
   * block, round and rank), so a moved match glides to its new place.
   */
  protected readonly slotsByCourt = computed(() => {
    const out = new Map<number, { slot: ScheduleSlotView; index: number; key: string }[]>();
    const seen = new Map<string, number>();
    (this.view().slots ?? []).forEach((slot, index) => {
      let key = slot.match?.id as string | undefined;
      if (!key) {
        const block = `${slot.key}#${slot.round}`;
        seen.set(block, (seen.get(block) ?? 0) + 1);
        key = `${block}#${seen.get(block)}`;
      }
      out.set(slot.court, [...(out.get(slot.court) ?? []), { slot, index, key }]);
    });
    return out;
  });

  protected reservedFor(court: number): { category: Category; slot: number; manual: boolean } | null {
    const r = (this.view().reserved ?? []).find((x) => x.number === court);
    if (!r) {
      return null;
    }
    const slot = (this.view().categories ?? []).find((c) => c.category === r.category)?.slot ?? 1;
    return { category: r.category, slot, manual: r.manual };
  }

  protected top(s: ScheduleSlotView): number {
    return this.minutes(s.start) * PX_PER_MIN;
  }

  protected slotHeight(s: ScheduleSlotView): number {
    return Math.max(18, (new Date(s.end).getTime() - new Date(s.start).getTime()) / 60_000) * PX_PER_MIN - 3;
  }

  private minutes(value: string | Date): number {
    return (new Date(value).getTime() - this.start()) / 60_000;
  }

  protected time(value: string | Date): string {
    return new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(new Date(value));
  }

  /** "Pool A · round 2", "Main bracket · Quarter-finals" or "Places 5–8"; a group: "Pool A". */
  protected label(b: ScheduleSlotView | ScheduleBlockView | ScheduleGroupView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const stage = stageLabel(this.transloco, { name: b.stageName, kind: b.stageKind });
    if (!('round' in b)) {
      return b.pool ? (b.entryStage ? '' : stage + ' · ') + t('schedule.pool', { name: b.pool }) : stage;
    }
    if (b.pool) {
      return (b.entryStage ? '' : stage + ' · ') + t('schedule.poolRound', { name: b.pool, n: b.round });
    }
    // A bracket match known: its round and places; a stand-in: the round by its size.
    const r = 'match' in b && b.match ? roundLabel(b.match) : roundLabel({ placeFrom: 1, placeTo: 0, roundSize: b.roundSize });
    return `${stage} · ${t(r.key, r.params)}`;
  }

  protected bibs(s: ScheduleSlotView): string {
    return s.match ? `#${s.match.a.bib || '–'} · #${s.match.b.bib || '–'}` : '';
  }

  protected title(s: ScheduleSlotView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const sides = s.match
      ? `${fullNames(s.match.a)} (${t('schedule.left', { n: s.leftA })}) – ${fullNames(s.match.b)} (${t('schedule.left', { n: s.leftB })})`
      : t('schedule.unknownPlayers');
    return `${this.time(s.start)} – ${this.time(s.end)} · ${this.label(s)} · ${sides}`;
  }

  // Validating fixes the order; recomputing or going back to blocks drops
  // the matches moved by hand, so they take a second click.
  protected validate(): void {
    if (this.view().validatedAt && this.confirming() !== 'recompute') {
      this.confirming.set('recompute');
      return;
    }
    this.confirming.set(null);
    void this.tournament.validateSchedule();
  }

  protected unvalidate(): void {
    if (this.confirming() !== 'unvalidate') {
      this.confirming.set('unvalidate');
      return;
    }
    this.confirming.set(null);
    void this.tournament.unvalidateSchedule();
  }

  /**
   * Drag and drop with pointer events (HTML5 drag and drop is unreliable in
   * webviews). Before validation, blocks are reordered in the block list;
   * once validated, matches move on the board. While dragging, the screen
   * shows the preview of the drop and the pointer works on what is shown,
   * like a sortable list: over an item, the dragged one goes before it (in a
   * list, after it from its lower half); over the dragged one, nothing
   * changes. After each preview the screen settles before the pointer moves
   * anything again, so items glide instead of bouncing.
   */
  protected readonly dragging = signal<DragItem | null>(null);
  /** While dragging, what the drop does, said above the timeline. */
  protected readonly dropInfo = signal<{ item: DragItem; text: string } | null>(null);
  /** Start of each waiting match and block when the drag began, to show how far the preview shifts it. */
  private baseStarts = new Map<string, number>();
  private drag: { item: DragItem; x: number; y: number; moved: boolean; move: Move | null } | null = null;
  private previewSeq = 0;
  private settleUntil = 0;
  private settleTimer: ReturnType<typeof setTimeout> | undefined;

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  /** Where each match and block was drawn, within its board or list (scrolling does not move it). */
  private positions = new Map<string, { x: number; y: number }>();

  constructor() {
    // A new schedule from Go replaces the preview of the last drop.
    effect(() => {
      this.live();
      untracked(() => !this.drag && this.preview.set(null));
    });
    afterEveryRender(() => this.glide());
  }

  /**
   * Every match moved by the last render glides from where it was drawn to
   * its new place, from one court to another included (FLIP): while dragging,
   * the desk sees the others take the freed places and the moved matches
   * take theirs.
   */
  private glide(): void {
    const next = new Map<string, { x: number; y: number }>();
    for (const el of this.host.nativeElement.querySelectorAll<HTMLElement>('[data-flip]')) {
      const lane = el.offsetParent as HTMLElement | null;
      const at = { x: el.offsetLeft + (lane?.offsetLeft ?? 0), y: el.offsetTop + (lane?.offsetTop ?? 0) };
      const key = el.dataset['flip'] ?? '';
      next.set(key, at);
      const was = this.positions.get(key);
      if (!was || (was.x === at.x && was.y === at.y)) {
        continue;
      }
      // Start from where it is seen, halfway through a glide included.
      let dx = was.x - at.x;
      let dy = was.y - at.y;
      const running = el.getAnimations();
      if (running.length > 0) {
        const m = new DOMMatrix(getComputedStyle(el).transform);
        dx += m.e;
        dy += m.f;
        running.forEach((a) => a.cancel());
      }
      el.animate([{ transform: `translate(${dx}px, ${dy}px)` }, { transform: 'translate(0, 0)' }], {
        duration: 280,
        easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)',
      });
    }
    this.positions = next;
  }

  protected onSlotDown(event: PointerEvent, slot: ScheduleSlotView): void {
    if (slot.movable && slot.match && event.button === 0) {
      this.startDrag(event, { kind: 'match', key: slot.key, match: slot.match.id, category: slot.category, slot: slot.slot, label: this.label(slot) });
    }
  }

  protected onBlockDown(event: PointerEvent, b: ScheduleBlockView): void {
    if (event.button === 0) {
      this.startDrag(event, { kind: 'block', key: b.key, match: null, category: b.category, slot: b.slot, label: this.label(b) });
    }
  }

  private startDrag(event: PointerEvent, item: DragItem): void {
    const base = this.live();
    this.drag = { item, x: event.clientX, y: event.clientY, moved: false, move: null };
    this.baseStarts = new Map([
      ...(base.slots ?? []).filter((s) => s.match).map((s): [string, number] => [s.match!.id, new Date(s.start).getTime()]),
      ...(base.blocks ?? []).map((b): [string, number] => ['b:' + b.key, new Date(b.start).getTime()]),
    ]);
  }

  @HostListener('document:pointermove', ['$event'])
  protected onPointerMove(event: PointerEvent): void {
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
      this.dropInfo.set({ item: drag.item, text: this.transloco.translate('schedule.dropNone') });
    }
    this.retarget();
  }

  @HostListener('document:pointerup')
  protected onPointerUp(): void {
    const drag = this.drag;
    this.drag = null;
    clearTimeout(this.settleTimer);
    this.dragging.set(null);
    this.dropInfo.set(null);
    this.previewSeq++;
    if (drag?.moved && drag.move) {
      void this.drop(drag.move);
    } else {
      this.preview.set(null);
    }
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
    this.dropInfo.set({ item, text: hit ? hit.text : this.transloco.translate('schedule.dropNone') });
    void this.showPreview(move);
  }

  /**
   * What the pointer is over, on the board as shown: a waiting match takes
   * the dragged one before it, the space below the last match of a court is
   * the end. The dragged match, a match on court or played, a gap, the court
   * names change nothing; out of the timeline, no move.
   */
  private hitBoard(x: number, y: number): Hit {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const el = document.elementFromPoint(x, y);
    if (!el?.closest('[data-timeline]')) {
      return null;
    }
    const lane = el.closest<HTMLElement>('[data-lane]');
    if (!lane) {
      return 'keep';
    }
    const slots = this.view().slots ?? [];
    const slotEl = el.closest<HTMLElement>('[data-slot-index]');
    if (slotEl) {
      const on = slots[Number(slotEl.dataset['slotIndex'])];
      if (!on?.match || on.state !== 'planned' || this.carried(on)) {
        return 'keep';
      }
      return { before: on.match.id, text: t('schedule.dropBefore', { label: this.withCategory(on.category, this.label(on)), court: on.court, time: this.time(on.start) }) };
    }
    const court = Number(lane.dataset['lane']);
    const at = y - lane.getBoundingClientRect().top;
    const below = slots.filter((s) => s.court === court).every((s) => this.top(s) + this.slotHeight(s) <= at);
    return below ? { before: '', text: t('schedule.dropLast') } : 'keep';
  }

  /**
   * What the pointer is over, in the block list as shown: the upper half of
   * a block takes the dragged one before it, the lower half after it; below
   * the last one, the end. The dragged block changes nothing; out of the
   * list, no move.
   */
  private hitList(item: DragItem, x: number, y: number): Hit {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const el = document.elementFromPoint(x, y);
    const list = el?.closest<HTMLElement>('[data-blocks]');
    if (!list) {
      return null;
    }
    const blocks = this.view().blocks ?? [];
    const before = (i: number): Hit => {
      const b = blocks[i];
      if (!b) {
        return { before: '', text: t('schedule.dropBlockLast', { label: this.withCategory(item.category, item.label) }) };
      }
      return b.key === item.key
        ? 'keep'
        : { before: b.key, text: t('schedule.dropBlockBefore', { label: this.withCategory(item.category, item.label), before: this.withCategory(b.category, this.label(b)) }) };
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
    return (this.view().categories ?? []).length > 1 ? `${this.transloco.translate('category.short.' + category)} ${label}` : label;
  }

  /** Where the preview puts the dragged match: its court and start. */
  protected readonly landing = computed(() => {
    const d = this.dragging();
    const p = this.preview();
    return d?.match && p ? ((p.slots ?? []).find((s) => s.state === 'planned' && s.match?.id === d.match) ?? null) : null;
  });

  /** How far the preview moves a match, or a block, from where it was when the drag began, in minutes (0: not moved). */
  protected shift(key: string | undefined, start: string): number {
    const before = this.dragging() && this.preview() && key ? this.baseStarts.get(key) : undefined;
    return before === undefined ? 0 : Math.round((new Date(start).getTime() - before) / 60_000);
  }

  private async showPreview(move: Move | null): Promise<void> {
    const seq = ++this.previewSeq;
    if (!move) {
      this.preview.set(null);
      this.settleUntil = performance.now() + SETTLE_MS;
      return;
    }
    try {
      const v = await ('block' in move
        ? TournamentService.PreviewScheduledBlockMove(move.block, move.before)
        : TournamentService.PreviewScheduledMatchMove(move.match, move.before as MatchID));
      if (seq === this.previewSeq) {
        this.preview.set(v);
        this.settleUntil = performance.now() + SETTLE_MS;
      }
    } catch (err) {
      console.error('schedule preview', err);
      if (seq === this.previewSeq) {
        this.preview.set(null);
      }
    }
  }

  private async drop(move: Move): Promise<void> {
    const ok = await ('block' in move ? this.tournament.moveScheduledBlock(move.block, move.before) : this.tournament.moveScheduledMatch(move.match, move.before));
    // Done: the preview stays until the new schedule from Go replaces it (never the old order in between).
    if (!ok) {
      this.preview.set(null);
    } else {
      setTimeout(() => !this.drag && this.preview.set(null), 2000);
    }
  }

  /** Matches carried by the drag: the dragged match, or the matches of the dragged block. */
  protected carried(s: ScheduleSlotView): boolean {
    const d = this.dragging();
    if (!d || s.state !== 'planned') {
      return false;
    }
    return d.match ? s.match?.id === d.match : s.key === d.key;
  }

  protected setMode(mode: ScheduleMode): void {
    void this.tournament.setScheduleMode(mode);
  }

  protected setFirst(category: Category): void {
    void this.tournament.setScheduleFirst(category);
  }

  protected reserve(court: number, event: Event): void {
    void this.tournament.setCourtCategory(court, (event.target as HTMLSelectElement).value as Category | '');
  }
}
