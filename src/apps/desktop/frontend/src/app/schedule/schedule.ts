import { Component, ElementRef, HostListener, afterEveryRender, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ScheduleBlockView, ScheduleGroupView, ScheduleSlotView, ScheduleView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { ScheduleMode } from '@bindings/internal/tournament';

import { LanguageService } from '../core/language.service';
import { Panels } from '../core/panels.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { Glide } from '../shared/glide';
import { fullNames } from '../shared/names';
import { PanelToggle } from '../shared/panel-strip';
import { roundLabel } from '../shared/match-stage';
import { stageLabel } from '../shared/stage-label';
import { ScheduleDrag } from './schedule-drag';
import { PX_PER_MIN, axisHeight, axisStart, halfHours, minutesFrom, slotHeight, slotTop, slotsByCourt } from './schedule-timeline';

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
 * their estimated times. Before validation the desk orders the rounds in the
 * block list; once validated, matches move one by one on the board
 * (ScheduleDrag). Go computes everything; this screen only lays it out.
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
  protected readonly drag: ScheduleDrag = new ScheduleDrag({
    live: this.live,
    view: () => this.view(),
    store: this.tournament,
    label: (b) => this.label(b),
    time: (v) => this.time(v),
    translate: (key, params) => this.transloco.translate(key, params),
    slotTop: (s) => this.top(s),
    slotHeight,
  });
  protected readonly view = computed<ScheduleView>(() => this.drag.preview() ?? this.live());
  protected readonly modes = ScheduleMode;
  protected readonly fullNames = fullNames;
  protected readonly slotHeight = slotHeight;

  /** A second click confirms an action that drops the matches moved by hand. */
  protected readonly confirming = signal<'recompute' | 'unvalidate' | null>(null);

  /** The time axis starts with the first match on court, or now. */
  protected readonly start = computed(() => axisStart(this.view()));
  protected readonly courts = computed(() => Array.from({ length: this.view().courts }, (_, i) => i + 1));
  protected readonly height = computed(() => axisHeight(this.view(), this.start()));
  /** Matches still to play (stand-ins included). */
  protected readonly planned = computed(() => (this.view().slots ?? []).filter((s) => s.state === 'planned').length);
  /** Where "now" sits on the time axis. */
  protected readonly nowTop = computed(() => minutesFrom(this.start(), this.view().now) * PX_PER_MIN);
  /** Half-hour marks of the time axis. */
  protected readonly ticks = computed(() => halfHours(this.start(), this.height(), (d) => this.time(d)));
  /** Slots per court, each with its index in the view (drop targets) and its glide key. */
  protected readonly slotsByCourt = computed(() => slotsByCourt(this.view()));

  constructor() {
    // Matches and blocks glide to their new place on every change.
    const glide = new Glide(inject<ElementRef<HTMLElement>>(ElementRef).nativeElement);
    afterEveryRender(() => glide.run());
  }

  @HostListener('document:pointermove', ['$event'])
  protected onPointerMove(event: PointerEvent): void {
    this.drag.move(event);
  }

  @HostListener('document:pointerup')
  protected onPointerUp(): void {
    this.drag.up();
  }

  protected reservedFor(court: number): { category: Category; slot: number; manual: boolean } | null {
    const r = (this.view().reserved ?? []).find((x) => x.number === court);
    if (!r) {
      return null;
    }
    const slot = (this.view().categories ?? []).find((c) => c.category === r.category)?.slot ?? 1;
    return { category: r.category, slot, manual: r.manual };
  }

  protected top(s: ScheduleSlotView): number {
    return slotTop(this.start(), s);
  }

  protected time(value: string | Date): string {
    return this.language.time(value);
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
    // A bracket match known: its round and places; a stand-in or a block: its part of the draw and its size.
    const r = 'match' in b && b.match ? roundLabel(b.match) : roundLabel({ placeFrom: b.placeFrom || 1, placeTo: b.placeTo, roundSize: b.roundSize });
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
