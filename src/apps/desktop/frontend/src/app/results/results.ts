import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ResultRow, ResultsView, TournamentService } from '@bindings/apps/desktop';

import { backendError } from '../core/backend-error';
import { LanguageService } from '../core/language.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { fullNames } from '../shared/names';
import { stageLabel } from '../shared/stage-label';
import { StageKind } from '@bindings/internal/tournament';
import { toCsv } from '../shared/csv';

/** Results: full standings per category, CSV export, archive. */
@Component({
  selector: 'app-results',
  imports: [TranslocoDirective, RouterLink],
  templateUrl: './results.html',
  styleUrl: './results.scss',
})
export class Results {
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);
  private readonly language = inject(LanguageService);
  protected readonly view = liveView<ResultsView>(() => TournamentService.GetResultsView(), {
    name: '',
    archived: false,
    categories: [],
  });

  /** Path of the last export, shown as a confirmation. */
  protected readonly exported = signal<string | null>(null);
  protected readonly confirmingArchive = signal(false);

  private readonly selected = signal<string | null>(null);
  protected readonly current = computed(() => {
    const categories = this.view().categories ?? [];
    return categories.find((c) => c.category === this.selected()) ?? categories[0] ?? null;
  });
  /** True while some category is not final: the standings are a preview. */
  protected readonly provisional = computed(() =>
    (this.view().categories ?? []).some((c) => c.started && !c.complete),
  );

  protected select(category: string): void {
    this.selected.set(category);
  }

  protected short(row: ResultRow): string {
    return fullNames(row.entrant);
  }

  protected names(row: ResultRow): string {
    return (row.entrant.names ?? []).join(' / ');
  }

  protected rank(row: ResultRow): string {
    if (!row.rank) {
      return '–';
    }
    return row.tied ? this.transloco.translate('results.tiedRank', { rank: row.rank }) : String(row.rank);
  }

  /** Where the rank was earned: "Main bracket · place 3", "Pools · 2nd of pool". */
  protected origin(row: ResultRow): string {
    const stage = stageLabel(this.transloco, { name: row.stageName, kind: row.stageKind });
    if (!row.stageRank) {
      return stage;
    }
    const rank = this.transloco.translate(
      row.stageKind === StageKind.StageBracket ? 'results.bracketPlace' : 'results.poolRank',
      { n: row.stageRank },
    );
    return `${stage} · ${rank}`;
  }

  /** One CSV for every category; ";" in French so spreadsheets split columns. */
  protected async exportCsv(): Promise<void> {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const separator = this.language.active === 'fr' ? ';' : ',';
    const rows: (string | number)[][] = [
      [t('results.csv.category'), t('results.csv.rank'), t('results.csv.bib'), t('results.csv.entrant'), t('results.csv.club'), t('results.csv.origin')],
    ];
    for (const c of this.view().categories ?? []) {
      for (const r of c.rows ?? []) {
        rows.push([
          t('category.name.' + c.category),
          this.rank(r),
          r.entrant.bib || '',
          this.names(r),
          (r.entrant.clubs ?? []).join(' / '),
          this.origin(r),
        ]);
      }
    }
    const name = `${this.view().name} - ${t('results.csv.fileSuffix')}.csv`;
    try {
      const path = await TournamentService.ExportCSV(name, toCsv(rows, separator));
      this.exported.set(path || null);
      this.tournament.error.set(null);
    } catch (err) {
      this.tournament.error.set(backendError(err));
    }
  }

  /** Archive takes a second click: it closes the tournament. */
  protected async archive(): Promise<void> {
    if (!this.confirmingArchive()) {
      this.confirmingArchive.set(true);
      return;
    }
    this.confirmingArchive.set(false);
    try {
      await TournamentService.ArchiveTournament();
      this.tournament.error.set(null);
    } catch (err) {
      this.tournament.error.set(backendError(err));
    }
  }
}
