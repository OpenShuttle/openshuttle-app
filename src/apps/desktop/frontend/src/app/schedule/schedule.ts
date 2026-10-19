import { Component, HostListener, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ScheduleGroupView, ScheduleSlotView, ScheduleView, TournamentService } from '@bindings/apps/desktop';
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

/** What a drop lands on: a planned slot (its index in the view), or the end of the order. */
type DropTarget = { slot: number } | { end: true };

function dropTargetAt(x: number, y: number): DropTarget | null {
  const el = document.elementFromPoint(x, y);
  const slot = el?.closest<HTMLElement>('[data-slot-index]');
  if (slot) {
    return { slot: Number(slot.dataset['slotIndex']) };
  }
  return el?.closest('[data-lane]') ? { end: true } : null;
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
  protected readonly view = liveView<ScheduleView>(() => TournamentService.GetScheduleView(), EMPTY, 60_000);
  protected readonly modes = ScheduleMode;
  protected readonly fullNames = fullNames;

  /** A second click confirms an action that drops the matches moved by hand. */
  protected readonly confirming = signal<'recompute' | 'unvalidate' | null>(null);

  /** The time axis starts with the first match on court, or now. */
  protected readonly start = computed(() => {
    const starts = (this.view().slots ?? []).map((s) => new Date(s.start).getTime());
    return Math.min(new Date(this.view().now).getTime(), ...starts);
  });
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

  /** Slots per court, each with its index in the view (drop targets). */
  protected readonly slotsByCourt = computed(() => {
    const out = new Map<number, { slot: ScheduleSlotView; index: number }[]>();
    (this.view().slots ?? []).forEach((slot, index) => {
      out.set(slot.court, [...(out.get(slot.court) ?? []), { slot, index }]);
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
  protected label(b: ScheduleSlotView | ScheduleGroupView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const stage = stageLabel(this.transloco, { name: b.stageName, kind: b.stageKind });
    if (!('round' in b)) {
      return b.pool ? (b.entryStage ? '' : stage + ' · ') + t('schedule.pool', { name: b.pool }) : stage;
    }
    if (b.pool) {
      return (b.entryStage ? '' : stage + ' · ') + t('schedule.poolRound', { name: b.pool, n: b.round });
    }
    // A bracket match known: its round and places; a stand-in: the round by its size.
    const r = b.match ? roundLabel(b.match) : roundLabel({ placeFrom: 1, placeTo: 0, roundSize: b.roundSize });
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

  /** Drag and drop with pointer events (HTML5 drag and drop is unreliable in webviews). */
  protected readonly dragging = signal<ScheduleSlotView | null>(null);
  protected readonly dropOn = signal<DropTarget | null>(null);
  private drag: { slot: ScheduleSlotView; x: number; y: number; moved: boolean } | null = null;

  protected onSlotDown(event: PointerEvent, slot: ScheduleSlotView): void {
    if (slot.movable && event.button === 0) {
      this.drag = { slot, x: event.clientX, y: event.clientY, moved: false };
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
    this.dragging.set(drag.slot);
    this.dropOn.set(dropTargetAt(event.clientX, event.clientY));
  }

  @HostListener('document:pointerup', ['$event'])
  protected onPointerUp(event: PointerEvent): void {
    const drag = this.drag;
    this.drag = null;
    this.dragging.set(null);
    this.dropOn.set(null);
    const target = drag?.moved ? dropTargetAt(event.clientX, event.clientY) : null;
    if (drag && target) {
      this.drop(drag.slot, target);
    }
  }

  /** Before validation the block moves before the target's block; after, the match before the target match. */
  private drop(slot: ScheduleSlotView, target: DropTarget): void {
    const on = 'slot' in target ? (this.view().slots ?? [])[target.slot] : null;
    if (on && on.state !== 'planned') {
      return; // already played: nothing goes before it
    }
    if (this.view().validatedAt) {
      if (slot.match) {
        void this.tournament.moveScheduledMatch(slot.match.id, (on?.match?.id ?? '') as MatchID | '');
      }
      return;
    }
    const before = on?.movable ? on.key : '';
    if (before !== slot.key) {
      void this.tournament.moveScheduledBlock(slot.key, before);
    }
  }

  /** Slots carried by the drag: the match, or its whole block before validation. */
  protected carried(s: ScheduleSlotView): boolean {
    const d = this.dragging();
    if (!d || s.state !== 'planned') {
      return false;
    }
    return this.view().validatedAt ? s.match?.id === d.match?.id : s.key === d.key;
  }

  protected isDropOn(index: number): boolean {
    const on = this.dropOn();
    return !!on && 'slot' in on && on.slot === index;
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
