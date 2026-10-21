import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TranslocoDirective } from '@jsverse/transloco';

import { TournamentStore } from '../core/tournament.service';
import { LanguageService } from '../core/language.service';
import { Panels } from '../core/panels.service';
import { Clock } from '../core/clock';

/** done: ✓; current: the step the desk is on; todo: an empty ring. */
type Mark = 'done' | 'current' | 'todo';

interface NavItem {
  key: string;
  path: string;
  mark: Mark;
  /** Translation key and params of the line under the label. */
  sub: string;
  params?: Record<string, unknown>;
}

/** Icon of each step (24×24 stroke paths), shown when the menu is folded. */
const ICONS: Record<string, string> = {
  info: 'M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18M12 11v5M12 7.5v.5',
  entrants: 'M9 11a3.5 3.5 0 1 0 0-7a3.5 3.5 0 1 0 0 7M2.5 20c.6-3.5 3.2-5.5 6.5-5.5s5.9 2 6.5 5.5M16 4.5a3.5 3.5 0 0 1 0 6.5M18.5 14.8c1.7.8 2.7 2.6 3 5.2',
  format: 'M3 4h6v5H3zM15 4h6v5h-6zM9 17h6v4H9zM6 9v4h12V9M12 13v4',
  checkIn: 'M8 4H6a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2h-2M9 3h6v3H9zM8.5 13.5l2.5 2.5l4.5-5',
  draw: 'M4 4h16v16H4zM8.5 8.5h.01M15.5 8.5h.01M12 12h.01M8.5 15.5h.01M15.5 15.5h.01',
  schedule: 'M4 6h16v14H4zM4 10h16M8 3v4M16 3v4M8 14h3M13 14h3M8 17h3',
  controlDesk: 'M3 4h18v12H3zM8 20h8M12 16v4M7 8h4M7 11h7',
  stages: 'M3 5h5v4H3zM3 15h5v4H3zM8 7h3v10H8M11 12h4M15 10h6v4h-6z',
  results: 'M8 4h8v5a4 4 0 0 1-8 0zM8 6H4a3 3 0 0 0 4 4M16 6h4a3 3 0 0 1-4 4M12 13v4M8 21h8M10 17h4v4h-4z',
};

/** Side navigation following the tournament flow, with where each step stands. */
@Component({
  selector: 'app-tournament-layout',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, TranslocoDirective],
  templateUrl: './tournament-layout.html',
  styleUrl: './tournament-layout.scss',
})
export class TournamentLayout {
  private readonly tournament = inject(TournamentStore);
  private readonly language = inject(LanguageService);
  protected readonly clock = inject(Clock);
  protected readonly icons = ICONS;

  /** Tooltip of the folded menu: shown at once, beside the hovered step. */
  protected readonly tip = signal<{ label: string; sub: string; top: number; left: number } | null>(null);

  protected showTip(event: MouseEvent, label: string, sub: string): void {
    if (!this.panels.collapsed('nav')) {
      return;
    }
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect();
    this.tip.set({ label, sub, top: box.top + box.height / 2, left: box.right + 10 });
  }
  protected readonly panels = inject(Panels);
  protected readonly view = this.tournament.view;

  /** File name without its folder. */
  protected readonly fileName = computed(() => this.view().path.split(/[\\/]/).pop() ?? '');

  protected readonly groups = computed(() => {
    const v = this.view();
    const p = v.progress;
    const allConfirmed = p.drawConfirmedAt != null;
    const preparation: NavItem[] = [
      {
        key: 'info',
        path: 'info',
        mark: (v.categories ?? []).length > 0 ? 'done' : 'todo',
        sub: 'nav.sub.info',
        params: { courts: v.courts, rest: v.minRest },
      },
      {
        key: 'entrants',
        path: 'entrants',
        mark: p.entrants > 0 ? 'done' : 'todo',
        sub: p.doubles ? 'nav.sub.pairs' : 'nav.sub.entrants',
        params: { count: p.entrants },
      },
      {
        key: 'format',
        path: 'format',
        mark: allConfirmed ? 'done' : 'todo',
        sub: (v.categories ?? []).length > 1 ? 'nav.sub.formatPerCategory' : 'nav.sub.format',
      },
    ];
    const dayOf: NavItem[] = [
      {
        key: 'checkIn',
        path: 'check-in',
        mark: allConfirmed || (p.players > 0 && p.playersPresent === p.players) ? 'done' : 'todo',
        sub: 'nav.sub.checkIn',
        params: { present: p.playersPresent, total: p.players },
      },
      {
        key: 'draw',
        path: 'draw',
        mark: allConfirmed ? 'done' : 'todo',
        sub: allConfirmed ? 'nav.sub.drawConfirmed' : 'nav.sub.drawToConfirm',
        params: { time: p.drawConfirmedAt ? this.time(p.drawConfirmedAt) : '' },
      },
      {
        key: 'schedule',
        path: 'schedule',
        mark: p.final ? 'done' : 'todo',
        sub: 'nav.sub.schedule',
      },
      {
        key: 'controlDesk',
        path: 'control-desk',
        mark: p.final ? 'done' : p.started ? 'current' : 'todo',
        sub: p.final ? 'nav.sub.deskOver' : p.started ? 'nav.sub.deskLive' : 'nav.sub.deskAfterDraw',
      },
      {
        key: 'stages',
        path: 'stages',
        mark: p.final ? 'done' : 'todo',
        sub: 'nav.sub.stages',
        params: { played: p.played, total: p.matches },
      },
      {
        key: 'results',
        path: 'results',
        mark: v.archived ? 'done' : 'todo',
        sub: v.archived ? 'nav.sub.archived' : p.final ? 'nav.sub.final' : 'nav.sub.resultsLater',
      },
    ];
    return [
      { key: 'preparation', items: preparation },
      { key: 'dayOf', items: dayOf },
    ];
  });

  protected time(value: string | Date): string {
    return this.language.time(value);
  }
}
