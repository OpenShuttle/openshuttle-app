import { Component, effect, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { TournamentStore } from '../core/tournament.service';

/** Tournament info: for now the name and the file. */
@Component({
  selector: 'app-tournament-info',
  imports: [TranslocoDirective],
  templateUrl: './tournament-info.html',
  styleUrl: './tournament-info.scss',
})
export class TournamentInfo {
  protected readonly tournament = inject(TournamentStore);
  protected readonly name = signal('');

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
