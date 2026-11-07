import { TranslocoService } from '@jsverse/transloco';
import { MatchSource, MatchView } from '@bindings/apps/desktop';

import { shortRoundLabel } from './match-stage';

type Named = Pick<MatchView, 'placeFrom' | 'placeTo' | 'roundSize' | 'letter'>;

/** Short name of a bracket match: "1/4 A", "5e–8e B", "Finale" (en: "QF A", "Final"). */
export function bracketMatchName(t: TranslocoService, m: Named): string {
  const r = shortRoundLabel(m);
  const round = t.translate(r.key, r.params);
  return m.letter ? `${round} ${m.letter}` : round;
}

/** A side not known yet: "Vainqueur 1/4 A", "Perdant 1/2 B". */
export function sourceLabel(t: TranslocoService, s: MatchSource): string {
  return t.translate(s.loser ? 'bracket.loserOf' : 'bracket.winnerOf', { match: bracketMatchName(t, s) });
}

/** Side that won a decided match: by forfeit, else by sets. */
export function matchWinner(m: MatchView): 'a' | 'b' | '' {
  if (m.walkoverA || m.walkoverB) {
    return m.walkoverA && m.walkoverB ? '' : m.walkoverA ? 'b' : 'a';
  }
  let a = 0;
  let b = 0;
  for (const set of m.score?.sets ?? []) {
    if (set.a > set.b) a++;
    else if (set.b > set.a) b++;
  }
  return a > b ? 'a' : b > a ? 'b' : '';
}
