import { Component, computed, effect, inject, input, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { FormatPreset, PresetService, StageView } from '@bindings/apps/desktop';
import { CrossPool, Tiebreak, WinConditionKind } from '@bindings/internal/engine';
import { FormatPreview, StageConfig, StageKind } from '@bindings/internal/tournament';

import { backendError, BackendError } from '../core/backend-error';
import { AppPresets } from '../core/presets.service';
import { FormatEditor, LAST_STAGE } from '../format-editor/format-editor';

const DEFAULT_TIEBREAKS = ['wins', 'headToHead', 'setDifference', 'pointDifference', 'draw'] as Tiebreak[];

/** A new format: pools of 4 (else 3, else 5) and nothing after. */
function newFormat(): StageConfig[] {
  return [
    {
      id: 'S1',
      name: '',
      kind: StageKind.StagePools,
      win: { kind: WinConditionKind.WinBySets, setsToWin: 2, pointsPerSet: 21, cap: 30 },
      pools: 0,
      poolSizes: [4, 3, 5],
      tiebreaks: [...DEFAULT_TIEBREAKS],
      crossPool: CrossPool.CrossPoolSameMatches,
      fullPlacement: false,
      routing: [],
    },
  ];
}

/**
 * Edits a format preset outside any tournament, with a simulation for a
 * number of teams: what the pools, routing and brackets give for real.
 * Built-in presets open read-only: duplicate them to change them.
 */
@Component({
  selector: 'app-format-preset',
  imports: [TranslocoDirective, RouterLink, FormatEditor],
  templateUrl: './format-preset.html',
  styleUrl: './format-preset.scss',
})
export class FormatPresetPage {
  /** Route parameter: a preset ID, or "new". */
  readonly id = input.required<string>();

  protected readonly presets = inject(AppPresets);
  private readonly transloco = inject(TranslocoService);
  private readonly router = inject(Router);

  protected readonly name = signal('');
  protected readonly description = signal('');
  protected readonly stages = signal<StageConfig[]>(newFormat());
  protected readonly entrants = signal(16);
  protected readonly preview = signal<FormatPreview | null>(null);
  protected readonly error = signal<BackendError | null>(null);
  protected readonly loaded = signal(false);
  protected readonly stageKinds = StageKind;

  protected readonly preset = computed<FormatPreset | null>(
    () => (this.presets.value().formats ?? []).find((p) => p.id === this.id()) ?? null,
  );
  protected readonly readonly = computed(() => this.id().startsWith('b-'));

  /** Stages with their simulation, as the editor shows them. */
  protected readonly views = computed<StageView[]>(() => {
    const preview = this.preview();
    return this.stages().map((s, i) => ({
      ...s,
      ...(preview?.stages?.[i] ?? emptyPreview()),
      entry: i === 0,
      filled: false,
      confirmed: false,
      started: false,
    }));
  });

  constructor() {
    // Load the preset once the presets are known.
    effect(() => {
      const p = this.preset();
      if (this.loaded() || (this.id() !== 'new' && !p)) {
        return;
      }
      if (p) {
        this.name.set(p.name || this.transloco.translate('presets.format.' + p.key + '.name'));
        this.description.set(p.description || (p.key ? this.transloco.translate('presets.format.' + p.key + '.description') : ''));
        this.stages.set(structuredClone(p.stages ?? []));
      }
      this.loaded.set(true);
    });
    // Simulate on every change: the format or the number of teams.
    effect(() => {
      const stages = this.stages();
      const n = this.entrants();
      PresetService.PreviewFormat(stages, n).then(
        (p) => {
          this.preview.set(p);
          this.error.set(null);
        },
        (err) => this.error.set(backendError(err)),
      );
    });
  }

  protected setEntrants(event: Event): void {
    const n = Math.round(Number((event.target as HTMLInputElement).value));
    if (Number.isFinite(n) && n >= 0 && n <= 512) {
      this.entrants.set(n);
    }
  }

  protected stepEntrants(delta: number): void {
    this.entrants.update((n) => Math.min(512, Math.max(0, n + delta)));
  }

  protected add(kind: StageKind, editor: FormatEditor): void {
    const stages = this.stages();
    const next = 1 + Math.max(0, ...stages.map((s) => Number(s.id.replace(/^S/, '')) || 0));
    const last = stages[stages.length - 1];
    const stage: StageConfig =
      kind === StageKind.StagePools
        ? { ...newFormat()[0], id: `S${next}`, win: last?.win ?? newFormat()[0].win }
        : { id: `S${next}`, name: '', kind, win: last?.win ?? newFormat()[0].win, fullPlacement: true, routing: [] };
    this.stages.set([...stages, stage]);
    editor.selectedID.set(LAST_STAGE);
  }

  protected remove(id: string, editor: FormatEditor): void {
    this.stages.update((stages) =>
      stages
        .filter((s) => s.id !== id)
        .map((s) => ({ ...s, routing: (s.routing ?? []).filter((r) => r.to !== id) })),
    );
    editor.selectedID.set('');
  }

  protected async save(): Promise<void> {
    const id = this.id() === 'new' ? '' : this.id();
    const ok = await this.presets.run(() =>
      PresetService.SaveFormatPreset(id, this.name(), this.description(), this.stages()),
    );
    if (ok) {
      void this.router.navigateByUrl('/presets');
    }
  }

  protected async duplicate(): Promise<void> {
    const before = new Set((this.presets.value().formats ?? []).map((p) => p.id));
    const name = this.transloco.translate('home.copyName', { name: this.name() });
    if (await this.presets.run(() => PresetService.DuplicatePreset(this.id(), name))) {
      const all = await PresetService.GetPresets();
      const copy = (all.formats ?? []).find((p) => !before.has(p.id));
      if (copy) {
        this.loaded.set(false);
        void this.router.navigate(['/presets/format', copy.id]);
      }
    }
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }
}

function emptyPreview() {
  return {
    phase: 1,
    entrants: 0,
    sizes: [],
    noSplit: false,
    byes: 0,
    matches: 0,
    rounds: 0,
    ranks: [],
    rules: [],
    kept: 0,
    placeFrom: 0,
    placeTo: 0,
  };
}
