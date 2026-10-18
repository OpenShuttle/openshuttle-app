import { SideView } from '@bindings/apps/desktop';

/** "Victor Gauthier / David Michel". */
export function fullNames(side: SideView): string {
  return (side.names ?? []).join(' / ');
}

/** "V. Gauthier / D. Michel": first names as initials, for narrow places. */
export function shortNames(side: SideView): string {
  return (side.names ?? []).map(shortName).join(' / ');
}

function shortName(name: string): string {
  const [first, ...rest] = name.trim().split(/\s+/);
  return rest.length ? `${first[0]}. ${rest.join(' ')}` : first;
}

/** The entrant's clubs, each once: "AS Bron" or "AS Bron / BC Lyon 8". */
export function clubs(side: SideView): string {
  return [...new Set((side.clubs ?? []).filter((c) => c))].join(' / ');
}
