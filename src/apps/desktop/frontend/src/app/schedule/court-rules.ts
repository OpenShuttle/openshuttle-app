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
import { CourtRule, CourtRuleKind, CourtScope } from '@bindings/internal/tournament';

import { Panels } from '../core/panels.service';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { shortRoundLabel } from '../shared/match-stage';

/** The highest priority level (1 goes first), as Go's MaxCourtPriority. */
const MAX_LEVEL = 9;

/** Identifies a scope among the view's: its category, stage, pool and round. */
export function scopeKey(sc: CourtScope): string {
  return [sc.category, sc.stage ?? '', sc.pool ?? '', sc.round ?? 0, sc.from ?? 0].join('/');
}

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

  /** The rules being chosen: their courts, kind, level and targets (scope keys, in the order picked). */
  protected readonly courts = signal<number[]>([]);
  protected readonly kind = signal<CourtRuleKind>(CourtRuleKind.CourtPriority);
  protected readonly level = signal(1);
  protected readonly targets = signal<string[]>([]);

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

  /** The scopes grouped by category, for the target picker; the ones picked already are disabled. */
  protected readonly scopeGroups = computed(() => {
    const picked = new Set(this.targets());
    const groups: {
      category: CourtScopeView;
      options: { key: string; scope: CourtScopeView; depth: number; picked: boolean }[];
    }[] = [];
    for (const sc of this.view().scopes ?? []) {
      if (!sc.scope.stage) {
        groups.push({ category: sc, options: [] });
      }
      const depth = !sc.scope.stage ? 0 : sc.scope.pool || sc.scope.round ? 2 : 1;
      const key = scopeKey(sc.scope);
      groups.at(-1)?.options.push({ key, scope: sc, depth, picked: picked.has(key) });
    }
    return groups;
  });

  /** The targets picked, still in the tournament, in the order picked. */
  protected readonly selected = computed(() => {
    const byKey = new Map((this.view().scopes ?? []).map((sc) => [scopeKey(sc.scope), sc]));
    return this.targets().flatMap((k) => byKey.get(k) ?? []);
  });

  /** The rules ready to add, one per target; null while incomplete. */
  protected readonly rules = computed<CourtRule[] | null>(() => {
    const targets = this.selected();
    const courts = this.courts().filter((n) => n <= this.view().courts);
    if (targets.length === 0 || courts.length === 0) {
      return null;
    }
    const level = this.kind() === CourtRuleKind.CourtPriority ? this.level() : 0;
    return targets.map((sc) => ({ kind: this.kind(), level, scope: sc.scope }));
  });

  private previewSeq = 0;

  constructor() {
    // The timeline previews the rules being chosen, again after each change
    // of the tournament.
    effect(() => {
      const rules = this.rules();
      const courts = this.courts();
      // Reading the view's time refetches the preview after each update.
      const live = this.view().now !== '';
      const seq = ++this.previewSeq;
      if (!rules || !live) {
        untracked(() => this.preview.emit(null));
        return;
      }
      void TournamentService.PreviewCourtRules(courts, rules).then(
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

  /** Picks a target from the picker, which goes back to its prompt. */
  protected pickTarget(select: HTMLSelectElement): void {
    const key = select.value;
    select.value = '';
    if (key && !this.targets().includes(key)) {
      this.targets.update((ts) => [...ts, key]);
    }
  }

  protected unpickTarget(sc: CourtScopeView): void {
    const key = scopeKey(sc.scope);
    this.targets.update((ts) => ts.filter((k) => k !== key));
  }

  protected async add(): Promise<void> {
    const rules = this.rules();
    if (rules && (await this.store.addCourtRules(this.courts(), rules))) {
      this.reset();
    }
  }

  protected reset(): void {
    this.courts.set([]);
    this.targets.set([]);
  }

  protected remove(court: number, r: CourtRuleView): void {
    void this.store.removeCourtRule(court, r.scope.scope);
  }

  protected clear(court: number): void {
    void this.store.clearCourtRules(court);
  }
}
