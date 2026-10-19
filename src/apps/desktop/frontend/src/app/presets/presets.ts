import { Component, effect, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { FormatPreset, PresetService } from '@bindings/apps/desktop';
import { Tiebreak, WinCondition, WinConditionKind } from '@bindings/internal/engine';
import { StageConfig, StageKind } from '@bindings/internal/tournament';

import { AppPresets } from '../core/presets.service';
import { stageLabel, winSummary } from '../shared/stage-label';

const ALL_TIEBREAKS: Tiebreak[] = ['wins', 'headToHead', 'setDifference', 'pointDifference', 'draw'] as Tiebreak[];
const POINT_CHOICES = [11, 15, 21];

interface WinDraft {
  id: string;
  name: string;
  win: WinCondition;
}

interface TiebreakDraft {
  id: string;
  name: string;
  tiebreaks: Tiebreak[];
}

/**
 * Presets of this PC: formats, win conditions and tiebreak orders. Built-in
 * presets can be duplicated, never changed; saved ones can be edited,
 * duplicated and deleted. Formats open in their own editor.
 */
@Component({
  selector: 'app-presets',
  imports: [TranslocoDirective],
  templateUrl: './presets.html',
  styleUrl: './presets.scss',
})
export class PresetsPage {
  protected readonly presets = inject(AppPresets);
  private readonly transloco = inject(TranslocoService);
  private readonly router = inject(Router);
  protected readonly kinds = WinConditionKind;
  protected readonly pointChoices = POINT_CHOICES;

  protected readonly winDraft = signal<WinDraft | null>(null);
  protected readonly tiebreakDraft = signal<TiebreakDraft | null>(null);
  /** Club being added (old "") or renamed. */
  protected readonly clubDraft = signal<{ old: string; name: string } | null>(null);

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
    (p.stages ?? []).forEach((st, i, all) => {
      if (!phase.has(st.id)) {
        phase.set(st.id, i === 0 ? 1 : (phase.get(all[i - 1].id) ?? 1) + 1);
      }
      for (const r of st.routing ?? []) {
        phase.set(r.to, Math.max(phase.get(r.to) ?? 1, (phase.get(st.id) ?? 1) + 1));
      }
    });
    const cols: StageConfig[][] = [];
    for (const st of p.stages ?? []) {
      const i = (phase.get(st.id) ?? 1) - 1;
      (cols[i] ??= []).push(st);
    }
    return cols.filter((c) => c);
  }

  protected chip(st: StageConfig): string {
    const label = stageLabel(this.transloco, st);
    if (st.kind !== StageKind.StagePools) {
      return label;
    }
    const sizes = st.poolSizes ?? [];
    return sizes.length ? `${label} · ${sizes.join('/')}` : `${label} × ${st.pools}`;
  }

  protected copyName(name: string): string {
    return this.transloco.translate('home.copyName', { name });
  }

  protected duplicate(id: string, name: string): void {
    void this.presets.run(() => PresetService.DuplicatePreset(id, this.copyName(name)));
  }

  /** Offers a preset in the tournament screens, or stops offering it. */
  protected setEnabled(id: string, event: Event): void {
    const enabled = (event.target as HTMLInputElement).checked;
    void this.presets.run(() => PresetService.SetPresetEnabled(id, enabled));
  }

  protected async saveClub(): Promise<void> {
    const d = this.clubDraft();
    if (d && (await this.presets.run(() => PresetService.SaveClub(d.old, d.name)))) {
      this.clubDraft.set(null);
    }
  }

  protected deleteClub(name: string): void {
    void this.presets.run(() => PresetService.DeleteClub(name));
  }

  protected remove(id: string): void {
    void this.presets.run(() => PresetService.DeletePreset(id));
  }

  protected openFormat(id: string): void {
    void this.router.navigate(['/presets/format', id]);
  }

  // Win conditions.

  protected newWin(): void {
    this.winDraft.set({ id: '', name: '', win: { kind: WinConditionKind.WinBySets, setsToWin: 2, pointsPerSet: 21, cap: 30 } });
  }

  protected editWin(p: WinDraft): void {
    this.winDraft.set({ id: p.id, name: p.name, win: { ...p.win } });
  }

  protected setWin(change: Partial<WinCondition>): void {
    this.winDraft.update((d) => (d ? { ...d, win: { ...d.win, ...change } } : d));
  }

  protected setWinKind(kind: WinConditionKind): void {
    this.setWin(
      kind === WinConditionKind.WinByTime
        ? { kind, minutes: 12, setsToWin: 0, pointsPerSet: 0, cap: 0, estimate: 0 }
        : { kind, setsToWin: 2, pointsPerSet: 21, cap: 30, minutes: 0, estimate: 0 },
    );
  }

  /** What the schedule uses for the win condition being edited. */
  protected readonly draftEstimate = signal(0);
  /** Default estimates of the listed win conditions without one set. */
  protected readonly estimates = signal<Record<string, number>>({});

  constructor() {
    // Go knows the default estimates: ask for the draft and the list.
    effect(() => {
      const d = this.winDraft();
      if (d) {
        void PresetService.EstimateMinutes(d.win).then((n) => this.draftEstimate.set(n));
      }
    });
    effect(() => {
      const list = this.presets.value().win ?? [];
      void Promise.all(list.map((p) => PresetService.EstimateMinutes(p.win))).then((ns) =>
        this.estimates.set(Object.fromEntries(list.map((p, i) => [p.id, ns[i]]))),
      );
    });
  }

  protected stepEstimate(delta: number): void {
    this.setWin({ estimate: Math.max(1, this.draftEstimate() + delta) });
  }

  protected stepWin(field: 'setsToWin' | 'pointsPerSet' | 'cap' | 'minutes', delta: number): void {
    const d = this.winDraft();
    if (d) {
      this.setWin({ [field]: Math.max(0, (d.win[field] ?? 0) + delta) });
    }
  }

  protected async saveWin(): Promise<void> {
    const d = this.winDraft();
    if (d && (await this.presets.run(() => PresetService.SaveWinPreset(d.id, d.name, d.win)))) {
      this.winDraft.set(null);
    }
  }

  // Tiebreak orders.

  protected newTiebreaks(): void {
    this.tiebreakDraft.set({ id: '', name: '', tiebreaks: [...ALL_TIEBREAKS] });
  }

  protected editTiebreaks(p: { id: string; name: string; tiebreaks: Tiebreak[] | null }): void {
    this.tiebreakDraft.set({ id: p.id, name: p.name, tiebreaks: [...(p.tiebreaks ?? [])] });
  }

  protected moveTiebreak(index: number, step: -1 | 1): void {
    this.tiebreakDraft.update((d) => {
      if (!d) {
        return d;
      }
      const tiebreaks = [...d.tiebreaks];
      const target = index + step;
      if (target >= 0 && target < tiebreaks.length) {
        [tiebreaks[index], tiebreaks[target]] = [tiebreaks[target], tiebreaks[index]];
      }
      return { ...d, tiebreaks };
    });
  }

  protected toggleTiebreak(tb: Tiebreak): void {
    this.tiebreakDraft.update((d) =>
      d ? { ...d, tiebreaks: d.tiebreaks.includes(tb) ? d.tiebreaks.filter((x) => x !== tb) : [...d.tiebreaks, tb] } : d,
    );
  }

  protected missingTiebreaks(d: TiebreakDraft): Tiebreak[] {
    return ALL_TIEBREAKS.filter((tb) => !d.tiebreaks.includes(tb));
  }

  protected async saveTiebreaks(): Promise<void> {
    const d = this.tiebreakDraft();
    if (d && (await this.presets.run(() => PresetService.SaveTiebreakPreset(d.id, d.name, d.tiebreaks)))) {
      this.tiebreakDraft.set(null);
    }
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }
}
