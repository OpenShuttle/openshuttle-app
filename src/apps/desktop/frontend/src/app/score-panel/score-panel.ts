import { Component, ElementRef, computed, effect, inject, input, output, signal, viewChildren } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { MatchView } from '@bindings/apps/desktop';
import { EntrantID, Score, WinConditionKind } from '@bindings/internal/engine';

import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';

/** Builds the score from the typed points, ignoring sets left empty. */
export function scoreFrom(points: string[][], goldenPoint: boolean): Score {
  const sets = [];
  for (let i = 0; i < points[0].length; i++) {
    const a = points[0][i].trim();
    const b = points[1][i].trim();
    if (a === '' && b === '') {
      continue;
    }
    sets.push({ a: Number(a || 0), b: Number(b || 0) });
  }
  return { sets, goldenPoint };
}

/** Score entry for one match: opened from a court or the pending list. */
@Component({
  selector: 'app-score-panel',
  imports: [TranslocoDirective, CategoryBadge],
  templateUrl: './score-panel.html',
  styleUrl: './score-panel.scss',
  host: { '(keydown.escape)': 'closed.emit()' },
})
export class ScorePanel {
  readonly match = input.required<MatchView>();
  readonly closed = output<void>();

  private readonly tournament = inject(TournamentStore);
  private readonly fields = viewChildren<ElementRef<HTMLInputElement>>('point');

  protected readonly timed = computed(() => this.match().win.kind === WinConditionKind.WinByTime);
  protected readonly setCount = computed(() => {
    const w = this.match().win;
    return w.kind === WinConditionKind.WinByTime ? 1 : 2 * (w.setsToWin ?? 1) - 1;
  });
  /** points[side][set], as typed. */
  protected readonly points = signal<string[][]>([[], []]);
  protected readonly goldenPoint = signal(false);
  /** Entrant whose forfeit waits for a second click to confirm. */
  protected readonly confirmingForfeit = signal<EntrantID | null>(null);

  constructor() {
    // Start from the recorded score when correcting one.
    effect(() => {
      const n = this.setCount();
      const sets = this.match().score?.sets ?? [];
      this.points.set(
        [0, 1].map((side) =>
          Array.from({ length: n }, (_, i) => {
            const set = sets[i];
            return set ? String(side === 0 ? set.a : set.b) : '';
          }),
        ),
      );
      this.goldenPoint.set(this.match().score?.goldenPoint ?? false);
    });
    effect(() => this.fields()[0]?.nativeElement.focus());
  }

  protected set(side: number, set: number, event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    this.points.update((p) => p.map((row, s) => (s === side ? row.map((v, i) => (i === set ? value : v)) : row)));
  }

  protected async save(): Promise<void> {
    const score = scoreFrom(this.points(), this.timed() && this.goldenPoint());
    if (await this.tournament.recordScore(this.match().id, score)) {
      this.closed.emit();
    }
  }

  /**
   * A forfeit turns every pool match of the entrant into a walkover: it takes
   * a second click, and the buttons stay out of the Tab order, so typing a
   * score and pressing Enter can never declare one.
   */
  protected async walkover(entrant: EntrantID): Promise<void> {
    if (this.confirmingForfeit() !== entrant) {
      this.confirmingForfeit.set(entrant);
      return;
    }
    if (await this.tournament.withdrawEntrant(entrant)) {
      this.closed.emit();
    }
  }

  protected sets(): number[] {
    return Array.from({ length: this.setCount() }, (_, i) => i);
  }
}
