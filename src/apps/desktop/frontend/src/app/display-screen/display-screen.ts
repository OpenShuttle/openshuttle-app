import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { DisplayBoard, DisplayCourt, DisplayView, MatchView, SideView, TournamentService } from '@bindings/apps/desktop';

import { liveView } from '../core/live-view';
import { MatchStage } from '../shared/match-stage';
import { formatCountdown, secondsLeft } from '../shared/countdown';
import { AppSettings } from '../core/settings.service';
import { LanguageService } from '../core/language.service';
import { shortNames } from '../shared/names';
import { stageLabel } from '../shared/stage-label';

/** Display screen: second window on the hall monitor. Read-only, read from 10 m. */
@Component({
  selector: 'app-display-screen',
  imports: [TranslocoDirective, NgTemplateOutlet, MatchStage],
  templateUrl: './display-screen.html',
  styleUrl: './display-screen.scss',
})
export class DisplayScreen {
  protected readonly view = liveView<DisplayView>(() => TournamentService.GetDisplayView(), {
    name: '',
    categories: [],
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

  private readonly language = inject(LanguageService);
  protected readonly now = signal(Date.now());
  /** Index of the board shown, for the dots. */
  protected readonly boardIndex = computed(() => {
    const boards = this.view().boards ?? [];
    const period = (this.settings.value()?.rotationSeconds ?? 8) * 1000;
    return boards.length ? Math.floor(this.now() / period) % boards.length : 0;
  });
  protected readonly hero = computed(() => (this.view().calls ?? [])[0] ?? null);
  protected readonly previous = computed(() => (this.view().calls ?? []).slice(1));
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

  private readonly transloco = inject(TranslocoService);

  protected stageName(b: DisplayBoard): string {
    return stageLabel(this.transloco, { name: b.stageName, kind: b.stageKind });
  }

  protected short(side: SideView): string {
    return shortNames(side);
  }

  protected time(value: string | number | null | undefined): string {
    return value
      ? new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(new Date(value))
      : '';
  }

  /** Whole minutes since a match was called. */
  protected minutes(m: MatchView): number {
    return m.calledAt ? Math.max(0, Math.floor((this.now() - Date.parse(m.calledAt)) / 60_000)) : 0;
  }

  protected tileState(c: DisplayCourt): string {
    return c.state === 'playing' ? 'playing' : c.state === 'finished' ? 'finished' : 'free';
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
