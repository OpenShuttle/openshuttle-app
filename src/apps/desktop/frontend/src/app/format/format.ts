import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { BracketView, FormatCategory, FormatView, TournamentService } from '@bindings/apps/desktop';
import { Category, Tiebreak, WinCondition, WinConditionKind } from '@bindings/internal/engine';
import { BracketConfig, PoolStage } from '@bindings/internal/tournament';

import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';

const MAX_POOLS = 16;
const DEFAULT_SETS: WinCondition = { kind: WinConditionKind.WinBySets, setsToWin: 2, pointsPerSet: 21, cap: 30 };
const DEFAULT_TIMED: WinCondition = { kind: WinConditionKind.WinByTime, minutes: 12 };

/** Format: the pool stage of each category. Every change is saved at once. */
@Component({
  selector: 'app-format',
  imports: [TranslocoDirective, RouterLink, CategoryBadge],
  templateUrl: './format.html',
  styleUrl: './format.scss',
})
export class Format {
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);
  protected readonly view = liveView<FormatView>(() => TournamentService.GetFormatView(), {
    categories: [],
  });
  protected readonly maxPools = MAX_POOLS;
  protected readonly kinds = WinConditionKind;

  private readonly selected = signal<Category | null>(null);
  protected readonly current = computed(() => {
    const categories = this.view().categories ?? [];
    return categories.find((c) => c.category === this.selected()) ?? categories[0] ?? null;
  });

  protected select(category: Category): void {
    this.selected.set(category);
  }

  protected setPools(fc: FormatCategory, pools: number): void {
    if (pools >= 1 && pools <= MAX_POOLS) {
      this.save(fc, { pools });
    }
  }

  protected setKind(fc: FormatCategory, kind: WinConditionKind): void {
    if (kind !== fc.stage.win.kind) {
      this.save(fc, { win: kind === WinConditionKind.WinByTime ? DEFAULT_TIMED : DEFAULT_SETS });
    }
  }

  protected setWin(fc: FormatCategory, field: keyof WinCondition, event: Event): void {
    const value = Number((event.target as HTMLInputElement | HTMLSelectElement).value);
    if (Number.isInteger(value)) {
      this.save(fc, { win: { ...fc.stage.win, [field]: value } });
    }
  }

  /** Moves a tiebreak one step up (-1) or down (+1). */
  protected moveTiebreak(fc: FormatCategory, index: number, step: -1 | 1): void {
    const tiebreaks = [...(fc.stage.tiebreaks ?? [])];
    const target = index + step;
    if (target < 0 || target >= tiebreaks.length) {
      return;
    }
    [tiebreaks[index], tiebreaks[target]] = [tiebreaks[target], tiebreaks[index]];
    this.save(fc, { tiebreaks });
  }

  protected tiebreaks(fc: FormatCategory): Tiebreak[] {
    return fc.stage.tiebreaks ?? [];
  }

  protected addBracket(fc: FormatCategory): void {
    const n = (fc.brackets ?? []).length;
    const name = this.transloco.translate(n === 0 ? 'format.mainBracket' : 'format.bracketN', { n: n + 1 });
    void this.tournament.addBracket(fc.category, { name, win: { ...DEFAULT_SETS }, fullPlacement: true });
  }

  protected setBracket(fc: FormatCategory, b: BracketView, change: Partial<BracketConfig>): void {
    const cfg: BracketConfig = { name: b.name, win: b.win, fullPlacement: b.fullPlacement };
    void this.tournament.configureBracket(fc.category, b.id, { ...cfg, ...change });
  }

  protected setBracketKind(fc: FormatCategory, b: BracketView, kind: WinConditionKind): void {
    if (kind !== b.win.kind) {
      this.setBracket(fc, b, { win: kind === WinConditionKind.WinByTime ? DEFAULT_TIMED : DEFAULT_SETS });
    }
  }

  protected setBracketWin(fc: FormatCategory, b: BracketView, field: keyof WinCondition, event: Event): void {
    const value = Number((event.target as HTMLInputElement | HTMLSelectElement).value);
    if (Number.isInteger(value)) {
      this.setBracket(fc, b, { win: { ...b.win, [field]: value } });
    }
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }

  /** Pool positions 1..rows with their destination ("" for eliminated). */
  protected routes(fc: FormatCategory): { position: number; target: string }[] {
    return Array.from({ length: fc.routingRows }, (_, i) => ({ position: i + 1, target: fc.routing?.[i] ?? '' }));
  }

  protected setRoute(fc: FormatCategory, position: number, event: Event): void {
    const routing = this.routes(fc).map((r) => r.target);
    routing[position - 1] = (event.target as HTMLSelectElement).value;
    void this.tournament.setRouting(fc.category, routing);
  }

  private save(fc: FormatCategory, change: Partial<PoolStage>): void {
    void this.tournament.configurePoolStage(fc.category, { ...fc.stage, ...change });
  }
}
