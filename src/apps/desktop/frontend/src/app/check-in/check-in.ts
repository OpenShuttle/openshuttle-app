import { Component, ElementRef, afterNextRender, computed, inject, signal, viewChild } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective } from '@jsverse/transloco';
import { CheckInTeam, CheckInView, PlayerView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { LanguageService } from '../core/language.service';
import { CategoryBadge } from '../shared/category-badge';
import { fold, matches, ownText, teamText } from './check-in-search';

/** Check-in: one row per team, the pair's players together; a whole team or one player at a time. */
@Component({
  selector: 'app-check-in',
  imports: [TranslocoDirective, RouterLink, CategoryBadge],
  templateUrl: './check-in.html',
  styleUrl: './check-in.scss',
})
export class CheckIn {
  protected readonly tournament = inject(TournamentStore);
  protected readonly view = liveView<CheckInView>(() => TournamentService.GetCheckInView(), {
    teams: [],
    categories: [],
    players: 0,
    present: 0,
  });

  protected readonly search = signal('');
  protected readonly category = signal<Category | 'all'>('all');
  protected readonly presence = signal<'all' | 'present' | 'absent'>('all');
  private readonly language = inject(LanguageService);
  private readonly searchField = viewChild<ElementRef<HTMLInputElement>>('searchField');

  constructor() {
    // Check-in is typing names: the search box is ready as the screen opens.
    afterNextRender(() => this.searchField()?.nativeElement.focus());
  }

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

  protected readonly teams = computed(() => {
    const words = this.words();
    const category = this.category();
    const presence = this.presence();
    return (this.view().teams ?? []).filter(
      (t) =>
        (category === 'all' || t.category === category) &&
        (presence === 'all' || t.complete === (presence === 'present')) &&
        matches(teamText(t), words),
    );
  });

  private readonly words = computed(() => fold(this.search().trim()).split(/\s+/).filter(Boolean));

  protected toggle(p: PlayerView): void {
    void (p.checkedIn ? this.tournament.cancelCheckIn(p.id) : this.tournament.checkIn(p.id));
  }

  /** Both players of a pair arriving together: one click, one undo. */
  protected checkInTeam(team: CheckInTeam): void {
    void this.tournament.checkInEntrant(team.entrantId);
  }

  protected absent(team: CheckInTeam): number {
    return (team.players ?? []).filter((p) => !p.checkedIn).length;
  }

  protected time(iso: string | null | undefined): string {
    return this.language.time(iso);
  }

  /**
   * Enter in the search box checks in the only matching absent player: only
   * players' own names, clubs or bibs count, so "zola p" picks Paul Zola, not
   * his partner.
   */
  protected async onSearchEnter(): Promise<void> {
    const absent = this.teams().flatMap((t) =>
      (t.players ?? []).filter((p) => !p.checkedIn && matches(ownText(p, t), this.words())),
    );
    if (absent.length === 1 && (await this.tournament.checkIn(absent[0].id))) {
      this.search.set('');
    }
  }
}
