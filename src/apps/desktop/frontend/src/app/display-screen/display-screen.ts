import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { DisplayView, MatchView, SideView, TournamentService } from '@bindings/apps/desktop';

import { liveView } from '../core/live-view';
import { CategoryBadge } from '../shared/category-badge';
import { MatchStage } from '../shared/match-stage';
import { formatCountdown, secondsLeft } from '../shared/countdown';

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

  protected trackCall(m: MatchView): string {
    return m.id;
  }
}
