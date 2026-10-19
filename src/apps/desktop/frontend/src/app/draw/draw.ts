import { Component, HostListener, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective } from '@jsverse/transloco';
import { DrawCategory, DrawEntrantView, DrawView, TournamentService } from '@bindings/apps/desktop';
import { EntrantID } from '@bindings/internal/engine';

import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';

/** Pointer travel before a press becomes a drag, in pixels. */
const DRAG_THRESHOLD = 5;

interface Drag {
  id: EntrantID;
  x: number;
  y: number;
  moved: boolean;
}

/** The pool element under a point: "<category>:<index>" from data-pool. */
function poolAt(x: number, y: number): string | null {
  const el = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-pool]');
  return el?.dataset['pool'] ?? null;
}

/** Where a dragged seed lands: before a ranking row, at the end of the ranking, or out of it. */
interface SeedTarget {
  category: string;
  list: 'ranking' | 'others';
  index: number;
}

function seedTargetAt(x: number, y: number): SeedTarget | null {
  const el = document.elementFromPoint(x, y);
  const row = el?.closest<HTMLElement>('[data-rank]');
  if (row) {
    const [category, index] = (row.dataset['rank'] ?? '').split(':');
    const box = row.getBoundingClientRect();
    // The lower half of a row drops after it.
    return { category, list: 'ranking', index: Number(index) + (y > box.top + box.height / 2 ? 1 : 0) };
  }
  const end = el?.closest<HTMLElement>('[data-rank-end]');
  if (end) {
    return { category: end.dataset['rankEnd'] ?? '', list: 'ranking', index: Number.MAX_SAFE_INTEGER };
  }
  const others = el?.closest<HTMLElement>('[data-unseeded]');
  return others ? { category: others.dataset['unseeded'] ?? '', list: 'others', index: -1 } : null;
}

/** Draw: pools of each category, seeds, drag and drop, validation. */
@Component({
  selector: 'app-draw',
  imports: [TranslocoDirective, RouterLink],
  templateUrl: './draw.html',
  styleUrl: './draw.scss',
})
export class Draw {
  protected readonly tournament = inject(TournamentStore);
  protected readonly view = liveView<DrawView>(() => TournamentService.GetDrawView(), {
    categories: [],
  });

  /** Pool hovered while dragging: "<category>:<index>". */
  protected readonly dropTarget = signal<string | null>(null);
  protected readonly Math = Math;

  /** Full names, always: initials leave the desk guessing who is who. */
  protected names(e: DrawEntrantView): string {
    return (e.players ?? []).map((p) => p.name).join(' / ');
  }

  /** Entrants sharing their pool with a club mate. */
  protected conflicts(c: DrawCategory): number {
    return (c.pools ?? []).reduce(
      (n, pool) => n + (pool.entrants ?? []).filter((e) => (e.sameClub ?? []).length > 0).length,
      0,
    );
  }

  protected entrantCount(c: DrawCategory): number {
    return (c.pools ?? []).reduce((n, pool) => n + (pool.entrants ?? []).length, 0) + (c.unplaced ?? []).length;
  }

  protected draw(c: DrawCategory): void {
    void this.tournament.drawEntry(c.category);
  }

  /** First slot picked for a swap in a bracket draw: "<category>:<slot>". */
  protected readonly swapping = signal<string | null>(null);

  protected async pickSlot(c: DrawCategory, slot: number): Promise<void> {
    const key = `${c.category}:${slot}`;
    const first = this.swapping();
    if (!first || !first.startsWith(c.category + ':')) {
      this.swapping.set(key);
      return;
    }
    const a = Number(first.split(':')[1]);
    if (a === slot || (await this.tournament.swapSlots(c.category, c.stage, a, slot))) {
      this.swapping.set(null);
    }
  }

  /** First-round pairs of a bracket draw: slots 0-1, 2-3… */
  protected pairs(c: DrawCategory): { slot: number; entrant: DrawEntrantView }[][] {
    const slots = c.pools?.[0]?.entrants ?? [];
    const out: { slot: number; entrant: DrawEntrantView }[][] = [];
    for (let i = 0; i < slots.length; i += 2) {
      out.push([
        { slot: i, entrant: slots[i] },
        { slot: i + 1, entrant: slots[i + 1] },
      ]);
    }
    return out;
  }

  // Seeds: a ranking, top seed first. Every change sends the whole ranking.
  private ranking(c: DrawCategory): EntrantID[] {
    return (c.ranking ?? []).map((e) => e.id);
  }

  protected moveSeed(c: DrawCategory, index: number, delta: number): void {
    const ids = this.ranking(c);
    const [id] = ids.splice(index, 1);
    ids.splice(index + delta, 0, id);
    void this.tournament.setSeeds(c.category, ids);
  }

  protected removeSeed(c: DrawCategory, index: number): void {
    void this.tournament.setSeeds(c.category, this.ranking(c).filter((_, i) => i !== index));
  }

  protected addSeed(c: DrawCategory, id: EntrantID): void {
    void this.tournament.setSeeds(c.category, [...this.ranking(c), id]);
  }

  /** Entrant dragged in the seed lists, and where it would land. */
  protected readonly seedDragging = signal<EntrantID | null>(null);
  protected readonly seedTarget = signal<SeedTarget | null>(null);
  private seedDrag: (Drag & { category: DrawCategory }) | null = null;

  protected onSeedDown(event: PointerEvent, c: DrawCategory, id: EntrantID): void {
    if (event.button === 0 && !(event.target as HTMLElement).closest('button')) {
      this.seedDrag = { id, category: c, x: event.clientX, y: event.clientY, moved: false };
    }
  }

  private dropSeed(c: DrawCategory, id: EntrantID, target: SeedTarget): void {
    const ids = this.ranking(c);
    const from = ids.indexOf(id);
    if (target.list === 'others') {
      if (from >= 0) {
        void this.tournament.setSeeds(c.category, ids.filter((x) => x !== id));
      }
      return;
    }
    let to = Math.min(target.index, ids.length);
    if (from >= 0) {
      ids.splice(from, 1);
      if (from < to) {
        to--;
      }
    }
    ids.splice(to, 0, id);
    if (ids.length <= 32) {
      void this.tournament.setSeeds(c.category, ids);
    }
  }

  /**
   * Drag and drop with pointer events rather than HTML5 drag and drop, which
   * behaves differently across WebKitGTK, WebView2 and WKWebView.
   */
  protected readonly dragging = signal<EntrantID | null>(null);
  private drag: Drag | null = null;

  protected onPointerDown(event: PointerEvent, id: EntrantID, draft: boolean): void {
    if (draft && event.button === 0) {
      this.drag = { id, x: event.clientX, y: event.clientY, moved: false };
    }
  }

  @HostListener('document:pointermove', ['$event'])
  protected onPointerMove(event: PointerEvent): void {
    const seed = this.seedDrag;
    if (seed) {
      if (!seed.moved && Math.hypot(event.clientX - seed.x, event.clientY - seed.y) < DRAG_THRESHOLD) {
        return;
      }
      seed.moved = true;
      event.preventDefault();
      this.seedDragging.set(seed.id);
      this.seedTarget.set(seedTargetAt(event.clientX, event.clientY));
      return;
    }
    const drag = this.drag;
    if (!drag) {
      return;
    }
    if (!drag.moved && Math.hypot(event.clientX - drag.x, event.clientY - drag.y) < DRAG_THRESHOLD) {
      return;
    }
    drag.moved = true;
    event.preventDefault();
    this.dragging.set(drag.id);
    this.dropTarget.set(poolAt(event.clientX, event.clientY));
  }

  @HostListener('document:pointerup', ['$event'])
  protected onPointerUp(event: PointerEvent): void {
    const seed = this.seedDrag;
    if (seed) {
      this.seedDrag = null;
      this.seedDragging.set(null);
      this.seedTarget.set(null);
      const target = seed.moved ? seedTargetAt(event.clientX, event.clientY) : null;
      if (target && target.category === seed.category.category) {
        this.dropSeed(seed.category, seed.id, target);
      }
      return;
    }
    const drag = this.drag;
    this.drag = null;
    this.dragging.set(null);
    this.dropTarget.set(null);
    const target = drag?.moved ? poolAt(event.clientX, event.clientY) : null;
    if (drag && target) {
      void this.tournament.moveEntrant(drag.id, Number(target.split(':')[1]));
    }
  }
}
