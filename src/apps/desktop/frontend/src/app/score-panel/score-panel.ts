import { Component, ElementRef, computed, effect, inject, input, output, signal, viewChild, viewChildren } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { MatchView, SideView, TournamentService } from '@bindings/apps/desktop';
import { EntrantID, Score, WinConditionKind } from '@bindings/internal/engine';

import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { MatchStage } from '../shared/match-stage';
import { clubs, fullNames } from '../shared/names';

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

/** The sets typed in full on both sides, from the first one up to the first gap. */
export function typedSets(points: string[][]): Score['sets'] {
  const sets = [];
  for (let i = 0; i < points[0].length; i++) {
    const a = points[0][i].trim();
    const b = points[1][i].trim();
    if (a === '' || b === '') {
      break;
    }
    sets.push({ a: Number(a), b: Number(b) });
  }
  return sets;
}

/** Score entry for one match: opened from a court or the pending list. */
@Component({
  selector: 'app-score-panel',
  imports: [TranslocoDirective, CategoryBadge, MatchStage],
  templateUrl: './score-panel.html',
  styleUrl: './score-panel.scss',
  host: { '(keydown.escape)': 'closed.emit()' },
})
export class ScorePanel {
  readonly match = input.required<MatchView>();
  readonly closed = output<void>();

  private readonly tournament = inject(TournamentStore);
  private readonly fields = viewChildren<ElementRef<HTMLInputElement>>('point');
  private readonly saveButton = viewChild<ElementRef<HTMLButtonElement>>('saveButton');
  private decideSeq = 0;

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
  /**
   * How many sets decide the match, as Go reads the typed score (0: not
   * decided yet). The sets after them and the forfeit are disabled, so Tab
   * goes from the deciding set straight to saving.
   */
  protected readonly decidedAfter = signal(0);

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
    effect(() => {
      const win = this.match().win;
      const sets = typedSets(this.points());
      const seq = ++this.decideSeq;
      void TournamentService.ScoreDecidedAfter(win, { sets, goldenPoint: false }).then(
        (n) => seq === this.decideSeq && this.decide(n),
        () => seq === this.decideSeq && this.decide(0),
      );
    });
  }

  private decide(n: number): void {
    // Tab may have reached a set the answer now disables: go on to saving.
    const active = document.activeElement;
    const fields = this.fields().map((f) => f.nativeElement);
    const lost = n > 0 && fields.some((f, k) => f === active && Math.floor(k / 2) >= n);
    this.decidedAfter.set(n);
    if (lost) {
      setTimeout(() => this.saveButton()?.nativeElement.focus());
    }
  }

  /** A set after the ones deciding the match: never played. */
  protected unplayed(set: number): boolean {
    const n = this.decidedAfter();
    return n > 0 && set >= n;
  }

  protected set(side: number, set: number, event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    this.points.update((p) => p.map((row, s) => (s === side ? row.map((v, i) => (i === set ? value : v)) : row)));
  }

  protected async save(): Promise<void> {
    const n = this.decidedAfter();
    const points = n > 0 ? this.points().map((row) => row.slice(0, n)) : this.points();
    const score = scoreFrom(points, this.timed() && this.goldenPoint());
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

  protected names(side: SideView): string {
    return fullNames(side);
  }

  protected clubs(side: SideView): string {
    return clubs(side);
  }

  protected sets(): number[] {
    return Array.from({ length: this.setCount() }, (_, i) => i);
  }
}
