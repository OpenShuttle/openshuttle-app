import { Component, ElementRef, computed, inject, signal, viewChild } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import {
  MatchView,
  PhaseCategory,
  PhasesView,
  PoolPhase,
  SideView,
  StagePhase,
  TournamentService,
} from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { CategoryShown } from '../core/category-shown.service';
import { liveView } from '../core/live-view';
import { Panels } from '../core/panels.service';
import { TournamentStore } from '../core/tournament.service';
import { HistoryPanel } from '../history/history-panel';
import { ScorePanel } from '../score-panel/score-panel';
import { shortRoundLabel } from '../shared/match-stage';
import { fullNames } from '../shared/names';
import { stageLabel, winSummary } from '../shared/stage-label';
import { WithBibPipe } from '../shared/with-bib';
import {
  BracketTree,
  TreeBlock,
  TreeLink,
  bracketTree,
  cardLeft,
  cardTop,
  linkPath,
  treeGeometry,
} from './bracket-tree';


/** Phases: pool standings and matches, with a correction mode. */
@Component({
  selector: 'app-phases',
  imports: [TranslocoDirective, RouterLink, NgTemplateOutlet, ScorePanel, HistoryPanel, WithBibPipe],
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
  private readonly panels = inject(Panels);
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

  protected toggleCorrection(): void {
    this.correction.update((on) => !on);
    this.swapping.set(null);
    this.swappingSlot.set(null);
  }

  protected shortRound(m: MatchView): { key: string; params: Record<string, number> } {
    return shortRoundLabel(m);
  }

  /** The bracket shown, laid out as a tree (none for pools or a bracket not drawn). */
  protected readonly tree = computed(() => {
    const s = this.stage();
    return s?.kind === 'bracket' && s.confirmed ? bracketTree(s.rounds ?? []) : null;
  });
  protected readonly geometry = computed(() => treeGeometry(this.correction()));
  private readonly scroller = viewChild<ElementRef<HTMLElement>>('scroller');
  /** Column at the left of the drawing, for the previous and next buttons. */
  protected readonly activeCol = signal(0);

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

  protected path(t: BracketTree, l: TreeLink): string {
    const from = t.matches.find((m) => m.match.id === l.from);
    const to = t.matches.find((m) => m.match.id === l.to);
    return from && to ? linkPath(this.geometry(), from, to) : '';
  }

  /** A link whose source match is decided: the one its entrants really took. */
  protected decided(t: BracketTree, from: string): boolean {
    const m = t.matches.find((x) => x.match.id === from)?.match;
    return !!m && (!!m.score || m.walkover);
  }

  protected onScroll(): void {
    const el = this.scroller()?.nativeElement;
    if (el) {
      const g = this.geometry();
      this.activeCol.set(Math.round(el.scrollLeft / (g.cardW + g.colGap)));
    }
  }

  /** Brings a column of the drawing to the left of the view. */
  protected goColumn(t: BracketTree, col: number): void {
    const c = t.columns[Math.max(0, Math.min(col, t.columns.length - 1))];
    this.activeCol.set(t.columns.indexOf(c));
    this.reveal(c.first, 'nearest');
  }

  /** Brings a part of the bracket (places 5–8…) to the middle of the view. */
  protected goBlock(b: TreeBlock): void {
    this.reveal(b.first, 'center');
  }

  private reveal(match: string, block: ScrollLogicalPosition): void {
    const card = this.scroller()?.nativeElement.querySelector(`[data-match="${match}"]`);
    card?.scrollIntoView?.({ behavior: 'smooth', block, inline: 'start' });
  }

  protected status(s: StagePhase): string {
    if (!s.confirmed) {
      return 'notDrawn';
    }
    return s.complete ? 'finished' : s.started ? 'inProgress' : 'ready';
  }

  /** Side label inside a bracket: names, "bye", or "to be decided". */
  protected sideLabel(side: SideView, bye: boolean): { key?: string; text?: string } {
    if (bye) {
      return { key: 'phases.bye' };
    }
    if (!side.entrant) {
      return { key: 'phases.tbd' };
    }
    return { text: `#${side.bib} ${fullNames(side)}` };
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

  /** Side that won a scored match, by sets. */
  protected winner(m: MatchView): 'a' | 'b' | '' {
    let a = 0;
    let b = 0;
    for (const set of m.score?.sets ?? []) {
      if (set.a > set.b) a++;
      else if (set.b > set.a) b++;
    }
    return a > b ? 'a' : b > a ? 'b' : '';
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
