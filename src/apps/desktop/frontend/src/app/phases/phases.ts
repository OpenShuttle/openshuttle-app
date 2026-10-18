import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective } from '@jsverse/transloco';
import {
  BracketPhase,
  MatchView,
  PhaseCategory,
  PhasesView,
  PoolPhase,
  RoundView,
  SideView,
  TournamentService,
} from '@bindings/apps/desktop';
import { Category, WinConditionKind } from '@bindings/internal/engine';

import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { HistoryPanel } from '../history/history-panel';
import { ScorePanel } from '../score-panel/score-panel';
import { roundLabel } from '../shared/match-stage';
import { shortNames } from '../shared/names';

/** Phases: pool standings and matches, with a correction mode. */
@Component({
  selector: 'app-phases',
  imports: [TranslocoDirective, RouterLink, ScorePanel, HistoryPanel],
  templateUrl: './phases.html',
  styleUrl: './phases.scss',
})
export class Phases {
  protected readonly tournament = inject(TournamentStore);
  protected readonly view = liveView<PhasesView>(() => TournamentService.GetPhasesView(), { categories: [] });

  private readonly selected = signal<Category | null>(null);
  protected readonly current = computed(() => {
    const categories = this.view().categories ?? [];
    return categories.find((c) => c.category === this.selected()) ?? categories[0] ?? null;
  });

  protected readonly correction = signal(false);
  protected readonly showHistory = signal(true);
  /** Stage shown: 'pools' or a bracket id. */
  private readonly stageKey = signal('pools');
  protected readonly bracket = computed(
    () => (this.current()?.brackets ?? []).find((b) => b.id === this.stageKey()) ?? null,
  );
  protected readonly scoring = signal<MatchView | null>(null);
  /** First match picked for a swap. */
  protected readonly swapping = signal<MatchView | null>(null);
  /** First bracket slot picked for a swap: "<bracket>:<slot>". */
  protected readonly swappingSlot = signal<string | null>(null);

  protected select(category: Category): void {
    this.selected.set(category);
    this.stageKey.set('pools');
  }

  protected selectStage(key: string): void {
    this.stageKey.set(key);
  }

  protected played(pool: PoolPhase): number {
    return (pool.matches ?? []).filter((m) => m.score || m.walkover).length;
  }

  protected short(side: SideView): string {
    return shortNames(side);
  }

  protected toggleCorrection(): void {
    this.correction.update((on) => !on);
    this.swapping.set(null);
    this.swappingSlot.set(null);
  }

  protected round(r: RoundView): { key: string; params: Record<string, number> } {
    return roundLabel({ placeFrom: r.from, placeTo: r.to, roundSize: r.size });
  }

  protected bracketStatus(b: BracketPhase): string {
    if (!b.drawn) {
      return 'notDrawn';
    }
    const total = b.rounds?.reduce((n, r) => n + (r.matches ?? []).filter((m) => !m.byeA && !m.byeB).length, 0) ?? 0;
    return (b.places ?? []).length > 0 && b.started && total > 0 && this.allPlayed(b) ? 'finished' : b.started ? 'inProgress' : 'ready';
  }

  private allPlayed(b: BracketPhase): boolean {
    return (b.rounds ?? []).every((r) => (r.matches ?? []).every((m) => m.byeA || m.byeB || m.score || m.walkover));
  }

  /** Side label inside a bracket: names, "bye", or "to be decided". */
  protected sideLabel(side: SideView, bye: boolean): { key?: string; text?: string } {
    if (bye) {
      return { key: 'phases.bye' };
    }
    if (!side.entrant) {
      return { key: 'phases.tbd' };
    }
    return { text: `#${side.bib} ${shortNames(side)}` };
  }

  protected place(from: number, to: number): string {
    return from === to ? String(from) : `${from}–${to}`;
  }

  protected async swapSlot(c: PhaseCategory, b: BracketPhase, slot: number): Promise<void> {
    const key = `${b.id}:${slot}`;
    const first = this.swappingSlot();
    if (!first || !first.startsWith(b.id + ':')) {
      this.swappingSlot.set(key);
      return;
    }
    const a = Number(first.split(':')[1]);
    if (a === slot || (await this.tournament.swapBracketSlots(c.category, b.id, a, slot))) {
      this.swappingSlot.set(null);
    }
  }

  protected names(side: SideView): string {
    return (side.names ?? []).join(' / ');
  }

  protected diff(won: number, lost: number): string {
    const d = won - lost;
    return d > 0 ? `+${d}` : String(d);
  }

  /** Side that won a scored match, by sets. */
  protected winner(m: MatchView): 'a' | 'b' | '' {
    let a = 0;
    let b = 0;
    for (const set of m.score?.sets ?? []) {
      if (set.a > set.b) a++;
      else if (set.b > set.a) b++;
    }
    return a > b ? 'a' : b > a ? 'b' : '';
  }

  protected scoreText(m: MatchView): string {
    return (m.score?.sets ?? []).map((s) => `${s.a}-${s.b}`).join(' · ');
  }

  /** Waiting: not called and not decided, so it can be swapped. */
  protected waiting(m: MatchView): boolean {
    return !m.court && !m.score && !m.walkover;
  }

  protected winKey(c: PhaseCategory): string {
    return c.stage.win.kind === WinConditionKind.WinByTime ? 'phases.winTimed' : 'phases.winSets';
  }

  protected async swap(m: MatchView): Promise<void> {
    const first = this.swapping();
    if (!first) {
      this.swapping.set(m);
      return;
    }
    if (first.id === m.id || (await this.tournament.swapMatches(first.id, m.id))) {
      this.swapping.set(null);
    }
  }
}
