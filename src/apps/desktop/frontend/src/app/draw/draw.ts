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

  protected names(e: DrawEntrantView): string {
    return (e.players ?? [])
      .map((p) => (p.firstName ? `${p.firstName[0]}. ${p.lastName}` : p.lastName))
      .join(' / ');
  }

  protected fullNames(e: DrawEntrantView): string {
    return (e.players ?? []).map((p) => `${p.firstName} ${p.lastName}`).join(' / ');
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

  protected setSeed(e: DrawEntrantView, event: Event): void {
    const seed = Number((event.target as HTMLSelectElement).value);
    void this.tournament.setSeed(e.id, seed);
  }

  protected seedOptions(c: DrawCategory): number[] {
    return Array.from({ length: Math.min(32, (c.seeded ?? []).length) }, (_, i) => i + 1);
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
