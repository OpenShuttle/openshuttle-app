import { DestroyRef, Injectable, inject } from '@angular/core';
import { TranslocoService } from '@jsverse/transloco';
import { Events } from '@wailsio/runtime';
import { TournamentService } from '@bindings/apps/desktop';

import { LanguageService } from '../core/language.service';
import { AppSettings } from '../core/settings.service';
import { snapshotHtml } from './snapshot-html';

/** Actions often come in bursts (a round, an import): write once they settle. */
export const SNAPSHOT_DELAY_MS = 1000;

/**
 * Writes the readable state of the open tournament beside its file
 * ("Tournament.html") after each action: the main window builds it from the
 * views (it holds the translations), Go writes it. A failure never shows: the
 * tournament file stays the reference.
 */
@Injectable({ providedIn: 'root' })
export class SnapshotWriter {
  private readonly transloco = inject(TranslocoService);
  private readonly language = inject(LanguageService);
  private readonly settings = inject(AppSettings);
  private timer: ReturnType<typeof setTimeout> | undefined;
  private started = false;

  /** Follows tournament.updated until the caller is destroyed. */
  start(destroy: DestroyRef): void {
    if (this.started) {
      return;
    }
    this.started = true;
    const off = Events.On('tournament.updated', () => this.schedule());
    destroy.onDestroy(() => {
      off();
      clearTimeout(this.timer);
      this.started = false;
    });
    // The tournament may already be open (a file opened at launch).
    this.schedule();
  }

  private schedule(): void {
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.write(), SNAPSHOT_DELAY_MS);
  }

  /** Builds and writes the snapshot of the open tournament, if any. */
  async write(): Promise<void> {
    try {
      const tournament = await TournamentService.GetTournamentView();
      if (!tournament.open) {
        return;
      }
      const [phases, desk, schedule] = await Promise.all([
        TournamentService.GetPhasesView(),
        TournamentService.GetControlDeskView(),
        TournamentService.GetScheduleView(),
      ]);
      const html = snapshotHtml(
        this.transloco,
        { tournament, phases, desk, schedule },
        {
          time: (v) => this.language.time(v),
          date: (v) => this.language.date(v),
          showBibs: this.settings.showBibs(),
          lang: this.language.active,
          now: new Date(),
        },
      );
      // Go skips it when an action came in meanwhile: the next one follows.
      await TournamentService.WriteSnapshot(tournament.version, html);
    } catch {
      // Logged by Go; the next action tries again.
    }
  }
}
