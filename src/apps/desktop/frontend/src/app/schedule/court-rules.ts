import {
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import {
  CourtRuleView,
  CourtScopeView,
  ScheduleView,
  TournamentService,
} from '@bindings/apps/desktop';
import { CourtRule, CourtRuleKind } from '@bindings/internal/tournament';

import { Panels } from '../core/panels.service';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { shortRoundLabel } from '../shared/match-stage';

/** The highest priority level (1 goes first), as Go's MaxCourtPriority. */
const MAX_LEVEL = 9;

/** Names a scope: "whole category", "P · Pools", "P A", "T · Final". */
export function scopeLabel(
  t: (key: string, params?: Record<string, unknown>) => string,
  sc: CourtScopeView,
): string {
  if (!sc.scope.stage) {
    return t('schedule.rules.wholeCategory');
  }
  const stage = sc.acronym || t('stage.acronym.' + sc.stageKind);
  if (sc.scope.pool) {
    return t('schedule.pool', { stage, name: sc.scope.pool });
  }
  if (sc.scope.round) {
    const r = shortRoundLabel({
      placeFrom: sc.scope.from ?? 1,
      placeTo: sc.placeTo,
      roundSize: sc.scope.round,
    });
    return t('schedule.bracketRound', { stage, round: t(r.key, r.params) });
  }
  return t('schedule.rules.stage', {
    stage,
    name: sc.stageName || t('stage.kind.' + sc.stageKind),
  });
}

/**
 * Court rules of the Schedule screen (ADR 12): per court, what it takes
 * first, only, or never, by category, stage, pool or bracket round. Adding
 * a rule previews it on the timeline (Go projects it, nothing is saved)
 * until it is added or the form is cleared. Go applies the rules; this
 * panel only edits them.
 */
@Component({
  selector: 'app-court-rules',
  imports: [TranslocoDirective, CategoryBadge],
  templateUrl: './court-rules.html',
  styleUrl: './court-rules.scss',
})
export class CourtRules {
  readonly view = input.required<ScheduleView>();
  /** The schedule as it would be with the rule being chosen; null without one. */
  readonly preview = output<ScheduleView | null>();

  protected readonly panels = inject(Panels);
  private readonly store = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);
  protected readonly kinds = [
    CourtRuleKind.CourtPriority,
    CourtRuleKind.CourtOnly,
    CourtRuleKind.CourtNever,
  ];
  protected readonly maxLevel = MAX_LEVEL;

  /** The rule being chosen: its courts, kind, level and scope (index in the view's scopes, -1 none). */
  protected readonly courts = signal<number[]>([]);
  protected readonly kind = signal<CourtRuleKind>(CourtRuleKind.CourtPriority);
  protected readonly level = signal(1);
  protected readonly scope = signal(-1);

  protected readonly courtNumbers = computed(() =>
    Array.from({ length: this.view().courts }, (_, i) => i + 1),
  );
  /** The courts with rules, and the numbers of the others. */
  protected readonly ruled = computed(() =>
    (this.view().courtRules ?? []).filter((c) => (c.rules ?? []).length > 0),
  );
  protected readonly free = computed(() =>
    (this.view().courtRules ?? []).filter((c) => (c.rules ?? []).length === 0).map((c) => c.number),
  );
  protected readonly count = computed(() =>
    (this.view().courtRules ?? []).reduce((n, c) => n + (c.rules ?? []).length, 0),
  );

  /** The scopes grouped by category, for the target picker. */
  protected readonly scopeGroups = computed(() => {
    const groups: {
      category: CourtScopeView;
      options: { index: number; scope: CourtScopeView; depth: number }[];
    }[] = [];
    (this.view().scopes ?? []).forEach((sc, index) => {
      if (!sc.scope.stage) {
        groups.push({ category: sc, options: [] });
      }
      const depth = !sc.scope.stage ? 0 : sc.scope.pool || sc.scope.round ? 2 : 1;
      groups.at(-1)?.options.push({ index, scope: sc, depth });
    });
    return groups;
  });

  /** The scope chosen, null before. */
  protected readonly selected = computed(() => (this.view().scopes ?? [])[this.scope()] ?? null);

  /** The rule ready to add, null while incomplete. */
  protected readonly rule = computed<CourtRule | null>(() => {
    const sc = this.selected();
    const courts = this.courts().filter((n) => n <= this.view().courts);
    if (!sc || courts.length === 0) {
      return null;
    }
    return {
      kind: this.kind(),
      level: this.kind() === CourtRuleKind.CourtPriority ? this.level() : 0,
      scope: sc.scope,
    };
  });

  private previewSeq = 0;

  constructor() {
    // The timeline previews the rule being chosen, again after each change
    // of the tournament.
    effect(() => {
      const rule = this.rule();
      const courts = this.courts();
      this.view().now;
      const seq = ++this.previewSeq;
      if (!rule) {
        untracked(() => this.preview.emit(null));
        return;
      }
      void TournamentService.PreviewCourtRule(courts, rule).then(
        (v) => seq === this.previewSeq && this.preview.emit(v),
        () => seq === this.previewSeq && this.preview.emit(null),
      );
    });
  }

  protected t(key: string, params?: Record<string, unknown>): string {
    return this.transloco.translate(key, params);
  }

  protected label(sc: CourtScopeView): string {
    return scopeLabel((k, p) => this.t(k, p), sc);
  }

  /** An option of the target picker, indented under its category and stage. */
  protected optionLabel(o: { scope: CourtScopeView; depth: number }): string {
    return '   '.repeat(o.depth) + this.label(o.scope);
  }

  /** "Priority 1", "Only", "Never". */
  protected kindLabel(r: CourtRuleView): string {
    return this.t('schedule.rules.chip.' + r.kind, { level: r.level });
  }

  protected toggleCourt(n: number): void {
    this.courts.update((cs) =>
      cs.includes(n) ? cs.filter((c) => c !== n) : [...cs, n].sort((a, b) => a - b),
    );
  }

  protected setLevel(level: number): void {
    this.level.set(Math.min(MAX_LEVEL, Math.max(1, level)));
  }

  protected async add(): Promise<void> {
    const rule = this.rule();
    if (rule && (await this.store.addCourtRule(this.courts(), rule))) {
      this.reset();
    }
  }

  protected reset(): void {
    this.courts.set([]);
    this.scope.set(-1);
  }

  protected remove(court: number, r: CourtRuleView): void {
    void this.store.removeCourtRule(court, r.scope.scope);
  }

  protected clear(court: number): void {
    void this.store.clearCourtRules(court);
  }
}
