import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective } from '@jsverse/transloco';
import { MatchView, PhaseCategory, PhasesView, SideView, TournamentService } from '@bindings/apps/desktop';
import { Category, WinConditionKind } from '@bindings/internal/engine';

import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { HistoryPanel } from '../history/history-panel';
import { ScorePanel } from '../score-panel/score-panel';
import { CategoryBadge } from '../shared/category-badge';

/** Phases: pool standings and matches, with a correction mode. */
@Component({
  selector: 'app-phases',
  imports: [TranslocoDirective, RouterLink, CategoryBadge, ScorePanel, HistoryPanel],
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
  protected readonly showHistory = signal(false);
  protected readonly scoring = signal<MatchView | null>(null);
  /** First match picked for a swap. */
  protected readonly swapping = signal<MatchView | null>(null);

  protected select(category: Category): void {
    this.selected.set(category);
  }

  protected toggleCorrection(): void {
    this.correction.update((on) => !on);
    this.swapping.set(null);
  }

  protected names(side: SideView): string {
    return (side.names ?? []).join(' / ');
  }

  protected diff(won: number, lost: number): string {
    const d = won - lost;
    return d > 0 ? `+${d}` : String(d);
  }

  protected scoreText(m: MatchView): string {
    return (m.score?.sets ?? []).map((s) => `${s.a}-${s.b}`).join('  ');
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
