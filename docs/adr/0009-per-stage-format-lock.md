# 9. Each stage of a format locks on its own

- Status: accepted, amends ADR 4
- Date: 2026-11-04

## Context

ADR 4 kept a filled stage's kind, size and feeding rules, and the app then
locked the whole format of a category as soon as one of its matches was
called or scored. On the day, that is too strict. Forfeits empty the pools,
or the tournament runs late, and the desk wants to reshape the final bracket
while the pools are still played: one set of 21 instead of two sets of 15,
pools instead of a bracket for a handful of teams, a stage added or removed.
Nothing has happened in those later stages yet, so nothing stands in the way.

## Decision

- **A started stage** (a match called or scored) keeps its kind, size, win
  condition, tiebreaks, cross-pool comparison and full placement
  (`STAGE_STARTED`). Its name, its routing and the estimated duration of its
  matches still change; its routing only as long as the stages it feeds are
  not filled or can be emptied (below). A started stage cannot be removed,
  and the rules feeding it stay.
- **A stage not started** changes freely: win condition, pool count or sizes,
  kind, full placement. Stages are added after the others and any stage not
  started can be removed, whatever else in the category has started.
  `FORMAT_LOCKED` is gone.
- **A later stage filled but not started** keeps its entrants when its win
  condition, tiebreaks or full placement change (matches are derived again
  when needed). When its kind, its size or the rules feeding it change, or
  when it is removed, the change empties it, and the filled stages it feeds
  in turn, in the same `FormatSet` event (`Emptied`): one undo brings it
  back. The desk then fills it again (`FillStage`) from the same sources.
  Emptying rather than refusing keeps the desk moving: refilling is one
  click, and the stage had no result to lose.
- **The entry stage** is filled by the draw: it is never emptied by a format
  change. Its kind and pool count stay refused once drawn (`STAGE_FILLED`,
  `DRAW_CONFIRMED`); the desk reopens the draw instead.
- **Schedule**: matches and blocks are derived from the format, so the
  blocks and matches of a reshaped stage disappear with it. A stored block
  order (`ScheduleBlocksSet`) ignores keys that no longer exist; new blocks
  follow the blocks the desk ordered, in default order. A validated match
  order (`ScheduleOrderSet`) keeps its matches; those of a stage filled
  again join the end, as for any stage filled later.
- **Forfeits** are unchanged: a walkover in the entrant's latest stage and
  every later one.
- The Format screen greys the fields a started stage keeps, with a note, and
  warns on a filled stage that reshaping it empties it. Applying a format
  preset stays refused once the draw is confirmed.

## Consequences

- Older files replay as before: their `FormatSet` events have no `Emptied`
  and never changed a filled stage's shape.
- No schema change.
