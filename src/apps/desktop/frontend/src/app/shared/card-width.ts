import { TranslocoService } from '@jsverse/transloco';
import { MatchView } from '@bindings/apps/desktop';

import { bracketMatchName, sourceLabel } from './bracket-labels';
import { clubs, fullNames } from './names';

/** Narrowest and widest bracket card, in px. */
const MIN_WIDTH = 150;
const MAX_WIDTH = 260;
/** Room around a team's name: paddings, gaps, the trophy and a place ("3rd"). */
const SIDE_ROOM = 20 + 8 + 16 + 8;
/** Width of a pool and rank chip ("A1") and its gap. */
const ORIGIN = 34;

let canvas: CanvasRenderingContext2D | null | undefined;

/** Width of a text in the app's font, estimated when no canvas measures it (tests). */
function textWidth(text: string, font: string): number {
  if (canvas === undefined) {
    try {
      canvas = document.createElement('canvas').getContext('2d');
    } catch {
      canvas = null;
    }
  }
  if (!canvas) {
    return text.length * 7.5;
  }
  const family = getComputedStyle(document.body).fontFamily || 'sans-serif';
  canvas.font = `${font} ${family}`;
  return canvas.measureText(text).width;
}

/**
 * The width bracket cards need: their longest line (a team's names, a side
 * to come such as "Winner QF A", a club, or the head with its score), within
 * MIN_WIDTH and MAX_WIDTH; longer names are cut with an ellipsis.
 */
export function cardWidth(
  t: TranslocoService,
  matches: MatchView[],
  options: { bibs: boolean; clubs: boolean; scores: boolean; origins: boolean; size?: number },
): number {
  let width = MIN_WIDTH;
  for (const m of matches) {
    const head = bracketMatchName(t, m);
    const score = options.scores ? (m.score?.sets ?? []).map((s) => `${s.a}-${s.b}`).join(' · ') : '';
    width = Math.max(width, textWidth(head, '700 12px') + textWidth(score, '700 12px') + 40);
    const sides: [MatchView['a'], MatchView['fromA']][] = [
      [m.a, m.fromA],
      [m.b, m.fromB],
    ];
    for (const [side, from] of sides) {
      const label = side.entrant
        ? (options.bibs ? `#${side.bib} ` : '') + fullNames(side)
        : from
          ? sourceLabel(t, from)
          : '';
      const origin = options.origins && side.entrant && m.placeFrom === 1 && m.roundSize === options.size ? ORIGIN : 0;
      width = Math.max(width, textWidth(label, '800 14px') + origin + SIDE_ROOM);
      if (options.clubs && side.entrant) {
        width = Math.max(width, textWidth(clubs(side), '500 12px') + origin + SIDE_ROOM);
      }
    }
  }
  return Math.ceil(Math.min(MAX_WIDTH, width));
}
