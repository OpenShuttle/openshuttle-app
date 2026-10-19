# 7. The schedule is an order the desk validates, then moves by hand

- Status: accepted, amends ADR 6
- Date: 2026-10-19

## Context

ADR 6 recomputed the schedule live with soft rules (same pool in a row,
rest-based pairing) and let the desk raise whole blocks. Organizers found it
hard to predict: they want to compute the schedule once, freeze it, and then
move single matches or whole pools themselves, seeing how many matches each
team has left.

## Decision

- **Automatic order** (`State.AutoOrder`): per category, stage by stage. In a
  pools stage, wave k holds round k of every pool, the pools with the most
  matches first (pool order on a tie): with pools of 5, 5, 4, 4 (A, B, C, D),
  the order is A, B, C, D round 1, … round 3, then A, B rounds 4 and 5. Each
  pool's round keeps its two matches together. In a bracket, wave k is round
  k. With two categories, their waves alternate (`First` opens).
- **Validation** (`ValidateSchedule`) stores the automatic order of the
  waiting matches in a `ScheduleOrderSet` event. Validating again recomputes
  it and drops the moves.
- **Moves** (`MoveScheduledMatch`, `MoveScheduledPool`) need a validated
  order and store the whole new order in a `ScheduleOrderSet` event:
  undoable, replayed as is. A pool moves as a block, its matches in their
  order.
- Matches that start or get a result leave the order; matches of stages
  filled later join its end in automatic order.
- **The forecast and the desk follow the order strictly**: a free court takes
  the first match of the order whose players are off court (bracket rounds
  and stages not filled yet still wait for what feeds them; reserved courts
  still apply). Rest is never waited for: an unrested match is flagged and
  the desk warned.
- **Durations**: a set match takes `sets × (2/3 × points + 1)` minutes by
  default (2 × 21: 30 min, 2 × 15: 22 min, 1 × 21: 15 min), editable per win
  condition.
- The schedule view lists the matches left per team in each stage.

## Amendment (same day)

- Before validation the desk moves **blocks** (a pool, a bracket round) by
  drag and drop on the timeline (`ScheduleBlocksSet`): within each wave,
  blocks play in that order. After validation, only single matches move.
  "Back to blocks" un-validates (an empty `ScheduleOrderSet`).
- Stages of the same phase (a main bracket and pools for the others) share
  their waves: round 1 of each, then round 2… Stand-ins of stages not filled
  yet follow the same wave order.
- The timeline also shows the matches played (greyed) and playing, at their
  real court and times.

## Consequences

- What the desk sees on the Schedule screen is what the courts will do.
- The soft rules and pinned blocks of ADR 6 are gone; `ScheduleSet.Pinned`
  of older files is ignored.
