import { Component, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { LanguageService } from '../core/language.service';
import { TournamentStore } from '../core/tournament.service';

/** Home screen, outside any tournament: recent files, create, open. */
@Component({
  selector: 'app-home',
  imports: [TranslocoDirective],
  templateUrl: './home.html',
  styleUrl: './home.scss',
})
export class Home {
  protected readonly tournament = inject(TournamentStore);
  private readonly language = inject(LanguageService);

  protected readonly creating = signal(false);
  protected readonly name = signal('');

  protected startCreate(): void {
    this.name.set('');
    this.creating.set(true);
  }

  protected async create(): Promise<void> {
    if (await this.tournament.create(this.name())) {
      this.creating.set(false);
    }
  }

  protected date(iso: string): string {
    return new Intl.DateTimeFormat(this.language.active, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(iso));
  }
}
