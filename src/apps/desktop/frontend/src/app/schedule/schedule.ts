import { Component, ElementRef, HostListener, afterEveryRender, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ScheduleBlockView, ScheduleGroupView, ScheduleHighlightView, ScheduleSlotView, ScheduleView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { LanguageService } from '../core/language.service';
import { Panels } from '../core/panels.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { Glide } from '../shared/glide';
import { fullNames } from '../shared/names';
import { PanelToggle } from '../shared/panel-strip';
import { shortRoundLabel } from '../shared/match-stage';
import { CourtRules, scopeLabel } from './court-rules';
import { ScheduleDrag } from './schedule-drag';
import { PX_PER_MIN, axisHeight, axisStart, breakBands, halfHours, minutesFrom, slotHeight, slotTop, slotsByCourt } from './schedule-timeline';

const EMPTY: ScheduleView = {
  now: '',
  first: '' as Category,
  categories: [],
  courts: 0,
  minRest: 0,
  courtRules: [],
  scopes: [],
  slots: [],
  blocks: [],
  validatedAt: null,
  groups: [],
  highlights: [],
  end: null,
  unrested: 0,
  offRules: 0,
  start: null,
  dayEnd: null,
  late: false,
  breaks: [],
};

/**
 * Schedule: every match court by court, played ones greyed, the others at
 * their estimated times. Before validation the desk orders the rounds in the
 * block list; once validated, matches move one by one on the board
 * (ScheduleDrag). Go computes everything; this screen only lays it out.
 */
@Component({
  selector: 'app-schedule',
  imports: [TranslocoDirective, RouterLink, CategoryBadge, PanelToggle, CourtRules],
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
  /** The schedule with the court rule being chosen, null without one. */
  protected readonly rulePreview = signal<ScheduleView | null>(null);
  protected readonly view = computed<ScheduleView>(() => this.drag.preview() ?? this.rulePreview() ?? this.live());
  /** The live schedule, for the court rules panel (it previews from there). */
  protected readonly liveView = computed(() => this.live());
  protected readonly hasRules = computed(() => (this.live().courtRules ?? []).some((c) => (c.rules ?? []).length > 0));
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
  /** The breaks on the time axis. */
  protected readonly breakBands = computed(() => breakBands(this.view(), this.start(), this.height()));

  /** The break being typed: from and to ("12:00"). */
  protected readonly breakFrom = signal('');
  protected readonly breakTo = signal('');

  /**
   * The pools and brackets the desk highlights (keys of view.highlights).
   * A view convenience: never stored, reset on leaving the screen.
   */
  private readonly selected = signal<ReadonlySet<string>>(new Set());
  /** The highlighted keys still in the tournament. */
  protected readonly lit = computed<ReadonlySet<string>>(() => {
    const keys = new Set((this.live().highlights ?? []).map((h) => h.key));
    return new Set([...this.selected()].filter((k) => keys.has(k)));
  });
  /** The toggles, category by category. */
  protected readonly highlightGroups = computed(() => {
    const out: { category: Category; items: ScheduleHighlightView[] }[] = [];
    for (const h of this.live().highlights ?? []) {
      const last = out.at(-1);
      if (last?.category === h.category) {
        last.items.push(h);
      } else {
        out.push({ category: h.category, items: [h] });
      }
    }
    return out;
  });

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

  protected toggleHighlight(key: string): void {
    const next = new Set(this.lit());
    if (!next.delete(key)) {
      next.add(key);
    }
    this.selected.set(next);
  }

  protected clearHighlights(): void {
    this.selected.set(new Set());
  }

  /** Lit: its pool or bracket is highlighted; dimmed: others are, not it. */
  protected lights(group: string): 'lit' | 'dimmed' | null {
    const lit = this.lit();
    if (lit.size === 0) {
      return null;
    }
    return lit.has(group) ? 'lit' : 'dimmed';
  }

  /** A court's running rules, as its header's tooltip: "Priority 1 DD whole category · Never DH T · Final". */
  protected rulesOf(court: number): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const rules = (this.view().courtRules ?? []).find((c) => c.number === court)?.rules ?? [];
    return rules
      .filter((r) => r.scope.left)
      .map((r) => `${t('schedule.rules.chip.' + r.kind, { level: r.level })} ${t('category.short.' + r.scope.category)} ${scopeLabel(t, r.scope)}`)
      .join(' · ');
  }

  protected top(s: ScheduleSlotView): number {
    return slotTop(this.start(), s);
  }

  protected time(value: string | Date): string {
    return this.language.time(value);
  }

  /**
   * The stage's acronym (typed in the format, else P or T), then "A · 2/5"
   * for round 2 of the 5 of pool A, or a bracket round: "T · 1/4",
   * "T · 3e place", "C · 5e–8e". A group: "P A", or "T".
   */
  protected label(b: ScheduleSlotView | ScheduleBlockView | ScheduleGroupView | ScheduleHighlightView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const stage = b.acronym || t('stage.acronym.' + b.stageKind);
    if (!('round' in b)) {
      return b.pool ? t('schedule.pool', { stage, name: b.pool }) : stage;
    }
    if (b.pool) {
      return t('schedule.poolRound', { stage, name: b.pool, round: b.round, rounds: b.rounds || b.round });
    }
    // A bracket match known: its round and places; a stand-in or a block: its part of the draw and its size.
    const r = 'match' in b && b.match ? shortRoundLabel(b.match) : shortRoundLabel({ placeFrom: b.placeFrom || 1, placeTo: b.placeTo, roundSize: b.roundSize });
    return t('schedule.bracketRound', { stage, round: t(r.key, r.params) });
  }

  /** The full name of a block's stage, as a tooltip: "Main bracket", "Pools". */
  protected stageName(b: ScheduleSlotView | ScheduleBlockView | ScheduleHighlightView): string {
    return b.stageName || this.transloco.translate('stage.kind.' + b.stageKind);
  }

  protected bibs(s: ScheduleSlotView): string {
    return s.match ? `#${s.match.a.bib || '–'} · #${s.match.b.bib || '–'}` : '';
  }

  protected title(s: ScheduleSlotView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const sides = s.match
      ? `${fullNames(s.match.a)} (${t('schedule.left', { n: s.leftA })}) – ${fullNames(s.match.b)} (${t('schedule.left', { n: s.leftB })})`
      : t('schedule.unknownPlayers');
    const off = s.offRules && s.state === 'planned' ? ` · ${t('schedule.rules.offRuleSlot')}` : '';
    return `${this.time(s.start)} – ${this.time(s.end)} · ${this.label(s)} · ${sides}${off}`;
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

  protected date(value: string): string {
    return this.language.date(value);
  }

  protected async addBreak(): Promise<void> {
    if (await this.tournament.addBreak(this.breakFrom(), this.breakTo())) {
      this.breakFrom.set('');
      this.breakTo.set('');
    }
  }

  protected removeBreak(id: string): void {
    void this.tournament.removeBreak(id);
  }

  protected setFirst(category: Category): void {
    void this.tournament.setScheduleFirst(category);
  }

}
