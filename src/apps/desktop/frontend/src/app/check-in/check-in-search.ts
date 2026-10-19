import { CheckInTeam, PlayerView } from '@bindings/apps/desktop';

/** Lower case, no accents: "Écuyer" matches "ecu". */
export function fold(s: string): string {
  return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
}

/** A player's own searchable fields: name, club, and the team's bib. */
export function ownText(p: PlayerView, team: CheckInTeam): (string | number)[] {
  return [p.name, p.club, team.bib || ''];
}

/** Every searchable field of a team: its players, their clubs, its bib. */
export function teamText(team: CheckInTeam): (string | number)[] {
  return [...(team.players ?? []).flatMap((p) => [p.name, p.club]), team.bib || ''];
}

/** True when every search word appears in the fields. */
export function matches(fields: (string | number | undefined | null)[], words: string[]): boolean {
  const text = fold(fields.filter((f) => f !== undefined && f !== null).join(' '));
  return words.every((word) => text.includes(fold(word)));
}
