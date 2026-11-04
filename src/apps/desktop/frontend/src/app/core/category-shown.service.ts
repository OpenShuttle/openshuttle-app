import { Injectable, signal } from '@angular/core';
import { Category } from '@bindings/internal/engine';

export const CATEGORY_SHOWN_KEY = 'openshuttle.categoryShown';

/** What the category selectors last showed. */
interface Shown {
  /** Last value chosen on a selector offering "all". */
  last: Category | 'all';
  /** Last specific category chosen on any selector. */
  category: Category | null;
}

function load(): Shown {
  try {
    const s = JSON.parse(localStorage.getItem(CATEGORY_SHOWN_KEY) ?? '{}') as Partial<Shown>;
    return { last: s.last ?? 'all', category: s.category ?? null };
  } catch {
    return { last: 'all', category: null };
  }
}

/**
 * The category the screens show, shared so that going to another screen keeps
 * the one last displayed. Screens offering "all" take the last value chosen,
 * the others the last specific category; a category the open tournament does
 * not have falls back to the screen's default. A convenience of this PC: kept
 * in local storage, like the folded side panels.
 */
@Injectable({ providedIn: 'root' })
export class CategoryShown {
  private readonly state = signal<Shown>(load());

  /** The value of a selector offering "all", among the tournament's categories. */
  orAll(categories: readonly { category: Category }[] | null | undefined): Category | 'all' {
    const last = this.state().last;
    return last !== 'all' && (categories ?? []).some((c) => c.category === last) ? last : 'all';
  }

  /** The shown item of a selector without "all": the stored category, else the first. */
  pick<T extends { category: Category }>(categories: readonly T[] | null | undefined): T | null {
    const list = categories ?? [];
    return list.find((c) => c.category === this.state().category) ?? list[0] ?? null;
  }

  /** Chosen on any selector: "all" or a category. */
  show(value: Category | 'all'): void {
    this.save(value === 'all' ? { ...this.state(), last: 'all' } : { last: value, category: value });
  }

  /**
   * Chosen on a selector without "all" sitting beside one that has it (the
   * add form of the Entrants screen): keeps the other selector's value.
   */
  showOnly(category: Category): void {
    this.save({ ...this.state(), category });
  }

  private save(shown: Shown): void {
    this.state.set(shown);
    try {
      localStorage.setItem(CATEGORY_SHOWN_KEY, JSON.stringify(shown));
    } catch {
      // Not saved: the screens still follow it for this session.
    }
  }
}
