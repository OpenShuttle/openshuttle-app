# 5. Pool sizes by preference, routing rules and editable presets

- Status: accepted
- Date: 2026-10-18

## Context

A format preset must not depend on a number of entrants: the organizer picks
the format on the day, once they know who came. With 22 teams and pools of 4
or 5, the pools are 5, 5, 4, 4, 4; a 16-team main bracket then takes the top
three of each pool plus the best fourth, compared across pools of different
sizes. Routing by rank alone (ADR 4) cannot express that.

Presets must also be edited outside a tournament, with a simulation for a
given number of teams, and the built-in ones must stay as shipped.

## Decision

- **Pool sizes by preference.** A pools stage holds either a fixed number of
  pools (`Pools`) or a priority list of sizes (`PoolSizes`, e.g. 4, then 3,
  then 6 as a last resort). `engine.PlanPools` picks the split that uses the
  fewest pools of the last size, then of the one before, and so on; larger
  pools come first. No split for that number of entrants is `NO_POOL_SPLIT`
  (a warning in the preview, a refusal at the draw).
- **Routing rules** replace the per-rank list. Each rule sends entrants to a
  later stage: `ranks` (every entrant at the given ranks), `bestOfRank` (the
  N best at one rank), `best` (the N best not routed yet), `rest` (everyone
  else). Rules apply in order to the stage's entrants ranked rank by rank,
  then across pools; entrants no rule takes keep their final rank there.
- **Comparing across pools** is a per-stage setting (`CrossPool`): `raw`
  results, or `sameMatches`, which leaves out matches against entrants ranked
  below the smallest pool size so everyone compares on as many matches. The
  key is wins, then set difference, then point difference.
- **Previews are pure**: `tournament.PreviewFormat` simulates a format for N
  stand-in entrants (sizes, matches, rounds, places, entrants per rule). The
  tournament Format screen and the preset editor share it, and the format
  editor component.
- **Presets are edited in the app, outside any tournament**: formats in their
  own editor with a team-count simulator, win conditions and tiebreak orders
  inline. Built-in presets (`b-…`) are read-only (`PRESET_READ_ONLY`):
  duplicate them to change them.
- The tournament file schema moves to version 3: files holding the per-rank
  routing are refused (`FILE_TOO_OLD`).

## Consequences

- One preset serves any number of teams; the number of pools follows.
- "Top 3 of every pool and the best fourth" is two rules (`ranks 1-3`,
  `bestOfRank 4 × 1`), or one (`best 16`), the rest going to a consolation.
- Routing into a stage changes its size with the number of entrants: a
  filled stage only keeps the rules feeding it, not a fixed entrant count.
