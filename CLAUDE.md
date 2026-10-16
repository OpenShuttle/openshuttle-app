# CLAUDE.md

**OpenShuttle** (openshuttle.app): open source desktop application to run non-sanctioned badminton club tournaments, fully offline.
The functional spec lives in `docs/specs/cadrage-v1.md` (French). Read it before any feature work.

## Scope

- **v1**: desktop app, 100 % local. One control desk PC + a second window on the hall display. No network, no printing. Up to two match categories per tournament, run at the same time on shared courts (see Multi-category). UI in English and French.
- **v2 (not now)**: optional one-way publication to a self-hostable relay server (Go) that also serves a mobile-first Angular website for players and spectators (no native mobile app). Never make the desktop depend on the network. Do not build v2 code unless asked.

## Stack

- **Desktop shell**: Wails v3 (beta, pin the exact version in `go.mod`)
- **Backend**: Go, single module in `src/`
- **Frontend**: Angular, in `src/apps/desktop/frontend`
- **i18n**: Transloco, runtime language switch, `en` and `fr`
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)

## Repository layout

All source code lives under `src/`. The repo root only holds docs, CI and project files.

```
CLAUDE.md, README.md, LICENSE
docs/specs/              functional spec
docs/adr/                architecture decision records
.github/workflows/       CI
src/go.mod               single Go module for all Go code
src/internal/engine/     pure domain logic: formats, stages, standings, tiebreaks, advancement, scheduling
src/internal/events/     event log: event types, append, replay, undo
src/internal/store/      SQLite access, schema migrations, backups
src/apps/desktop/        Wails app: main.go, services (bound to the frontend), windows
src/apps/desktop/frontend/  Angular app (main window + display window routes)
src/apps/relay/          v2 relay server, empty for now (will serve src/apps/web via go:embed)
src/apps/web/            v2 mobile-first Angular website, empty for now
```

## Architecture rules

- **Go is the source of truth.** All business logic lives in Go. The frontend never computes standings, advancement, schedules or score validity.
- **`src/internal/engine` is pure**: no I/O, no SQLite, no Wails imports, no clock reads (time is passed in). Fully unit-tested.
- **Event-sourced tournament state.** Every change is a command → validated → appended as an event → projections updated. Undo appends a compensating event or rebuilds from the log. History must show what changed and when.
- **Commands and queries are separate.**
    - Commands express intent (`RecordScore`, `DeclareWalkover`, `CallMatch`, `EndMatch`, `StartRound`, `EndRound`, `CheckInPlayer`, `MoveEntrant`, `SwapMatches`, `Undo`) and return OK or an error code.
    - Queries return ready-to-render views, one per screen (`GetControlDeskView`, `GetPoolStandings`, `GetBracket`, `GetDisplayView`).
    - After each event, Go emits a Wails event `tournament.updated` with a version number. Each window refetches the views it shows.
- **No optimistic updates** in the frontend. Calls are in-process and fast, wait for the result.
- **Go never returns human-readable text.** Errors are stable codes (`INVALID_SCORE`, `COURT_BUSY`). The frontend translates them.
- **Timed matches**: Go stores the round start time (`RoundStarted` event) and duration. The frontend only renders the countdown.
- **Never block the user.** Inconsistencies produce warnings, not refusals, unless the action is impossible.

## Data

- `app.db`: player and club registry, presets, app settings.
- One SQLite file per tournament, extension `.openshuttle`: event log, entrants, stages, matches.
- No player ranking or level management (no FFBaD classification such as P10, NC). Entrants have no level field.
- A tournament file is self-contained: entrants are copied from the registry at creation.
- Every file stores its schema version and is migrated on open.
- WAL mode and `synchronous=FULL` always: a committed action survives a power cut. Automatic periodic backup copy (`VACUUM INTO`, named `<name>.backup-YYYYMMDD-HHMMSS.openshuttle`, rotated).
- Files are recognized by their SQLite `application_id` (`OSHT` tournament, `OSAP` app.db), versioned by `user_version`. Migrations are append-only lists in `internal/store`.
- Undo appends an `ActionUndone` event targeting an earlier seq; undoing an undo restores its target. `events.Active` gives the events in effect. Nothing is deleted from the log.
- Open SQLite with a plain path, never a `file:` URI (Windows paths, `#` and `%` in names).

## Domain rules (v1, decided)

- **Walkover in a pool**: all matches of the withdrawn entrant become walkovers, played or not. The win counts for the opponent, but these matches are excluded from set and point calculations. The withdrawn entrant ranks last in the pool.
- **Walkover in a bracket**: counts as a loss, the opponent advances.
- **Pool tiebreaks**, in a per-stage configurable order: wins, head-to-head, set difference, point difference, random draw. Always differences, never ratios.
- **Timed match tie**: decided by a golden point.
- **Head-to-head** only applies between exactly two tied entrants. When a criterion splits a tied group, each subgroup is ranked again from the first criterion (so a pair left from a 3-way tie can be decided by head-to-head).
- **Random draw**: the engine never draws. Lots are recorded as an event; until then, entrants still tied in a complete pool are flagged `DrawNeeded` and share their rank.
- **Set scores**: a set is won at `PointsPerSet` with a 2-point lead, deuce up to `Cap` (at `Cap-1` all, the next point wins). `Cap = 0`: no cap. `Cap = PointsPerSet`: no deuce.
- **Timed score**: one set holding the final points. A golden point is counted in the score, so it is a 1-point win.

## Match lifecycle (v1, decided)

- **A match ends when the control desk says so.** Nothing ends automatically, not even a timed match at 0:00.
- **Court states**: `Free` → `Playing` → `Finished` (ended by the desk, score not entered yet) → `Free`.
- **A finished court can start its next match before the score is entered.** The previous match moves to the *pending scores* list (`ScorePending`), tied to its court and end time. Scores can be entered later, in any order.
- **Rest time** counts from the real end of the match (`MatchEnded`), not from score entry.
- **Set-based stages**: each court runs on its own. The desk ends a match (`EndMatch`), then calls the next one or enters the score.
- **Timed stages run in rounds.** All courts start and end together: one round = one match per court, one shared countdown.
    - The desk starts the round (`StartRound`): every proposed match is called at once.
    - At 0:00 the desk sees "time elapsed" and ends the round (`EndRound`), which can also be done early. All matches of the round go to pending scores.
    - Round composition: an entrant never plays twice in a round. Entrants who sat out the previous round get priority. The minimum rest rule does not apply between rounds.
- **Timed stages are only allowed when a single category is playing.** With two categories, the format editor refuses `WinCondition` = timed (warning, not a hidden option).

## Multi-category (v1, decided)

- **At most two categories** per tournament, e.g. men's doubles + women's doubles.
- **Categories are disjoint**: a player cannot be entered in both. No cross-category rest or conflict handling.
- **One independent format per category**: its own stages, win conditions, advancement rules, full standings.
- **Courts are shared**, with a per-tournament court priority: `Balanced` (the least advanced category goes first, by share of matches played), or one category first. Priority only reorders proposals, it never reserves a court.
- **Bib numbers are unique across the tournament.** Pool names stay A, B, C… within each category.
- **Check-in is per player**: each player is checked in once. A pair is complete when both players are present. The bib is assigned at the first check-in of the pair.
- **The draw is per category.** Moving an entrant to a pool of the other category is refused (impossible action, not a warning).
- **UI**: every match, court, queue item and call shows a colored category badge (English: `MS`, `WS`, `MD`, `WD`, `XD`; French: `SH`, `SD`, `DH`, `DD`, `DM`). The control desk is a single view with a category filter on the queue. The display screen mixes both categories, with a very visible badge.
- **Full standings** require full-placement brackets. Otherwise, entrants losing in the same round are tied.

## Glossary (French spec → code)

| French | Code |
| --- | --- |
| Tournoi | Tournament |
| Catégorie | Category |
| Participant (joueur ou paire) | Entrant |
| Dossard | BibNumber |
| Format | Format |
| Preset | Preset |
| Phase | Stage |
| Poule | Pool |
| Tableau | Bracket |
| Tableau consolante | ConsolationBracket |
| Routage | AdvancementRule |
| Condition de victoire | WinCondition |
| Placement complet | FullPlacement |
| Point en or | GoldenPoint |
| Forfait / WO | Walkover |
| Pointage | CheckIn |
| Tirage | Draw |
| Tête de série | Seed |
| Table de marque | ControlDesk |
| Appel | MatchCall |
| Écran d'appel | DisplayScreen |
| Terrain | Court |
| Rotation | Round |
| Score en attente | PendingScore |
| Priorité des terrains | CourtPriority |

Use these names everywhere: types, functions, events, i18n keys, UI labels.

## Conventions

- All code, comments, commit messages, issues and docs in English (the functional spec stays in French).
- i18n keys in English, namespaced by screen: `controlDesk.callMatch`, `checkIn.markPresent`. ICU plurals. No hardcoded UI strings.
- French labels run 15 to 20 % longer than English: layouts must not break.
- Commits: Conventional Commits (`feat(engine): ...`, `fix(desktop): ...`).
- Every engine change comes with tests. Prefer table-driven tests.
- Record significant architecture decisions as ADRs in `docs/adr/`.

## Commands

Prerequisites: Go (version in `src/go.mod`), Node 24, the Wails CLI pinned to the
same version as `src/go.mod`:
`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.30`.
On Linux: `libgtk-4-dev libwebkitgtk-6.0-dev libsoup-3.0-dev`.

```
cd src/apps/desktop && wails3 dev             # run the desktop app with hot reload
cd src/apps/desktop && wails3 build           # production binary in src/apps/desktop/bin
cd src/apps/desktop && wails3 generate bindings -clean=true -ts -i   # after changing a Go service
cd src && go test ./...                       # all Go tests (build the frontend first: the desktop package embeds frontend/dist)
cd src/apps/desktop/frontend && npm test      # frontend unit tests (Vitest)
cd src/apps/desktop/frontend && npm run i18n:check   # en.json and fr.json must share the same keys
```

Generated bindings (`frontend/bindings`) are committed. Import them through the
`@bindings/*` path alias.

## Design

The Claude Design mockup lives in `docs/tournoi-badminton-interactif/project/`
(`Tournoi Badminton v2.dc.html` is the reference). Recreate it, do not copy its
structure. Colors are CSS custom properties in `frontend/src/styles.scss`, taken
from the mockup. Category badges are colored by the category's slot in the
tournament (`--color-category-1`, `--color-category-2`), not by type.
