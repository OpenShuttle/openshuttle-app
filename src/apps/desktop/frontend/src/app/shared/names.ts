import { SideView } from '@bindings/apps/desktop';

/** "Victor Gauthier / David Michel". */
export function fullNames(side: SideView): string {
  return (side.names ?? []).join(' / ');
}

/** The entrant's clubs, each once: "AS Bron" or "AS Bron / BC Lyon 8". */
export function clubs(side: SideView): string {
  return [...new Set((side.clubs ?? []).filter((c) => c))].join(' / ');
}
