import { Component, computed, inject } from '@angular/core';
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
    return new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(
      new Date(value),
    );
  }
}
