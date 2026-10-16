import { Component, computed, input } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { MatchView } from '@bindings/apps/desktop';

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
    default:
      return { key: 'round.ofN', params: { n: m.roundSize } };
  }
}

/** "Pool A", or "Main bracket · Semi-final" for a bracket match. */
@Component({
  selector: 'app-match-stage',
  imports: [TranslocoDirective],
  template: `<ng-container *transloco="let t">{{
    match().bracket
      ? match().bracketName + ' · ' + t(round().key, round().params)
      : t('desk.pool', { pool: match().pool })
  }}</ng-container>`,
})
export class MatchStage {
  readonly match = input.required<MatchView>();
  protected readonly round = computed(() => roundLabel(this.match()));
}
