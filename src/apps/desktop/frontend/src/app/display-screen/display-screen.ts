import { Component, DestroyRef, ElementRef, HostListener, computed, inject, signal, viewChild } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { DisplayStage, DisplayView, MatchView, PoolPhase, TournamentService } from '@bindings/apps/desktop';

import { Window } from '@wailsio/runtime';

import { liveView } from '../core/live-view';
import { AppSettings } from '../core/settings.service';
import { LanguageService } from '../core/language.service';
import { BracketTree, TreeGeometry, cardLeft, cardTop, routeLinks, treeGeometry } from '../phases/bracket-tree';
import { BracketMatch } from '../shared/bracket-match';
import { cardWidth } from '../shared/card-width';
import { formatCountdown, secondsLeft } from '../shared/countdown';
import { clubs } from '../shared/names';
import { stageLabel } from '../shared/stage-label';
import { WithBibPipe } from '../shared/with-bib';
import { foldBracket } from '../phases/split-bracket';

/** How long each screenful of a stage taller than the screen stays, at least, in ms. */
const PAGE_TIME = 10_000;
/** Share of the screen a page moves by when no pool card gives a better break. */
const PAGE_STEP = 0.9;
/** Largest zoom of a small bracket. */
const MAX_SCALE = 1.2;

function stageKey(s: DisplayStage): string {
  return `${s.category}/${s.id}`;
}

/**
 * Display screen: second window on the hall monitor, read from 10 m. The
 * stages the desk shows (Stages screen) take turns: every pool, or the
 * bracket; a stage taller than the screen goes page by page.
 */
@Component({
  selector: 'app-display-screen',
  imports: [TranslocoDirective, WithBibPipe, BracketMatch],
  templateUrl: './display-screen.html',
  styleUrl: './display-screen.scss',
})
export class DisplayScreen {
  protected readonly view = liveView<DisplayView>(() => TournamentService.GetDisplayView(), {
    name: '',
    categories: [],
    round: null,
    stages: [],
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

  /** The bracket shown, folded in two, the places packed under the main draw. */
  protected readonly tree = computed(() => {
    const s = this.stage();
    return s?.kind === 'bracket' ? foldBracket(s.rounds ?? []).tree : null;
  });
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
   * it is taller than the screen and goes page by page (each page for
   * PAGE_TIME at least), then the next stage comes; a single stage starts over.
   */
  private tick(now: number): void {
    const stages = this.stages();
    if (stages.length === 0) {
      return;
    }
    const el = this.body()?.nativeElement;
    const tree = this.tree();
    if (el && tree && tree.matches.length > 0 && el.clientWidth > 0) {
      this.scale.set(Math.min(MAX_SCALE, el.clientWidth / this.treeWidth(tree)));
    }
    const pages = el ? pageOffsets(el) : [0];
    const duration = Math.max((this.settings.value()?.rotationSeconds ?? 30) * 1000, pages.length * PAGE_TIME);
    const elapsed = now - this.shownAt;
    if (elapsed >= duration) {
      const next = stages[(this.stageIndex() + 1) % stages.length];
      this.current.set(stageKey(next));
      this.shownAt = now;
      this.progress.set(0);
      this.page = 0;
      if (el) {
        el.scrollTop = 0;
      }
      return;
    }
    this.progress.set(elapsed / duration);
    const page = Math.min(pages.length - 1, Math.floor((elapsed / duration) * pages.length));
    if (el && page !== this.page) {
      this.page = page;
      if (el.scrollTo) {
        el.scrollTo({ top: pages[page], behavior: 'smooth' });
      } else {
        el.scrollTop = pages[page];
      }
    }
  }

  /** Page of the stage shown now. */
  private page = 0;

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

  /** Cards as wide as their longest line needs. */
  private readonly cardW = computed(() =>
    cardWidth(this.transloco, this.tree()?.matches.map((m) => m.match) ?? [], {
      bibs: this.settings.showBibs(),
      clubs: false,
      scores: false,
      origins: true,
      size: (this.stage()?.slots ?? []).length,
    }),
  );

  protected geometry(t: BracketTree): TreeGeometry {
    return treeGeometry(false, t.titled, true, this.cardW());
  }

  protected cardLeft(t: BracketTree, col: number): number {
    return cardLeft(this.geometry(t), col);
  }

  protected cardTop(t: BracketTree, row: number): number {
    return cardTop(this.geometry(t), row);
  }

  protected treeWidth(t: BracketTree): number {
    const g = this.geometry(t);
    return cardLeft(g, t.columns.length - 1) + g.cardW;
  }

  protected treeHeight(t: BracketTree): number {
    const g = this.geometry(t);
    return cardTop(g, t.rows - 1) + g.cardH + g.rowGap;
  }

  /** Every link, routed: from and to the teams' rows, on separate lanes. */
  protected readonly routes = computed(() => {
    const tree = this.tree();
    return tree ? routeLinks(this.geometry(tree), tree, tree.links, true) : [];
  });

  /** A link whose source match is decided: the one its entrants really took. */
  protected decided(t: BracketTree, from: string): boolean {
    const m = t.matches.find((x) => x.match.id === from)?.match;
    return !!m && (!!m.score || m.walkover);
  }
}

/**
 * Where each page of a scrolling stage starts: a page ends before the first
 * pool card or bracket part it would cut, else after PAGE_STEP of the screen; the last page
 * ends with the stage.
 */
export function pageOffsets(el: HTMLElement): number[] {
  const height = el.clientHeight;
  const overflow = el.scrollHeight - height;
  if (height <= 0 || overflow <= 0) {
    return [0];
  }
  const cards = [...el.querySelectorAll<HTMLElement>('.page-block')].map((c) => ({ top: c.offsetTop, bottom: c.offsetTop + c.offsetHeight }));
  const offsets = [0];
  let start = 0;
  while (start < overflow && offsets.length < 50) {
    const cut = cards.find((c) => c.top > start && c.bottom > start + height);
    const next = Math.min(overflow, cut ? cut.top : start + height * PAGE_STEP);
    if (next <= start) {
      break;
    }
    offsets.push(next);
    start = next;
  }
  return offsets;
}
