import { Component, DestroyRef, HostListener, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ControlDeskView, CourtView, MatchView, SideView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { LanguageService } from '../core/language.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { ScorePanel } from '../score-panel/score-panel';
import { CategoryBadge } from '../shared/category-badge';
import { MatchStage } from '../shared/match-stage';
import { formatCountdown, secondsLeft } from '../shared/countdown';
import { clubs, fullNames } from '../shared/names';
import { stageLabel } from '../shared/stage-label';

/** Pending scores older than this are highlighted. */
const LATE_SCORE_MINUTES = 10;

const EMPTY: ControlDeskView = {
  now: new Date().toISOString(),
  courts: [],
  pending: [],
  queue: [],
  complete: [],
  stagesToFill: [],
  minRest: 10,
  categories: [],
  started: false,
  rounds: { active: false, current: null, next: [], played: 0 },
};

/** Control desk: courts, pending scores, queue, alerts. */
@Component({
  selector: 'app-control-desk',
  imports: [TranslocoDirective, RouterLink, CategoryBadge, ScorePanel, MatchStage],
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
  /** Queue match whose court picker is open. */
  protected readonly launching = signal<string | null>(null);

  /** Courts a match can be called on now: nothing in progress there. */
  protected readonly openCourts = computed(() =>
    (this.view().courts ?? []).filter((c) => c.state !== 'playing').map((c) => c.number),
  );

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

  /** Seconds left in the round in progress; negative once elapsed. */
  protected readonly roundLeft = computed(() => {
    const current = this.view().rounds.current;
    return current ? secondsLeft(current.endsAt, this.now()) : 0;
  });
  protected readonly roundClock = computed(() => formatCountdown(this.roundLeft()));

  /** Matches of the round in progress: no timer nor End button per court. */
  protected inRound(m: MatchView): boolean {
    return (this.view().rounds.current?.matches ?? []).some((r) => r.id === m.id);
  }

  protected startRound(): void {
    void this.tournament.startRound();
  }

  protected endRound(): void {
    void this.tournament.endRound();
  }

  constructor() {
    const timer = setInterval(() => this.now.set(Date.now()), 1000);
    inject(DestroyRef).onDestroy(() => clearInterval(timer));
  }

  private readonly transloco = inject(TranslocoService);

  protected stageName(stage: { name: string; kind: string }): string {
    return stageLabel(this.transloco, stage);
  }

  protected short(side: SideView): string {
    return fullNames(side);
  }

  protected clubs(side: SideView): string {
    return clubs(side);
  }

  /** Court a waiting match is proposed on, 0 if none. */
  protected proposedCourt(id: string): number {
    return (this.view().courts ?? []).find((c) => c.proposal?.id === id)?.number ?? 0;
  }

  protected readonly alertCount = computed(
    () => (this.view().complete ?? []).length + (this.view().stagesToFill ?? []).length,
  );

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

  /** Calls a queued match on a court, rest or not: rest is only a warning. */
  protected async launch(m: MatchView, court: number): Promise<void> {
    if (await this.tournament.callMatch(m.id, court)) {
      this.launching.set(null);
    }
  }

  /** Courts whose proposal has not had its rest: the desk forces or reorders. */
  protected readonly unrestedCourts = computed(() =>
    (this.view().courts ?? []).filter((c) => c.proposal && c.proposalUnrested && !this.view().rounds.active).map((c) => c.number),
  );

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
    const key = event.key.toLowerCase();
    const rounds = this.view().rounds;
    if (key === 'l' && rounds.active && !rounds.current && (rounds.next ?? []).length > 0) {
      event.preventDefault();
      this.startRound();
      return;
    }
    if (key === 'f' && rounds.current) {
      event.preventDefault();
      this.endRound();
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
    if (court.state === 'playing' && court.match && this.inRound(court.match)) {
      this.scoring.set(court.match); // a round ends with the whistle, not per court
    } else if (court.state === 'playing') {
      this.end(court);
    } else if (court.state === 'finished' && court.match) {
      this.scoring.set(court.match);
    } else {
      this.call(court);
    }
  }
}
