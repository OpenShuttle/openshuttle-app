import { Component, computed, effect, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { Category } from '@bindings/internal/engine';

import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';

const ALL_CATEGORIES = [
  Category.MensSingles,
  Category.WomensSingles,
  Category.MensDoubles,
  Category.WomensDoubles,
  Category.MixedDoubles,
];
const MAX_CATEGORIES = 2;

/** Tournament info: name, file and categories. */
@Component({
  selector: 'app-tournament-info',
  imports: [TranslocoDirective, CategoryBadge],
  templateUrl: './tournament-info.html',
  styleUrl: './tournament-info.scss',
})
export class TournamentInfo {
  protected readonly tournament = inject(TournamentStore);
  protected readonly name = signal('');
  protected readonly maxCategories = MAX_CATEGORIES;

  protected readonly available = computed(() => {
    const used = (this.tournament.view().categories ?? []).map((c) => c.category);
    return ALL_CATEGORIES.filter((c) => !used.includes(c));
  });
  protected readonly full = computed(
    () => (this.tournament.view().categories ?? []).length >= MAX_CATEGORIES,
  );

  constructor() {
    // Follow the stored name: after a rename, an undo, or another window.
    effect(() => this.name.set(this.tournament.view().name));
  }

  protected save(): void {
    if (this.name() !== this.tournament.view().name) {
      void this.tournament.rename(this.name());
    }
  }

  protected reset(): void {
    this.name.set(this.tournament.view().name);
  }
}
