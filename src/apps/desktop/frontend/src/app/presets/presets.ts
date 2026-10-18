import { Component, inject } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { FormatPreset, PresetService } from '@bindings/apps/desktop';
import { Category, WinCondition } from '@bindings/internal/engine';
import { StageConfig, StageKind } from '@bindings/internal/tournament';

import { AppPresets } from '../core/presets.service';
import { TournamentStore } from '../core/tournament.service';
import { stageLabel, winSummary } from '../shared/stage-label';

/** Presets of this PC: formats, win conditions and tiebreak orders. */
@Component({
  selector: 'app-presets',
  imports: [TranslocoDirective],
  templateUrl: './presets.html',
  styleUrl: './presets.scss',
})
export class PresetsPage {
  protected readonly presets = inject(AppPresets);
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);

  protected builtIn(id: string): boolean {
    return id.startsWith('b-');
  }

  protected formatName(p: FormatPreset): string {
    return p.name || this.transloco.translate('presets.format.' + p.key + '.name');
  }

  protected formatDescription(p: FormatPreset): string {
    return p.description || (p.key ? this.transloco.translate('presets.format.' + p.key + '.description') : '');
  }

  protected tiebreakName(p: { key: string; name: string }): string {
    return p.name || this.transloco.translate('presets.tiebreaks.' + p.key);
  }

  protected win(w: WinCondition): string {
    return winSummary(this.transloco, w);
  }

  /** Stages of a preset, phase by phase, for the mini diagram. */
  protected columns(p: FormatPreset): StageConfig[][] {
    const phase = new Map<string, number>();
    for (const st of p.stages ?? []) {
      if (!phase.has(st.id)) {
        phase.set(st.id, 1);
      }
      for (const to of st.routing ?? []) {
        if (to) {
          phase.set(to, Math.max(phase.get(to) ?? 1, (phase.get(st.id) ?? 1) + 1));
        }
      }
    }
    const cols: StageConfig[][] = [];
    for (const st of p.stages ?? []) {
      const i = (phase.get(st.id) ?? 1) - 1;
      (cols[i] ??= []).push(st);
    }
    return cols.filter((c) => c);
  }

  protected chip(st: StageConfig): string {
    const label = stageLabel(this.transloco, st);
    return st.kind === StageKind.StagePools ? `${label} × ${st.pools}` : label;
  }

  protected remove(id: string): void {
    void this.presets.run(() => PresetService.DeletePreset(id));
  }

  protected apply(p: FormatPreset, category: Category): void {
    void this.tournament.applyFormatPreset(category, p.id);
  }
}
