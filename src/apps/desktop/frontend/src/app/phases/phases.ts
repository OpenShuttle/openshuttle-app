import { Component, ElementRef, computed, inject, signal, viewChild } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import {
  MatchView,
  PhaseCategory,
  PhasesView,
  PoolPhase,
  StagePhase,
  TournamentService,
} from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { CategoryShown } from '../core/category-shown.service';
import { liveView } from '../core/live-view';
import { PanelToggle } from '../shared/panel-strip';
import { Panels } from '../core/panels.service';
import { TournamentStore } from '../core/tournament.service';
import { HistoryPanel } from '../history/history-panel';
import { ScorePanel } from '../score-panel/score-panel';
import { AppSettings } from '../core/settings.service';
import { BracketMatch } from '../shared/bracket-match';
import { cardWidth } from '../shared/card-width';
import { clubs, fullNames } from '../shared/names';
import { stageLabel, winSummary } from '../shared/stage-label';
import { WithBibPipe } from '../shared/with-bib';
import {
  BracketTree,
  cardLeft,
  cardTop,
  routeLinks,
  treeGeometry,
} from './bracket-tree';
import { BracketPart, foldBracket } from './split-bracket';


/** Phases: pool standings and matches, with a correction mode. */
@Component({
  selector: 'app-phases',
  imports: [TranslocoDirective, RouterLink, NgTemplateOutlet, ScorePanel, HistoryPanel, WithBibPipe, PanelToggle, BracketMatch],
  templateUrl: './phases.html',
  styleUrl: './phases.scss',
})
export class Phases {
  protected readonly tournament = inject(TournamentStore);
  protected readonly view = liveView<PhasesView>(() => TournamentService.GetPhasesView(), {
    categories: [],
  });

  private readonly shown = inject(CategoryShown);
  protected readonly current = computed(() => this.shown.pick(this.view().categories));

  protected readonly correction = signal(false);
  protected readonly panels = inject(Panels);
  /** The history panel, shown unless folded (remembered on this PC). */
  protected readonly showHistory = computed(() => !this.panels.collapsed('phases.history'));

  protected toggleHistory(): void {
    this.panels.toggle('phases.history');
  }
  private readonly transloco = inject(TranslocoService);
  /** Stage chosen by the desk; empty until then. */
  private readonly stageKey = signal('');
  /** Stage shown: the chosen one, else the first not over (the one holding the next match), else the last. */
  protected readonly stage = computed(() => {
    const stages = this.current()?.stages ?? [];
    return (
      stages.find((s) => s.id === this.stageKey()) ??
      stages.find((s) => !s.complete) ??
      stages[stages.length - 1] ??
      null
    );
  });
  protected readonly scoring = signal<MatchView | null>(null);
  /** First match picked for a swap. */
  protected readonly swapping = signal<MatchView | null>(null);
  /** First bracket slot picked for a swap: "<bracket>:<slot>". */
  protected readonly swappingSlot = signal<string | null>(null);

  protected select(category: Category): void {
    this.shown.show(category);
    this.stageKey.set('');
  }

  protected label(stage: { name: string; kind: string }): string {
    return stageLabel(this.transloco, stage);
  }

  protected win(stage: StagePhase): string {
    return winSummary(this.transloco, stage.win);
  }

  protected selectStage(key: string): void {
    this.stageKey.set(key);
  }

  protected played(pool: PoolPhase): number {
    return (pool.matches ?? []).filter((m) => m.score || m.walkover).length;
  }

  protected readonly short = fullNames;
  protected readonly clubs = clubs;

  /** Shows or hides the stage on the display screen. */
  protected setDisplayed(c: PhaseCategory, b: StagePhase, event: Event): void {
    void this.tournament.setStageDisplay(c.category, b.id, (event.target as HTMLInputElement).checked);
  }

  protected toggleCorrection(): void {
    this.correction.update((on) => !on);
    this.swapping.set(null);
    this.swappingSlot.set(null);
  }

  /**
   * The bracket shown, folded in two around its final, the places packed
   * under the main draw; none for pools or a bracket not drawn.
   */
  protected readonly folded = computed(() => {
    const s = this.stage();
    return s?.kind === 'bracket' && s.confirmed ? foldBracket(s.rounds ?? []) : null;
  });
  private readonly settings = inject(AppSettings);
  /** Cards as wide as their longest line needs. */
  protected readonly geometry = computed(() => {
    const matches = this.folded()?.tree.matches.map((m) => m.match) ?? [];
    const size = (this.stage()?.slots ?? []).length;
    const width = cardWidth(this.transloco, matches, { bibs: this.settings.showBibs(), clubs: true, scores: true, origins: true, size });
    return treeGeometry(this.correction(), false, false, width);
  });
  private readonly scroller = viewChild<ElementRef<HTMLElement>>('scroller');

  protected left(col: number): number {
    return cardLeft(this.geometry(), col);
  }

  protected top(row: number): number {
    return cardTop(this.geometry(), row);
  }

  protected treeWidth(t: BracketTree): number {
    const g = this.geometry();
    return cardLeft(g, t.columns.length - 1) + g.cardW;
  }

  protected treeHeight(t: BracketTree): number {
    const g = this.geometry();
    return cardTop(g, t.rows - 1) + g.cardH + g.rowGap;
  }

  /** Every link, routed: from and to the teams' rows, on separate lanes. */
  protected readonly routes = computed(() => {
    const fb = this.folded();
    return fb ? routeLinks(this.geometry(), fb.tree, fb.tree.links) : [];
  });

  /** A link whose source match is decided: the one its entrants really took. */
  protected decided(t: BracketTree, from: string): boolean {
    const m = t.matches.find((x) => x.match.id === from)?.match;
    return !!m && (!!m.score || m.walkover);
  }

  /** Brings a part of the bracket (places 5–8…) to the top of the view. */
  protected goPart(p: BracketPart): void {
    const card = this.scroller()?.nativeElement.querySelector(`[data-match="${p.first}"]`);
    card?.scrollIntoView?.({ behavior: 'smooth', block: 'center', inline: 'nearest' });
  }

  protected status(s: StagePhase): string {
    if (!s.confirmed) {
      return 'notDrawn';
    }
    return s.complete ? 'finished' : s.started ? 'inProgress' : 'ready';
  }

  protected place(from: number, to: number): string {
    return from === to ? String(from) : `${from}–${to}`;
  }

  protected async swapSlot(c: PhaseCategory, b: StagePhase, slot: number): Promise<void> {
    const key = `${b.id}:${slot}`;
    const first = this.swappingSlot();
    if (!first || !first.startsWith(b.id + ':')) {
      this.swappingSlot.set(key);
      return;
    }
    const a = Number(first.split(':')[1]);
    if (a === slot || (await this.tournament.swapSlots(c.category, b.id, a, slot))) {
      this.swappingSlot.set(null);
    }
  }

  protected diff(won: number, lost: number): string {
    const d = won - lost;
    return d > 0 ? `+${d}` : String(d);
  }

  protected scoreText(m: MatchView): string {
    return (m.score?.sets ?? []).map((s) => `${s.a}-${s.b}`).join(' · ');
  }

  /** Waiting: not called and not decided, so it can be swapped. */
  protected waiting(m: MatchView): boolean {
    return !m.court && !m.score && !m.walkover;
  }

  protected async swap(m: MatchView): Promise<void> {
    const first = this.swapping();
    if (!first) {
      this.swapping.set(m);
      return;
    }
    if (first.id === m.id || (await this.tournament.swapMatches(first.id, m.id))) {
      this.swapping.set(null);
    }
  }
}
