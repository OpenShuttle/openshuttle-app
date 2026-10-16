import { Component } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TranslocoDirective } from '@jsverse/transloco';

interface NavItem {
  key: string;
  path?: string; // no path: not built yet
}

/** Side navigation following the tournament flow, and the current step. */
@Component({
  selector: 'app-tournament-layout',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, TranslocoDirective],
  templateUrl: './tournament-layout.html',
  styleUrl: './tournament-layout.scss',
})
export class TournamentLayout {
  protected readonly groups: { key: string; items: NavItem[] }[] = [
    {
      key: 'preparation',
      items: [{ key: 'info', path: 'info' }, { key: 'entrants', path: 'entrants' }, { key: 'format', path: 'format' }],
    },
    {
      key: 'dayOf',
      items: [
        { key: 'checkIn', path: 'check-in' },
        { key: 'draw', path: 'draw' },
        { key: 'controlDesk' },
        { key: 'stages' },
        { key: 'results' },
      ],
    },
  ];
}
