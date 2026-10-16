import { Component, DestroyRef, HostListener, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective } from '@jsverse/transloco';
import { ControlDeskView, CourtView, MatchView, SideView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { LanguageService } from '../core/language.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { ScorePanel } from '../score-panel/score-panel';
import { CategoryBadge } from '../shared/category-badge';

/** Pending scores older than this are highlighted. */
const LATE_SCORE_MINUTES = 10;

const EMPTY: ControlDeskView = {
  now: new Date().toISOString(),
  courts: [],
  pending: [],
  queue: [],
  complete: [],
  priority: 'balanced',
  minRest: 10,
  categories: [],
  started: false,
};

/** Control desk: courts, pending scores, queue, alerts. */
@Component({
  selector: 'app-control-desk',
  imports: [TranslocoDirective, RouterLink, CategoryBadge, ScorePanel],
  templateUrl: './control-desk.html',
  styleUrl: './control-desk.scss',
})
export class ControlDesk {
  protected readonly tournament = inject(TournamentStore);
  private readonly language = inject(LanguageService);

  // Rest periods end without an event: refetch every 15 s.
  protected readonly view = liveView<ControlDeskView>(() => TournamentService.GetControlDeskView(), EMPTY, 15_000);

  /** Ticks every second for the "played for" timers. */
  protected readonly now = signal(Date.now());
  protected readonly scoring = signal<MatchView | null>(null);
  protected readonly queueFilter = signal<Category | 'all'>('all');

  protected readonly counts = computed(() => {
    const courts = this.view().courts ?? [];
    return {
      playing: courts.filter((c) => c.state === 'playing').length,
      free: courts.filter((c) => c.state === 'free').length,
      pending: (this.view().pending ?? []).length,
    };
  });

  protected readonly queue = computed(() => {
    const filter = this.queueFilter();
    return (this.view().queue ?? []).filter((q) => filter === 'all' || q.match.category === filter);
  });

  constructor() {
    const timer = setInterval(() => this.now.set(Date.now()), 1000);
    inject(DestroyRef).onDestroy(() => clearInterval(timer));
  }

  protected names(side: SideView): string {
    return (side.names ?? []).join(' / ');
  }

  /** "mm:ss" since an ISO time. */
  protected elapsed(iso: string | null | undefined): string {
    if (!iso) {
      return '';
    }
    const seconds = Math.max(0, Math.floor((this.now() - Date.parse(iso)) / 1000));
    const m = Math.floor(seconds / 60);
    return `${m}:${String(seconds % 60).padStart(2, '0')}`;
  }

  protected minutesSince(iso: string | null | undefined): number {
    return iso ? Math.max(0, Math.floor((this.now() - Date.parse(iso)) / 60_000)) : 0;
  }

  protected late(m: MatchView): boolean {
    return this.minutesSince(m.endedAt) >= LATE_SCORE_MINUTES;
  }

  protected time(iso: string | null | undefined): string {
    if (!iso) {
      return '';
    }
    return new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(
      new Date(iso),
    );
  }

  protected call(court: CourtView): void {
    if (court.proposal) {
      void this.tournament.callMatch(court.proposal.id, court.number);
    }
  }

  protected end(court: CourtView): void {
    if (court.match) {
      void this.tournament.endMatch(court.match.id);
    }
  }

  /**
   * Keyboard: a court number calls, ends, or opens the score of that court;
   * S opens the oldest pending score. Digit codes work on AZERTY too.
   */
  @HostListener('document:keydown', ['$event'])
  protected onKeydown(event: KeyboardEvent): void {
    const target = event.target as HTMLElement | null;
    if (this.scoring() || event.ctrlKey || event.metaKey || event.altKey ||
        (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))) {
      return;
    }
    const digit = /^(Digit|Numpad)([1-9])$/.exec(event.code);
    if (digit) {
      const court = (this.view().courts ?? []).find((c) => c.number === Number(digit[2]));
      if (court) {
        event.preventDefault();
        this.act(court);
      }
      return;
    }
    if (event.key.toLowerCase() === 's') {
      const oldest = (this.view().pending ?? [])[0];
      if (oldest) {
        event.preventDefault();
        this.scoring.set(oldest);
      }
    }
  }

  private act(court: CourtView): void {
    if (court.state === 'playing') {
      this.end(court);
    } else if (court.state === 'finished' && court.match) {
      this.scoring.set(court.match);
    } else {
      this.call(court);
    }
  }
}
