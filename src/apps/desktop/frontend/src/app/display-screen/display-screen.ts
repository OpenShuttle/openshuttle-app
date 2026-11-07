import { Component, DestroyRef, ElementRef, HostListener, computed, inject, signal, viewChild } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { DisplayStage, DisplayView, MatchView, PoolPhase, TournamentService } from '@bindings/apps/desktop';

import { Window } from '@wailsio/runtime';

import { liveView } from '../core/live-view';
import { AppSettings } from '../core/settings.service';
import { LanguageService } from '../core/language.service';
import { BracketTree, TreeLink, bracketTree, cardLeft, cardTop, linkPath, treeGeometry } from '../phases/bracket-tree';
import { BracketMatch } from '../shared/bracket-match';
import { matchWinner } from '../shared/bracket-labels';
import { formatCountdown, secondsLeft } from '../shared/countdown';
import { MatchStage } from '../shared/match-stage';
import { clubs } from '../shared/names';
import { ResultIcon } from '../shared/result-icon';
import { stageLabel } from '../shared/stage-label';
import { WithBibPipe } from '../shared/with-bib';

/** Scrolling speed of a stage taller than the screen, in px per second. */
const SCROLL_SPEED = 40;
/** Pause at the top and at the bottom of a stage before scrolling or moving on, in ms. */
const HOLD = 4000;
/** Largest zoom of a small bracket. */
const MAX_SCALE = 1.6;

function stageKey(s: DisplayStage): string {
  return `${s.category}/${s.id}`;
}

/**
 * Display screen: second window on the hall monitor, read from 10 m. The
 * stages being played take turns (the desk picks which ones on the Stages
 * screen): every pool, or the bracket, scrolling slowly when taller than the
 * screen; the latest results stay on the right.
 */
@Component({
  selector: 'app-display-screen',
  imports: [TranslocoDirective, MatchStage, WithBibPipe, BracketMatch, ResultIcon],
  templateUrl: './display-screen.html',
  styleUrl: './display-screen.scss',
})
export class DisplayScreen {
  protected readonly view = liveView<DisplayView>(() => TournamentService.GetDisplayView(), {
    name: '',
    categories: [],
    round: null,
    stages: [],
    results: [],
  });

  private readonly settings = inject(AppSettings);
  private readonly language = inject(LanguageService);
  private readonly transloco = inject(TranslocoService);
  protected readonly now = signal(Date.now());

  /** The stage shown: the one the rotation is on, else the first. */
  private readonly current = signal('');
  protected readonly stages = computed(() => this.view().stages ?? []);
  protected readonly stage = computed<DisplayStage | null>(() => {
    const stages = this.stages();
    return stages.find((s) => stageKey(s) === this.current()) ?? stages[0] ?? null;
  });
  protected readonly stageIndex = computed(() => {
    const s = this.stage();
    return s ? this.stages().indexOf(s) : 0;
  });
  /** Share of the stage's time already spent, for the progress bar. */
  protected readonly progress = signal(0);

  protected readonly tree = computed(() => {
    const s = this.stage();
    return s?.kind === 'bracket' ? bracketTree(s.rounds ?? []) : null;
  });
  protected readonly geometry = computed(() => treeGeometry(false, this.tree()?.titled));
  /** Zoom of the bracket so that it fills the width of the screen. */
  protected readonly scale = signal(1);

  protected readonly left = computed(() => {
    const round = this.view().round;
    return round ? secondsLeft(round.endsAt, this.now()) : 0;
  });
  protected readonly countdown = computed(() => formatCountdown(this.left()));

  private readonly body = viewChild<ElementRef<HTMLElement>>('body');
  private shownAt = Date.now();

  /** Full screen or in a window; the button shows only while the mouse moves. */
  protected readonly fullscreen = signal(false);
  protected readonly controlsVisible = signal(false);
  private hideControls: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    const clock = setInterval(() => this.now.set(Date.now()), 500);
    const scroll = setInterval(() => this.tick(Date.now()), 50);
    const refresh = () => void Window.IsFullscreen().then((f) => this.fullscreen.set(f), () => undefined);
    refresh();
    window.addEventListener('resize', refresh);
    inject(DestroyRef).onDestroy(() => {
      clearInterval(clock);
      clearInterval(scroll);
      clearTimeout(this.hideControls);
      window.removeEventListener('resize', refresh);
    });
  }

  /**
   * Moves the rotation on: a stage stays its rotation time, or longer when
   * it needs to scroll to its end (a pause at the top, a slow scroll, a pause
   * at the bottom), then the next stage comes; a single stage starts over.
   */
  private tick(now: number): void {
    const stages = this.stages();
    if (stages.length === 0) {
      return;
    }
    const el = this.body()?.nativeElement;
    const tree = this.tree();
    if (el && tree && el.clientWidth > 0) {
      this.scale.set(Math.min(MAX_SCALE, el.clientWidth / this.treeWidth(tree)));
    }
    const overflow = el ? Math.max(0, el.scrollHeight - el.clientHeight) : 0;
    const scrolling = overflow > 0 ? HOLD * 2 + (overflow / SCROLL_SPEED) * 1000 : 0;
    const duration = Math.max((this.settings.value()?.rotationSeconds ?? 30) * 1000, scrolling);
    const elapsed = now - this.shownAt;
    if (elapsed >= duration) {
      const next = stages[(this.stageIndex() + 1) % stages.length];
      this.current.set(stageKey(next));
      this.shownAt = now;
      this.progress.set(0);
      if (el) {
        el.scrollTop = 0;
      }
      return;
    }
    this.progress.set(elapsed / duration);
    if (el) {
      el.scrollTop = Math.min(overflow, Math.max(0, ((elapsed - HOLD) / 1000) * SCROLL_SPEED));
    }
  }

  /** The hall never sees the button: it appears when someone moves the mouse here. */
  @HostListener('document:mousemove')
  protected showControls(): void {
    this.controlsVisible.set(true);
    clearTimeout(this.hideControls);
    this.hideControls = setTimeout(() => this.controlsVisible.set(false), 3000);
  }

  @HostListener('document:dblclick')
  protected toggleFullscreen(): void {
    void Window.ToggleFullscreen();
  }

  protected stageName(s: DisplayStage): string {
    return stageLabel(this.transloco, { name: s.name, kind: s.kind });
  }

  protected readonly clubs = clubs;
  protected readonly sideKeys: ('a' | 'b')[] = ['a', 'b'];

  protected time(value: string | number | null | undefined): string {
    return this.language.time(value);
  }

  protected diff(won: number, lost: number): string {
    const d = won - lost;
    return d > 0 ? `+${d}` : String(d);
  }

  protected played(pool: PoolPhase): number {
    return (pool.matches ?? []).filter((m) => m.score || m.walkover).length;
  }

  /** Matches of a pool on court now. */
  protected onCourt(pool: PoolPhase): MatchView[] {
    return (pool.matches ?? []).filter((m) => m.court && !m.endedAt);
  }

  protected winner(m: MatchView): 'a' | 'b' | '' {
    return matchWinner(m);
  }

  protected points(m: MatchView, side: 'a' | 'b'): number[] {
    return (m.score?.sets ?? []).map((s) => s[side]);
  }

  protected cardLeft(col: number): number {
    return cardLeft(this.geometry(), col);
  }

  protected cardTop(row: number): number {
    return cardTop(this.geometry(), row);
  }

  protected treeWidth(t: BracketTree): number {
    const g = treeGeometry(false, t.titled);
    return cardLeft(g, t.columns.length - 1) + g.cardW;
  }

  protected treeHeight(t: BracketTree): number {
    const g = treeGeometry(false, t.titled);
    return cardTop(g, t.rows - 1) + g.cardH + g.rowGap;
  }

  protected path(t: BracketTree, l: TreeLink): string {
    const from = t.matches.find((m) => m.match.id === l.from);
    const to = t.matches.find((m) => m.match.id === l.to);
    return from && to ? linkPath(this.geometry(), l.kind, from, to) : '';
  }

  /** A link whose source match is decided: the one its entrants really took. */
  protected decided(t: BracketTree, from: string): boolean {
    const m = t.matches.find((x) => x.match.id === from)?.match;
    return !!m && (!!m.score || m.walkover);
  }
}
