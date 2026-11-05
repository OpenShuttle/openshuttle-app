import { Component, computed, inject, signal } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import {
  MatchView,
  PhaseCategory,
  PhasesView,
  PoolPhase,
  RoundView,
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
import { roundLabel, shortRoundLabel } from '../shared/match-stage';
import { fullNames } from '../shared/names';
import { stageLabel, winSummary } from '../shared/stage-label';
import { WithBibPipe } from '../shared/with-bib';

/** A sub-bracket of the phases screen: the places it plays for and its rounds. */
interface BracketSection {
  start: number;
  end: number;
  rounds: RoundView[];
}

/** Phases: pool standings and matches, with a correction mode. */
@Component({
  selector: 'app-phases',
  imports: [TranslocoDirective, RouterLink, NgTemplateOutlet, ScorePanel, HistoryPanel, WithBibPipe],
  templateUrl: './phases.html',
  styleUrl: './phases.scss',
})
export class Phases {
  protected readonly tournament = inject(TournamentStore);
  protected readonly view = liveView<PhasesView>(() => TournamentService.GetPhasesView(), { categories: [] });

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
    return stages.find((s) => s.id === this.stageKey()) ?? stages.find((s) => !s.complete) ?? stages[stages.length - 1] ?? null;
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

  protected round(r: RoundView): { key: string; params: Record<string, number> } {
    return roundLabel({ placeFrom: r.from, placeTo: r.to, roundSize: r.size });
  }

  protected shortRound(m: MatchView): { key: string; params: Record<string, number> } {
    return shortRoundLabel(m);
  }

  /**
   * A bracket cut into sub-brackets, one under the other: the main draw (with
   * the third place match), then each part playing for lower places (5–8…).
   * A part is rooted at a round of 4 or more starting after place 1; the
   * rounds starting at or after its root, up to the next root, belong to it.
   */
  protected sections(b: StagePhase): BracketSection[] {
    const rounds = b.rounds ?? [];
    const roots = [...new Set(rounds.filter((r) => r.size >= 4 && r.from > 1).map((r) => r.from))].sort((x, y) => x - y);
    const out = new Map<number, BracketSection>();
    for (const r of rounds) {
      const start = roots.filter((s) => s <= r.from).pop() ?? 1;
      const sec = out.get(start) ?? { start, end: start, rounds: [] };
      sec.end = Math.max(sec.end, r.to);
      sec.rounds.push(r);
      out.set(start, sec);
    }
    return [...out.values()].sort((x, y) => x.start - y.start);
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
