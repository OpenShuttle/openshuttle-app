# 8. Before validation, the desk orders rounds in a list

- Status: accepted, amends ADR 7
- Date: 2026-10-19

## Context

ADR 7 let the desk drag a match on the timeline to move its whole pool
before validation, the order then applying within every wave. On the board,
a pool spans every round and several courts: dragging one card moved cards
all over the grid, and organizers could not tell where a pool would land.
They asked for something simpler: put "pool A round 1, pool B round 1…" in
order, as a list.

## Decision

- A **block** is one round of a pool (`MD/S1/A/2`) or one round of a part
  of a bracket: the main draw (`MD/S2/R2`, the semi-finals) or, with full
  placement, the places below (`MD/S2/R2/P5`, places 5 to 8; `MD/S2/R3/P3`,
  the match for third place). `ScheduleBlocks` lists them in the order they
  are played; the matches follow that order (`AutoOrder`).
- **Stages not filled yet** are in the list too, from their stand-ins
  (players to come): the desk orders a bracket's rounds before the pools
  feeding it are over, and the order holds once it is filled.
- **Default order**: unchanged, wave by wave (round k of every pool and
  bracket of a phase, categories side by side, the category opening first,
  then the pools with the most matches).
- Before validation the desk reorders blocks in a **list** beside the
  timeline, by drag and drop (`MoveScheduledBlock`). The timeline is read
  only and shows the result live. `ScheduleBlocksSet` stores the whole
  order; blocks added later (a stage filled since) follow in default order.
- **The rounds of a pool, and of a bracket, keep their order**: after a
  move, the places a pool's rounds hold go to its rounds, first round first.
  Nothing is refused, the list shows what it gives.
- **Older files** stored pools (`MD/S1/A`): with no round in the stored
  order, it still ranks the pools within each wave, as before.
- After validation nothing changes: matches move one by one on the timeline.

## Consequences

- The list says in which order rounds are played; the timeline only shows
  where and when.
- A round can be pulled forward on its own (a pool that must finish early),
  which waves did not allow.
