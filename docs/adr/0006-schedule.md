# 6. A schedule drives the desk

- Status: accepted
- Date: 2026-10-19

## Context

The desk proposed the next match in queue order: pool rounds interleaved,
two categories merged by a court priority. Organizers want a forecast of
the day (when each pool, each bracket round plays, when it ends), rules
that match how clubs run a hall, and a way to change the order.

## Decision

- **One pure simulation** (`tournament.State.Schedule(now)`) gives every
  match left a court and estimated times. The desk's proposals are its
  first decisions (what starts now on a free court), the queue follows its
  order, and the Schedule screen shows all of it. Go computes everything.
- **Durations**: each win condition has an `Estimate` (minutes on court,
  changeover included); 0 uses a default from the win condition
  (2 sets of 15: 22 min).
- **Courts take the next match as they free up**, by these rules:
  - hard: a bracket round starts once the previous round of that bracket is
    over; a stage not filled yet starts once the stages feeding it are over
    (its matches are stand-ins: who plays is not known);
  - hard: pinned blocks (pools, bracket rounds) first, in the desk's order;
  - hard, two categories: *alternate* mode plays round 1 of one category,
    round 1 of the other, then round 2…; *reserved* mode keeps each
    category on its courts (set by hand, else shared by matches left) until
    it is over;
  - soft, within a category: the same pool goes on, pools with the most
    rounds left go first, and within a pool the match whose two sides
    rested the longest on average (earliest round on a tie).
- **Rest is never waited for.** When no rested match fits the hard rules,
  the best one is placed anyway and flagged. The desk is warned: it calls
  the match anyway, or changes the order of the blocks.
- The setup (mode, first category, reserved courts, pinned blocks) is one
  `ScheduleSet` event, undoable. The court priority of older files picks
  the category opening each round.

## Consequences

- The forecast and the desk never disagree: they are the same computation.
- Pool matches are picked live (rest decides), so the schedule shows which
  pool plays, and the likely match.
- The simulation runs on every desk refresh; it is linear in matches times
  candidates and stays fast for club tournaments (a few hundred matches).
