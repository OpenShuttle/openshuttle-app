# 2. Event-sourced tournament state

- Status: accepted
- Date: 2026-10-16

## Context

The control desk must be able to fix the tournament live: move an entrant,
change or clear a score, swap two matches, declare a walkover. Every action
must be undoable, and the history must show who changed what and when. A crash
or a power cut must never lose more than the last action.

## Decision

- Tournament state is **event-sourced**. Every change is a command, validated,
  then appended as an event to the tournament file. Projections (standings,
  brackets, court states, queues) are derived from the log.
- **Commands and queries are separate.** Commands express intent
  (`RecordScore`, `CallMatch`, `EndMatch`, `StartRound`, `Undo`…) and return OK
  or a stable error code. Queries return one ready-to-render view per screen.
- **Undo** appends a compensating event, or rebuilds projections from the log
  without the undone event. Nothing is deleted from the log.
- Each event records the operator name (set in Settings) and the time.
- After each event Go emits the Wails event `tournament.updated` with the new
  version. Each window refetches the views it shows. The frontend does no
  optimistic updates.
- Each tournament is one SQLite file with the `.openshuttle` extension, in WAL
  mode, with its schema version, migrated on open, and periodically backed up.
- The domain logic in `src/internal/engine` is pure: no I/O, no clock reads,
  no SQLite or Wails imports.

## Consequences

- The engine is fully unit-testable from plain values.
- History, undo and crash recovery come from the same log.
- Projections must be rebuildable from scratch at any time, so event types are
  versioned and never change meaning once released.
