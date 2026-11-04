import { Component, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ImportFile, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { Club, ImportMapping as Mapping, ImportPlayerColumns, ImportPreview, ImportReport } from '@bindings/internal/tournament';

import { backendError } from '../core/backend-error';
import { AppPresets } from '../core/presets.service';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';

/** A column index meaning none (tournament.NoColumn). */
export const NO_COLUMN = -1;
/** As a partner's club column: the club of player 1 (tournament.SameClub). */
export const SAME_CLUB = -2;

/** Lines of the file shown under the mapping. */
const SAMPLE_LINES = 5;
/** Entrants to add shown before "and N more". */
const SAMPLE_ENTRANTS = 6;

/** A category of the tournament, with its badge color slot. */
export interface ImportCategory {
  category: Category;
  slot: number;
}

/**
 * What becomes of a club value matching no club of the list: added to the
 * list (name, optional acronym), entered as a listed club, or kept as typed.
 */
export type ClubChoice = { kind: 'add'; name: string; acronym: string } | { kind: 'map'; club: string } | { kind: 'keep' };

/**
 * A value not in the club list is added by default: a short word in capitals
 * ("PLVPB") as the acronym, the name left to type; anything else as the name.
 */
export function defaultClubChoice(value: string): ClubChoice {
  return /^[\p{Lu}\p{N}.&-]{2,10}$/u.test(value) ? { kind: 'add', name: '', acronym: value } : { kind: 'add', name: value, acronym: '' };
}

/** The club a value is entered as, with its choice. */
export function chosenClub(value: string, choice: ClubChoice): string {
  switch (choice.kind) {
    case 'add':
      return choice.name.trim() || choice.acronym.trim() || value;
    case 'map':
      return choice.club;
    default:
      return value;
  }
}

const noPlayer = (): ImportPlayerColumns => ({ name: NO_COLUMN, first: NO_COLUMN, last: NO_COLUMN, club: NO_COLUMN });

/**
 * Says what the columns of a chosen player list hold, from the guess Go made
 * (OpenShuttle's export, HelloAsso and other registration tools): the
 * category column and what each of its values means, the name (whole or
 * first and last) and club of each player. Go previews the result on every
 * change; nothing is saved before Import. Club values matching no club of the
 * list, by name or acronym, are listed: each is added to the list (the
 * default), entered as a listed club, or kept as typed.
 */
@Component({
  selector: 'app-import-mapping',
  imports: [TranslocoDirective, CategoryBadge],
  templateUrl: './import-mapping.html',
  styleUrl: './import-mapping.scss',
})
export class ImportMapping {
  readonly file = input.required<ImportFile>();
  readonly categories = input.required<ImportCategory[]>();
  /** The import's report, or null when cancelled. */
  readonly done = output<ImportReport | null>();

  private readonly transloco = inject(TranslocoService);
  private readonly tournament = inject(TournamentStore);
  protected readonly presets = inject(AppPresets);

  protected readonly NO_COLUMN = NO_COLUMN;
  protected readonly SAME_CLUB = SAME_CLUB;

  protected readonly mapping = signal<Mapping>({ header: false, category: NO_COLUMN, categories: {}, fallback: '' as Category, players: [noPlayer()], clubs: {} });
  /** Per player: a whole name column, or first and last name columns. */
  protected readonly split = signal<boolean[]>([]);
  protected readonly preview = signal<ImportPreview | null>(null);
  protected readonly importing = signal(false);
  /** The choices made for club values matching no listed club. */
  protected readonly clubChoices = signal<Record<string, ClubChoice>>({});
  protected readonly unknownClubs = computed(() => this.preview()?.clubs ?? []);
  private seq = 0;

  protected readonly fileName = computed(() => this.file().path.split(/[\\/]/).pop() ?? '');
  protected readonly lines = computed(() => this.file().lines ?? []);
  protected readonly width = computed(() => Math.max(0, ...this.lines().map((l) => (l.cells ?? []).length)));
  protected readonly columns = computed(() => {
    const head = this.mapping().header ? (this.lines()[0]?.cells ?? []) : [];
    return Array.from({ length: this.width() }, (_, i) => {
      const letter = columnLetter(i);
      return { index: i, letter, label: head[i] ? `${letter} · ${head[i]}` : this.transloco.translate('importMap.column', { letter }) };
    });
  });
  protected readonly sample = computed(() => this.lines().slice(this.mapping().header ? 1 : 0, (this.mapping().header ? 1 : 0) + SAMPLE_LINES));
  protected readonly dataLines = computed(() => this.lines().length - (this.mapping().header ? 1 : 0));
  protected readonly players = computed(() => this.mapping().players ?? []);
  protected readonly slotOf = computed(() => new Map(this.categories().map((c) => [c.category, c.slot])));
  protected readonly shown = computed(() => (this.preview()?.entrants ?? []).slice(0, SAMPLE_ENTRANTS));
  protected readonly more = computed(() => Math.max(0, (this.preview()?.entrants ?? []).length - SAMPLE_ENTRANTS));

  constructor() {
    effect(() => {
      const m = structuredClone(this.file().mapping);
      m.categories ??= {};
      m.players = m.players?.length ? m.players : [noPlayer()];
      untracked(() => {
        this.mapping.set(m);
        this.split.set(m.players!.map((p) => p.name < 0 && (p.first >= 0 || p.last >= 0)));
      });
    });
    effect(() => {
      const m = this.mapping();
      void untracked(() => this.refresh(m));
    });
  }

  private async refresh(m: Mapping): Promise<void> {
    const seq = ++this.seq;
    try {
      const p = await TournamentService.PreviewImport(this.file().path, m);
      if (seq === this.seq) {
        this.preview.set(p);
      }
    } catch (err) {
      this.tournament.error.set(backendError(err));
    }
  }

  private update(change: (m: Mapping) => void): void {
    const m = structuredClone(this.mapping());
    change(m);
    this.mapping.set(m);
  }

  protected setHeader(header: boolean): void {
    this.update((m) => (m.header = header));
  }

  protected setCategoryColumn(i: number): void {
    this.update((m) => {
      m.category = i;
      m.categories = {};
    });
  }

  protected setFallback(c: Category): void {
    this.update((m) => (m.fallback = c));
  }

  /** The value's lines go to category c, or are left out ('' ). */
  protected setValue(value: string, c: Category | ''): void {
    this.update((m) => (m.categories = { ...m.categories, [value]: c as Category }));
  }

  /** What the value's select shows: its category, '' when left out on purpose, '?' when not played here. */
  protected valueChoice(v: { value: string; category: Category }): string {
    if (this.slotOf().has(v.category)) {
      return v.category;
    }
    return this.mapping().categories?.[v.value] === '' ? '' : '?';
  }

  protected setColumn(k: number, field: keyof ImportPlayerColumns, i: number): void {
    this.update((m) => (m.players![k][field] = i));
  }

  protected setSplit(k: number, split: boolean): void {
    this.split.update((s) => s.map((v, j) => (j === k ? split : v)));
    this.update((m) => {
      const p = m.players![k];
      if (split) {
        p.name = NO_COLUMN;
      } else {
        [p.first, p.last] = [NO_COLUMN, NO_COLUMN];
      }
    });
  }

  /** Pairs read a second player; singles one. */
  protected setPairs(pairs: boolean): void {
    this.update((m) => (m.players = pairs ? [m.players![0], noPlayer()] : [m.players![0]]));
    this.split.update((s) => (pairs ? [s[0] ?? false, false] : [s[0] ?? false]));
  }

  protected clubChoice(value: string): ClubChoice {
    return this.clubChoices()[value] ?? defaultClubChoice(value);
  }

  /** The select of a value: "add", "keep" or "map:<club>". */
  protected clubChoiceKey(value: string): string {
    const c = this.clubChoice(value);
    return c.kind === 'map' ? 'map:' + c.club : c.kind;
  }

  protected setClubChoice(value: string, key: string): void {
    const choice: ClubChoice = key.startsWith('map:') ? { kind: 'map', club: key.slice(4) } : key === 'keep' ? { kind: 'keep' } : defaultClubChoice(value);
    this.clubChoices.update((c) => ({ ...c, [value]: choice }));
  }

  protected setNewClub(value: string, field: 'name' | 'acronym', text: string): void {
    const c = this.clubChoice(value);
    if (c.kind === 'add') {
      this.clubChoices.update((all) => ({ ...all, [value]: { ...c, [field]: text } }));
    }
  }

  /** A previewed player's club, as it will be entered. */
  protected previewClub(club: string): string {
    return this.unknownClubs().some((u) => u.value === club) ? chosenClub(club, this.clubChoice(club)) : club;
  }

  protected async confirm(): Promise<void> {
    this.importing.set(true);
    const m = structuredClone(this.mapping());
    const add: Club[] = [];
    m.clubs = {};
    for (const { value } of this.unknownClubs()) {
      const c = this.clubChoice(value);
      if (c.kind === 'keep') {
        continue;
      }
      if (c.kind === 'add') {
        add.push({ name: c.name.trim() || (c.acronym.trim() ? '' : value), acronym: c.acronym.trim() });
      }
      m.clubs[value] = chosenClub(value, c);
    }
    try {
      const report = await TournamentService.ImportEntrants(this.file().path, m, add);
      this.tournament.error.set(null);
      this.done.emit(report);
    } catch (err) {
      this.tournament.error.set(backendError(err));
    } finally {
      this.importing.set(false);
    }
  }

  protected number(event: Event): number {
    return Number((event.target as HTMLSelectElement).value);
  }

  protected value(event: Event): string {
    return (event.target as HTMLSelectElement).value;
  }

  protected checked(event: Event): boolean {
    return (event.target as HTMLInputElement).checked;
  }
}

/** A, B… Z, AA, AB…: the column names of a spreadsheet. */
export function columnLetter(i: number): string {
  let s = '';
  for (let n = i + 1; n > 0; n = Math.floor((n - 1) / 26)) {
    s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
  }
  return s;
}
