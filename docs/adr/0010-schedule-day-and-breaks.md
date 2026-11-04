# 10. The schedule starts at the tournament's start time and stops for breaks

- Status: accepted, amends ADR 6
- Date: 2026-11-04

## Context

The forecast (ADR 6) always started now. Organizers prepare the schedule the
day before or in the morning, and want it to read "9:00, 9:30…", not the
time they happen to look at it. They also stop every court for lunch, and
want the schedule to show it.

## Decision

- **Date and hours**: `TournamentDaySet` stores a date (`2026-11-14`), a
  start and an end hour (`09:00`, local time), each optional. It is undoable
  and in the history. Without a date, the hours are on the current day.
- **Forecast start** (`State.ForecastStart`): while no match has been
  called, the forecast starts at the start hour, or now once it is past. As
  soon as a match is called, it starts now: real times take over, as before.
  Proposals ignore the start hour (the desk may start early).
- **End hour**: a forecast ending after it is flagged (`ScheduleView.Late`),
  a warning, never a refusal.
- **Breaks** are periods at fixed hours (`BreakAdded {from, to}`, ID
  `k<seq>`; `BreakRemoved`), for every court. They are independent of the
  order of rounds, so they hold before and after validation. In the
  forecast (`forecast.Input.Breaks`) a court never starts a match that would
  run into a break: it waits for its end. A shorter match is not looked for.
  Matches are not proposed during a break.
- Breaks inserted in the order with a duration (wait for every round before,
  then pause) were considered: they idle courts and depend on the order, which
  changes at validation. Rejected for now.

## Consequences

- Older files have no day and no break: the forecast starts now, as before.
- A tournament over two days is not supported: the breaks and hours are on
  one day.
