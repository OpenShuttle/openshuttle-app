# 13. Court rules replace the reserved mode and ghost matches

- Status: accepted, amends ADR 6 and ADR 12, supersedes ADR 11
- Date: 2026-11-05

## Context

With court rules (ADR 12), an organizer set "women's doubles pools first on
courts 1 and 2", the men's pools on courts 3 to 7, and saw the women play on
courts 6 and 7. The schedule was in *reserved* mode: its automatic share
(courts 1 to 5 for the men, 6 and 7 for the women) was applied before the
rules, so a priority for the other category could never win. The rules say
everything the share said, and more precisely; the two models fought.

Ghost matches (ADR 11) left a court empty in its turn. With rules, the
organizer no longer needs them, and they added a second way to shape the
same timeline.

The same organizer saw the app freeze after setting rules no court could
follow (court 1 never the pools, court 2 only the semi-finals). The
forecast marked a court with nothing left by moving it to a sentinel time
(2038). Another court waiting for "the next court to free up" took that
time as real, and the stuck match was placed in 2038: the timeline then
drew twelve years of half-hour marks.

## Decision

- **No more modes.** The categories always alternate, round by round, the
  chosen category opening each round; court rules alone decide which courts
  take which matches. `SetScheduleMode` is gone. A file in reserved mode
  reads as alternate; its courts reserved by hand still read as an *only*
  rule for their category (ADR 12), and the next schedule change writes
  them as rules.
- **No more ghost matches.** Their commands, view and timeline blocks are
  gone. `GhostMatchAdded` and `GhostMatchRemoved` events still replay
  (ignored) and show in the history, so older files open.
- **A rule set can target several scopes at once**: the desk picks several
  targets for one rule ("priority 1 for pools A and B" on courts 3 to 5),
  and one `ScheduleSet` event adds one rule per target on each court
  (`AddCourtRules`), the same scope given twice kept once, a court's rule of
  a scope already ruled replaced. One undo takes them all back. The timeline
  previews them live (`PreviewCourtRules`).
- **The forecast always ends, at real times.** Idle courts are a flag, not a
  time; time only moves forward to a ready match or a busy court's end.
  When every court is idle with matches left, the matches ready to place
  are ones no court may take: they are freed from the rules and placed
  anyway, spread over the courts (flagged `OffRules`, a court preferring
  what its rules allow), while the other matches keep their rules. A
  property test runs the forecast on thousands of random rule sets with a
  timeout.

## Consequences

- One model for the courts: what was the reserved share is a priority or
  only rule per court, visible in the court rules card.
- Organizers who used ghost matches leave a court out with an *only* rule
  on a later round, or a break.
- Impossible rules show as matches flagged against the rules, never as a
  frozen screen.
