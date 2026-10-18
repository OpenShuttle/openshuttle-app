# 4. Flexible format: stages of any kind, chained by routing

- Status: accepted
- Date: 2026-10-18

## Context

The first format was fixed: one pool stage, then knockout brackets fed by the
pool ranks. Clubs want more: a bracket from the start, pools after pools,
three phases or more, a consolation bracket fed by the losers of the main one.
Formats must also be saved as presets and reused from one tournament to the
next, together with win conditions and tiebreak orders.

No real tournament has been played yet: files from the first format do not
need to stay readable.

## Decision

- A category's **format is an ordered list of stages**. A stage has an ID
  (`S1`, `S2`… never reused in a category), a name, a kind (`pools` or
  `bracket`), a win condition, and its kind's settings: number of pools and
  tiebreak order, or full placement.
- **The first stage is the entry stage**: the draw fills it with the present
  entrants (pools drawn as before, or bracket slots: seeds first, the others
  at random). It is a draft until the draw is confirmed.
- **Routing** links stages. Each stage maps its ranks (pool position, or
  bracket place) to a *later* stage of the list, or keeps them (final rank
  there). Routing only points forward, so the format is always acyclic and the
  list order is the play order. A stage's phase is 1 + the highest phase of
  the stages feeding it.
- A later stage is **filled** once every stage feeding it is complete (ties
  broken): pools by a snake over ranks that keeps entrants from the same group
  apart, brackets with the existing seeding. Filling records the result as an
  event (`StageFilled`); it can be redone until a match of the stage starts.
- The whole format is one event, `FormatSet`, sent on every change: commands
  (`SetFormat`, `AddStage`, `RemoveStage`, applying a preset) all validate the
  full list. A stage that is filled cannot change kind or size, nor be
  removed, and routing into it cannot change.
- **Full standings** follow the routing from the entry stage: rank by rank,
  a rank routed to another stage expands that stage's standings (once), a
  rank kept is the entrants at that rank, tied across groups.
- **Presets** (win conditions, tiebreak orders, whole formats) live in app.db,
  are shared by every tournament, and are copied into a tournament when
  applied: a tournament file stays self-contained.
- The tournament file schema moves to version 2: files written by the first
  format are refused (`FILE_TOO_OLD`).

## Consequences

- Any chain of pools and brackets is possible, including a bracket in phase 1
  and pools in phase 2 or 3.
- Double elimination and Swiss rounds can be added later as new stage kinds,
  with their own match generation and standings, without changing routing.
- Event types `PoolStageConfigured`, `BracketAdded`, `BracketConfigured`,
  `BracketRemoved`, `RoutingSet` and `BracketDrawn` are retired.
