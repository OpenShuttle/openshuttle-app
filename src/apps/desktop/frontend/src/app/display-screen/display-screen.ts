import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { DisplayBoard, DisplayView, MatchView, SideView, TournamentService } from '@bindings/apps/desktop';

import { liveView } from '../core/live-view';
import { CategoryBadge } from '../shared/category-badge';
import { MatchStage } from '../shared/match-stage';
import { formatCountdown, secondsLeft } from '../shared/countdown';
import { AppSettings } from '../core/settings.service';

/** Display screen: second window on the hall monitor. Read-only, read from 10 m. */
@Component({
  selector: 'app-display-screen',
  imports: [TranslocoDirective, CategoryBadge, MatchStage],
  templateUrl: './display-screen.html',
  styleUrl: './display-screen.scss',
})
export class DisplayScreen {
  protected readonly view = liveView<DisplayView>(() => TournamentService.GetDisplayView(), {
    calls: [],
    courts: [],
    round: null,
    boards: [],
  });

  private readonly settings = inject(AppSettings);
  /** The board shown now: boards rotate every rotationSeconds. */
  protected readonly board = computed<DisplayBoard | null>(() => {
    const boards = this.view().boards ?? [];
    if (boards.length === 0) {
      return null;
    }
    const period = (this.settings.value()?.rotationSeconds ?? 8) * 1000;
    return boards[Math.floor(this.now() / period) % boards.length];
  });

  private readonly now = signal(Date.now());
  protected readonly left = computed(() => {
    const round = this.view().round;
    return round ? secondsLeft(round.endsAt, this.now()) : 0;
  });
  protected readonly clock = computed(() => formatCountdown(this.left()));

  constructor() {
    const timer = setInterval(() => this.now.set(Date.now()), 500);
    inject(DestroyRef).onDestroy(() => clearInterval(timer));
  }

  protected names(side: SideView): string {
    return (side.names ?? []).join(' / ');
  }

  protected diff(won: number, lost: number): string {
    const d = won - lost;
    return d > 0 ? `+${d}` : String(d);
  }

  protected score(m: MatchView): string {
    return (m.score?.sets ?? []).map((s) => `${s.a}-${s.b}`).join(' ');
  }

  protected trackCall(m: MatchView): string {
    return m.id;
  }
}
