import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective } from '@jsverse/transloco';
import { CheckInRow, CheckInView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { LanguageService } from '../core/language.service';
import { CategoryBadge } from '../shared/category-badge';
import { fold, matches, ownText } from './check-in-search';

/** Check-in: one row per player, one click to mark present. */
@Component({
  selector: 'app-check-in',
  imports: [TranslocoDirective, RouterLink, CategoryBadge],
  templateUrl: './check-in.html',
  styleUrl: './check-in.scss',
})
export class CheckIn {
  protected readonly tournament = inject(TournamentStore);
  protected readonly view = liveView<CheckInView>(() => TournamentService.GetCheckInView(), {
    rows: [],
    categories: [],
    players: 0,
    present: 0,
  });

  protected readonly search = signal('');
  protected readonly category = signal<Category | 'all'>('all');
  protected readonly presence = signal<'all' | 'present' | 'absent'>('all');
  private readonly language = inject(LanguageService);

  protected readonly pairs = computed(() => {
    const categories = (this.view().categories ?? []).filter((c) => c.doubles);
    return {
      doubles: categories.length > 0,
      complete: categories.reduce((n, c) => n + c.entrantsComplete, 0),
      total: categories.reduce((n, c) => n + (c.entrants ?? []).length, 0),
    };
  });

  protected readonly percent = computed(() => {
    const v = this.view();
    return v.players ? Math.round((100 * v.present) / v.players) : 0;
  });

  /** Time the draw was confirmed, once every category's is. */
  protected readonly drawConfirmedAt = computed(() => this.tournament.view().progress.drawConfirmedAt);

  protected readonly rows = computed(() => {
    const words = this.words();
    const category = this.category();
    return (this.view().rows ?? []).filter(
      (r) =>
        (category === 'all' || r.category === category) &&
        (this.presence() === 'all' || r.checkedIn === (this.presence() === 'present')) &&
        matches([...ownText(r), r.partner?.firstName, r.partner?.lastName], words),
    );
  });

  private readonly words = computed(() => fold(this.search().trim()).split(/\s+/).filter(Boolean));

  protected toggle(row: CheckInRow): void {
    void (row.checkedIn ? this.tournament.cancelCheckIn(row.id) : this.tournament.checkIn(row.id));
  }

  protected time(iso: string | null | undefined): string {
    return iso
      ? new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(new Date(iso))
      : '';
  }

  /** Enter in the search box checks in the only matching absent player. */
  protected async onSearchEnter(): Promise<void> {
    // Partners also match the search; Enter only considers the player's own
    // name, club or bib, so "zola p" picks Paul Zola, not his partner.
    const absent = this.rows().filter((r) => !r.checkedIn && matches(ownText(r), this.words()));
    if (absent.length === 1 && (await this.tournament.checkIn(absent[0].id))) {
      this.search.set('');
    }
  }
}
