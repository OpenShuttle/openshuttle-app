import { Injectable, computed, inject, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';
import { PresetService, Presets } from '@bindings/apps/desktop';
import { Tiebreak, WinCondition, WinConditionKind } from '@bindings/internal/engine';

import { BackendError, backendError } from './backend-error';
import { AppSettings } from './settings.service';

const EMPTY: Presets = { win: [], tiebreaks: [], formats: [], disabled: [] };

/** Presets of this PC (win conditions, tiebreak orders, formats), kept in sync. */
@Injectable({ providedIn: 'root' })
export class AppPresets {
  readonly value = signal<Presets>(EMPTY);
  readonly error = signal<BackendError | null>(null);
  private readonly settings = inject(AppSettings);

  /** The win condition of a new stage or format: the default preset of the settings. */
  readonly defaultWin = computed<WinCondition>(
    () =>
      (this.value().win ?? []).find((p) => p.id === this.settings.value()?.defaultWin)?.win ?? {
        kind: WinConditionKind.WinBySets,
        setsToWin: 2,
        pointsPerSet: 15,
        cap: 21,
      },
  );

  /** The tiebreak order of a new pools stage: the default preset of the settings. */
  readonly defaultTiebreaks = computed<Tiebreak[]>(() => [
    ...((this.value().tiebreaks ?? []).find((p) => p.id === this.settings.value()?.defaultTiebreaks)?.tiebreaks ??
      this.value().tiebreaks?.[0]?.tiebreaks ??
      []),
  ]);

  /** The presets offered in the tournament screens: the disabled ones left out. */
  readonly offered = computed<Presets>(() => {
    const p = this.value();
    const on = (x: { id: string }) => this.enabled(x.id);
    return { ...p, win: (p.win ?? []).filter(on), tiebreaks: (p.tiebreaks ?? []).filter(on), formats: (p.formats ?? []).filter(on) };
  });

  enabled(id: string): boolean {
    return !(this.value().disabled ?? []).includes(id);
  }

  constructor() {
    Events.On('presets.changed', (event) => this.value.set(event.data));
    void PresetService.GetPresets().then((p) => this.value.set(p));
  }

  /** Runs a preset command; Go broadcasts the new presets on success. */
  async run(call: () => Promise<unknown>): Promise<boolean> {
    try {
      await call();
      this.error.set(null);
      return true;
    } catch (err) {
      this.error.set(backendError(err));
      return false;
    }
  }
}
