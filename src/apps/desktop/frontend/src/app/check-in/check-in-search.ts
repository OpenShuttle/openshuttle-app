import { CheckInRow } from '@bindings/apps/desktop';

/** Lower case, no accents: "Écuyer" matches "ecu". */
export function fold(s: string): string {
  return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
}

/** The player's own searchable fields: name, club, bib. */
export function ownText(r: CheckInRow): (string | number)[] {
  return [r.name, r.club, r.bib || ''];
}

/** True when every search word appears in the fields. */
export function matches(fields: (string | number | undefined | null)[], words: string[]): boolean {
  const text = fold(fields.filter((f) => f !== undefined && f !== null).join(' '));
  return words.every((word) => text.includes(fold(word)));
}
