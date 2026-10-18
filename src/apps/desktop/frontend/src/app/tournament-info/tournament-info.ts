import { Component, computed, effect, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { RouterLink } from '@angular/router';
import { Category } from '@bindings/internal/engine';

import { TournamentStore } from '../core/tournament.service';

const MAX_CATEGORIES = 2;

/** Tournament info: name, categories, courts, rest and file. */
@Component({
  selector: 'app-tournament-info',
  imports: [TranslocoDirective, RouterLink],
  templateUrl: './tournament-info.html',
  styleUrl: './tournament-info.scss',
})
export class TournamentInfo {
  protected readonly tournament = inject(TournamentStore);
  protected readonly name = signal('');
  protected readonly maxCategories = MAX_CATEGORIES;
  protected readonly allCategories: Category[] = ['MS', 'WS', 'MD', 'WD', 'XD'] as Category[];

  /** Categories Go still accepts: none sharing a sex with one already added. */
  protected readonly available = computed(() => this.tournament.view().availableCategories ?? []);
  protected readonly full = computed(
    () => (this.tournament.view().categories ?? []).length >= MAX_CATEGORIES,
  );

  constructor() {
    // Follow the stored name: after a rename, an undo, or another window.
    effect(() => this.name.set(this.tournament.view().name));
  }

  protected has(c: Category): boolean {
    return (this.tournament.view().categories ?? []).some((r) => r.category === c);
  }

  /** A category can be added when Go allows it and there is room. */
  protected canAdd(c: Category): boolean {
    return !this.full() && this.available().includes(c);
  }

  protected toggle(c: Category): void {
    void (this.has(c) ? this.tournament.removeCategory(c) : this.tournament.addCategory(c));
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
