import { TranslocoService } from '@jsverse/transloco';
import { WinCondition, WinConditionKind } from '@bindings/internal/engine';

/** A stage's name, or its kind's label when the desk left it empty. */
export function stageLabel(t: TranslocoService, stage: { name: string; kind: string }): string {
  return stage.name || t.translate('stage.kind.' + stage.kind);
}

/** "2 sets of 21 · cap 30" or "12 min · golden point". */
export function winSummary(t: TranslocoService, win: WinCondition): string {
  if (win.kind === WinConditionKind.WinByTime) {
    return t.translate('format.timedSummary', { minutes: win.minutes ?? 0 });
  }
  return t.translate('format.setsSummary', { sets: win.setsToWin ?? 1, points: win.pointsPerSet ?? 21, cap: win.cap ?? 0 });
}
