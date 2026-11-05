# 11. Ghost matches keep a court empty in its turn

- Status: superseded by ADR 13 (court rules replaced ghost matches), amended ADR 10
- Date: 2026-11-05

## Context

Breaks (ADR 10) stop every court at fixed hours. Organizers also know that
one court will be empty for a while: a court lent to a warm-up, a net to
repair, a referee course. They want the schedule to show it, so the
forecast times of the other matches hold. They see it as a match: it comes
in its turn after the matches before it, and slides with them when the day
runs late.

## Decision

- **A ghost match** (`GhostMatchAdded {court, minutes, after}`, ID
  `g<seq>`; `GhostMatchRemoved`) leaves one court empty for some minutes
  (1 to 600), right after a match of the order (`after`), or as soon as the
  court is free (no `after`). One court per ghost match: two courts take
  two. Both events are undoable and in the history.
- **In the forecast** (`forecast.Item.Ghost`, `Court`, `AfterMatch`), it is
  an item of the order only its court may take, ignoring reserved courts,
  ready once the match it follows has ended (forecast, then real end). It
  is inserted right after that match in the items, whether the order is
  made of blocks or validated, so it follows the match wherever it moves.
  It does not count in the forecast's end.
- **Only the Schedule screen sees it** (`State.ScheduleWithGhosts`). The
  control desk's queue and proposals (`State.Schedule`, `Proposals`) leave
  it out: the desk keeps calling matches as before.
- **It is over** once the desk calls a match on its court after it was due:
  after the match it follows ended, or after it was added when it follows
  no match (or a match no longer to play). Then it leaves the schedule; the
  file keeps it. A court removed drops it too.
- A ghost match at fixed hours on one court (a per-court break) was
  considered: it does not follow the day when it runs late. Rejected.

## Consequences

- Older files have no ghost match: nothing changes for them.
- The desk sees no warning when it calls a match on a court a ghost match
  holds: the forecast simply moves on.
