import { Component, HostListener, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ScheduleGroupView, ScheduleMatchView, ScheduleSlotView, ScheduleView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { MatchID, ScheduleMode } from '@bindings/internal/tournament';

import { LanguageService } from '../core/language.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { fullNames } from '../shared/names';
import { stageLabel } from '../shared/stage-label';

/** Pixels per minute on the time axis. */
const PX_PER_MIN = 2.4;

/** Pointer travel before a press becomes a drag, in pixels. */
const DRAG_THRESHOLD = 5;

/** Where a dragged match lands: before the row at this index (the length: at the end). */
function dropIndexAt(x: number, y: number): number | null {
  const row = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-order-index]');
  if (!row) {
    return null;
  }
  const box = row.getBoundingClientRect();
  return Number(row.dataset['orderIndex']) + (y > box.top + box.height / 2 ? 1 : 0);
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
  validatedAt: null,
  order: [],
  groups: [],
  end: null,
  unrested: 0,
};

/**
 * Schedule: the forecast of every match left, court by court, from the
 * estimated durations. The desk validates the order once, then moves a match
 * or a whole pool by drag and drop. Go computes everything; this screen only
 * lays it out.
 */
@Component({
  selector: 'app-schedule',
  imports: [TranslocoDirective, RouterLink, CategoryBadge],
  templateUrl: './schedule.html',
  styleUrl: './schedule.scss',
})
export class Schedule {
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);
  private readonly language = inject(LanguageService);
  protected readonly view = liveView<ScheduleView>(() => TournamentService.GetScheduleView(), EMPTY, 60_000);
  protected readonly modes = ScheduleMode;

  /** Match highlighted in the timeline and the list. */
  protected readonly selected = signal<string | null>(null);
  /** Right panel: the order of the matches, or the teams. */
  protected readonly tab = signal<'order' | 'teams'>('order');
  /** What a drag moves: the match, or every waiting match of its pool. */
  protected readonly moveWhole = signal(false);
  /** A second click confirms recomputing a validated order. */
  protected readonly confirmingRecompute = signal(false);

  protected readonly start = computed(() => new Date(this.view().now).getTime());
  protected readonly courts = computed(() => Array.from({ length: this.view().courts }, (_, i) => i + 1));
  protected readonly height = computed(() => {
    const end = this.view().end;
    return end ? Math.max(200, this.minutes(end) * PX_PER_MIN + 40) : 200;
  });

  /** Half-hour marks of the time axis. */
  protected readonly ticks = computed(() => {
    const start = this.start();
    const end = this.view().end ? new Date(this.view().end!).getTime() : start;
    const first = Math.ceil(start / 1_800_000) * 1_800_000;
    const out: { top: number; label: string }[] = [];
    for (let t = first; t <= end + 1_800_000; t += 1_800_000) {
      out.push({ top: ((t - start) / 60_000) * PX_PER_MIN, label: this.time(new Date(t)) });
    }
    return out;
  });

  protected readonly slotsByCourt = computed(() => {
    const out = new Map<number, ScheduleSlotView[]>();
    for (const s of this.view().slots ?? []) {
      out.set(s.court, [...(out.get(s.court) ?? []), s]);
    }
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

  /** "Pool A · round 2" or "Main bracket · round 2". */
  protected label(b: ScheduleSlotView | ScheduleGroupView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const stage = b.entryStage && b.pool ? '' : stageLabel(this.transloco, { name: b.stageName, kind: b.stageKind }) + ' · ';
    if (!('round' in b)) {
      return b.pool ? stage + t('schedule.pool', { name: b.pool }) : stage.replace(/ · $/, '');
    }
    return b.pool ? stage + t('schedule.poolRound', { name: b.pool, n: b.round }) : stage + t('schedule.round', { n: b.round });
  }

  protected bibs(s: ScheduleSlotView): string {
    return s.match ? `#${s.match.a.bib || '–'} · #${s.match.b.bib || '–'}` : '';
  }

  protected names(s: ScheduleSlotView): string {
    return s.match ? `${fullNames(s.match.a)} – ${fullNames(s.match.b)}` : this.transloco.translate('schedule.unknownPlayers');
  }

  protected readonly fullNames = fullNames;

  protected matchKey(o: ScheduleMatchView): string {
    return o.match?.id ?? '';
  }

  // Validating fixes the order; validating again recomputes it from the
  // rules, so it takes a second click once moves were made.
  protected validate(): void {
    if (this.view().validatedAt && !this.confirmingRecompute()) {
      this.confirmingRecompute.set(true);
      return;
    }
    this.confirmingRecompute.set(false);
    void this.tournament.validateSchedule();
  }

  /** Moves a match (or its pool) before the row at index ("at the end" past the last). */
  private moveTo(o: ScheduleMatchView, index: number): void {
    const order = this.view().order ?? [];
    const id = o.match?.id;
    if (!id) {
      return;
    }
    const before = (order[index]?.match?.id ?? '') as MatchID | '';
    if (this.moveWhole() && o.pool) {
      void this.tournament.moveScheduledPool(id, before);
    } else {
      void this.tournament.moveScheduledMatch(id, before);
    }
  }

  protected step(index: number, delta: number): void {
    const order = this.view().order ?? [];
    // Down one: before the row two places on.
    this.moveTo(order[index], delta < 0 ? index - 1 : Math.min(order.length, index + 2));
  }

  /** Drag and drop with pointer events (HTML5 drag and drop is unreliable in webviews). */
  protected readonly dragging = signal<string | null>(null);
  protected readonly dropIndex = signal<number | null>(null);
  private drag: { row: ScheduleMatchView; x: number; y: number; moved: boolean } | null = null;

  protected onRowDown(event: PointerEvent, row: ScheduleMatchView): void {
    if (this.view().validatedAt && event.button === 0 && !(event.target as HTMLElement).closest('button')) {
      this.drag = { row, x: event.clientX, y: event.clientY, moved: false };
    }
  }

  @HostListener('document:pointermove', ['$event'])
  protected onPointerMove(event: PointerEvent): void {
    const drag = this.drag;
    if (!drag || (!drag.moved && Math.hypot(event.clientX - drag.x, event.clientY - drag.y) < DRAG_THRESHOLD)) {
      return;
    }
    drag.moved = true;
    event.preventDefault();
    this.dragging.set(this.matchKey(drag.row));
    this.dropIndex.set(dropIndexAt(event.clientX, event.clientY));
  }

  @HostListener('document:pointerup', ['$event'])
  protected onPointerUp(event: PointerEvent): void {
    const drag = this.drag;
    this.drag = null;
    this.dragging.set(null);
    this.dropIndex.set(null);
    const index = drag?.moved ? dropIndexAt(event.clientX, event.clientY) : null;
    if (drag && index !== null) {
      this.moveTo(drag.row, index);
    }
  }

  /** Rows dragged along with the one held: its pool in "whole pool" mode. */
  protected inDrag(o: ScheduleMatchView): boolean {
    const key = this.dragging();
    if (!key) {
      return false;
    }
    if (this.matchKey(o) === key) {
      return true;
    }
    const held = (this.view().order ?? []).find((x) => this.matchKey(x) === key);
    return this.moveWhole() && !!held?.pool && held.key === o.key;
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
