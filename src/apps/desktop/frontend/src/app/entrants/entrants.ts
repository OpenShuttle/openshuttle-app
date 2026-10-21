import { Component, ElementRef, computed, inject, signal, viewChild } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { EntrantView, EntrantsView, ImportFile, TournamentService } from '@bindings/apps/desktop';
import { Category, EntrantID } from '@bindings/internal/engine';
import { ImportReport, PlayerInput } from '@bindings/internal/tournament';

import { backendError } from '../core/backend-error';
import { LanguageService } from '../core/language.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { ClubSelect } from '../shared/club-select';
import { ImportMapping } from './import-mapping';
import { toCsv } from '../shared/csv';

const emptyPlayers = (count: number): PlayerInput[] =>
  Array.from({ length: count }, () => ({ name: '', club: '' }));

/** Entrants: one tab per category, add, edit and remove players or pairs. */
@Component({
  selector: 'app-entrants',
  imports: [TranslocoDirective, RouterLink, CategoryBadge, ClubSelect, ImportMapping],
  templateUrl: './entrants.html',
  styleUrl: './entrants.scss',
})
export class Entrants {
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);
  private readonly language = inject(LanguageService);
  protected readonly importReport = signal<ImportReport | null>(null);
  /** The player list chosen for import, while its columns are mapped. */
  protected readonly importFile = signal<ImportFile | null>(null);
  protected readonly exported = signal<string | null>(null);
  protected readonly view = liveView<EntrantsView>(() => TournamentService.GetEntrantsView(), {
    categories: [],
  });

  /** Category of the add form. */
  private readonly selected = signal<Category | null>(null);
  protected readonly current = computed(() => {
    const categories = this.view().categories ?? [];
    return categories.find((c) => c.category === this.selected()) ?? categories[0] ?? null;
  });
  /** Category shown in the list; 'all' mixes them. */
  protected readonly filter = signal<Category | 'all'>('all');

  protected readonly rows = computed(() =>
    (this.view().categories ?? [])
      .filter((c) => this.filter() === 'all' || c.category === this.filter())
      .flatMap((c) => (c.entrants ?? []).map((e) => ({ entrant: e, category: c.category, slot: c.slot }))),
  );

  protected readonly totals = computed(() => {
    const categories = this.view().categories ?? [];
    return {
      entrants: categories.reduce((n, c) => n + (c.entrants ?? []).length, 0),
      players: categories.reduce((n, c) => n + c.players, 0),
      doubles: categories.length > 0 && categories.every((c) => c.doubles),
    };
  });

  /** Players being typed in the add form. */
  protected readonly draft = signal<PlayerInput[]>(emptyPlayers(1));
  /** Entrant being edited, and its players. */
  protected readonly editing = signal<EntrantID | null>(null);
  protected readonly editDraft = signal<PlayerInput[]>([]);

  private readonly firstField = viewChild<ElementRef<HTMLInputElement>>('firstField');

  protected select(category: Category): void {
    this.selected.set(category);
    this.resetDraft();
  }

  protected size(): number {
    return this.current()?.doubles ? 2 : 1;
  }

  protected resetDraft(): void {
    this.draft.set(emptyPlayers(this.size()));
  }

  protected setDraft(index: number, field: keyof PlayerInput, value: string): void {
    this.draft.update((players) => {
      const next = players.length === this.size() ? [...players] : emptyPlayers(this.size());
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  }

  protected draftValue(index: number, field: keyof PlayerInput): string {
    return this.draft()[index]?.[field] ?? '';
  }

  protected async add(): Promise<void> {
    const category = this.current()?.category;
    const players = emptyPlayers(this.size()).map((empty, i) => this.draft()[i] ?? empty);
    if (category && (await this.tournament.registerEntrant(category, players))) {
      this.resetDraft();
      this.firstField()?.nativeElement.focus();
    }
  }

  protected edit(entrant: EntrantView): void {
    this.editing.set(entrant.id);
    this.editDraft.set(
      (entrant.players ?? []).map((p) => ({ name: p.name, club: p.club })),
    );
  }

  protected setEdit(index: number, field: keyof PlayerInput, value: string): void {
    this.editDraft.update((players) => {
      const next = [...players];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  }

  protected async saveEdit(): Promise<void> {
    const id = this.editing();
    if (id && (await this.tournament.updateEntrant(id, this.editDraft()))) {
      this.editing.set(null);
    }
  }

  /**
   * Chooses a player list to import; its columns are mapped before anything
   * is entered. Lines without category go to the add form's category.
   */
  protected async importList(): Promise<void> {
    const category = this.current()?.category;
    if (!category) {
      return;
    }
    try {
      const file = await TournamentService.ChooseImportFile(category);
      this.importFile.set(file.path ? file : null);
      this.importReport.set(null);
      this.tournament.error.set(null);
    } catch (err) {
      this.tournament.error.set(backendError(err));
    }
  }

  /** The mapping is done: imported (its report) or cancelled (null). */
  protected imported(report: ImportReport | null): void {
    this.importFile.set(null);
    this.importReport.set(report && (report.added > 0 || (report.skipped ?? []).length > 0 || report.ignored > 0) ? report : null);
  }

  /**
   * Exports the player list in the layout the import reads: category,
   * player 1, club 1, player 2, club 2. ";" in French.
   */
  protected async exportList(): Promise<void> {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const rows: string[][] = [
      [t('entrants.csv.category'), t('entrants.csv.player', { n: 1 }), t('entrants.csv.club', { n: 1 }), t('entrants.csv.player', { n: 2 }), t('entrants.csv.club', { n: 2 })],
    ];
    for (const c of this.view().categories ?? []) {
      for (const e of c.entrants ?? []) {
        rows.push([t('category.short.' + c.category), ...(e.players ?? []).flatMap((p) => [p.name, p.club])]);
      }
    }
    const separator = this.language.active === 'fr' ? ';' : ',';
    try {
      const path = await TournamentService.ExportCSV(`${this.tournament.view().name} - ${t('entrants.csv.fileSuffix')}.csv`, toCsv(rows, separator));
      this.exported.set(path || null);
      this.tournament.error.set(null);
    } catch (err) {
      this.tournament.error.set(backendError(err));
    }
  }

  protected present(e: EntrantView): number {
    return (e.players ?? []).filter((p) => p.checkedIn).length;
  }

  protected hasDuplicate(e: EntrantView): boolean {
    return (e.players ?? []).some((p) => p.duplicate);
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }
}
