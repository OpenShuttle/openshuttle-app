import { Component, computed, inject, input } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { MatchView } from '@bindings/apps/desktop';
import { StageKind } from '@bindings/internal/tournament';

import { stageLabel } from './stage-label';

/** Translation key and params naming the round of a bracket match. */
export function roundLabel(m: Pick<MatchView, 'placeFrom' | 'placeTo' | 'roundSize'>): {
  key: string;
  params: Record<string, number>;
} {
  if (m.placeFrom > 1) {
    return m.roundSize === 2
      ? { key: 'round.placeMatch', params: { place: m.placeFrom } }
      : { key: 'round.places', params: { from: m.placeFrom, to: m.placeTo } };
  }
  switch (m.roundSize) {
    case 2:
      return { key: 'round.final', params: {} };
    case 4:
      return { key: 'round.semi', params: {} };
    case 8:
      return { key: 'round.quarter', params: {} };
    case 16:
      return { key: 'round.eighth', params: {} };
    default:
      return { key: 'round.ofN', params: { n: m.roundSize } };
  }
}

/**
 * Translation key and params of the short name of a bracket round, as the
 * schedule shows it: "1/4", "Finale", "3e place", "5e–8e" (en: "QF", "Final",
 * "3rd place", "5th–8th").
 */
export function shortRoundLabel(m: Pick<MatchView, 'placeFrom' | 'placeTo' | 'roundSize'>): {
  key: string;
  params: Record<string, number>;
} {
  if (m.placeFrom > 1) {
    return m.roundSize === 2
      ? { key: 'round.short.place', params: { place: m.placeFrom } }
      : { key: 'round.short.places', params: { from: m.placeFrom, to: m.placeTo } };
  }
  switch (m.roundSize) {
    case 2:
      return { key: 'round.short.final', params: {} };
    case 4:
      return { key: 'round.short.semi', params: {} };
    case 8:
      return { key: 'round.short.quarter', params: {} };
    case 16:
      return { key: 'round.short.eighth', params: {} };
    default:
      return { key: 'round.short.ofN', params: { n: m.roundSize } };
  }
}

/**
 * Where a match is played: "Pool A", "Pools 2 · Pool A" in a later stage, or
 * "Main bracket · Semi-final".
 */
@Component({
  selector: 'app-match-stage',
  imports: [TranslocoDirective],
  template: `<ng-container *transloco="let t">{{ label() }}</ng-container>`,
})
export class MatchStage {
  readonly match = input.required<MatchView>();
  private readonly transloco = inject(TranslocoService);

  protected readonly label = computed(() => {
    const m = this.match();
    const t = this.transloco;
    const stage = stageLabel(t, { name: m.stageName, kind: m.stageKind });
    if (m.stageKind === StageKind.StageBracket) {
      const r = roundLabel(m);
      return `${stage} · ${t.translate(r.key, r.params)}`;
    }
    const pool = t.translate('desk.pool', { pool: m.pool });
    return m.entryStage ? pool : `${stage} · ${pool}`;
  });
}
