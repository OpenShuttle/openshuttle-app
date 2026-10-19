# CLAUDE.md

**OpenShuttle** (openshuttle.app): open source desktop application to run non-sanctioned badminton club tournaments, fully offline.
The functional spec lives in `docs/specs/scope-v1.md`. Read it before any feature work. Contributors follow `CONTRIBUTING.md`.

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
    - Codes are `engine.Code` values wrapped in `*engine.Error`. The app-level `MarshalError` sends them as the call's `cause` (`{code, set?}`); anything else becomes `INTERNAL` and is logged. The frontend reads it with `backendError()` and shows `errors.<CODE>`.
- **Desktop layering**: `internal/tournament` (pure: event types, projection, command validation) → `apps/desktop` services (load log, validate, append, emit `tournament.updated`). Services take `emit`, `now` and dialogs as dependencies so they are tested without Wails.
- **Settings** live in app.db and are broadcast with `settings.changed`; both windows apply them (language included).
- **Navigation**: going home keeps the tournament open (the top bar offers to resume it); opening another file replaces it. The home screen lists recent files with a summary saved in app.db when a file is opened or closed (`TournamentSummary`), live for the open one.
- **Duplicate** (`tournament.Duplicate`) starts a new file with the setup of another: courts, rest, court priority, categories, pool stages, brackets and routing (bracket IDs renumbered). Never entrants, draws or results.
- **Timed matches**: Go stores the round start time (`RoundStarted` event) and duration. The frontend only renders the countdown.
- **Never block the user.** Inconsistencies produce warnings, not refusals, unless the action is impossible.

## Data

- `app.db`: player and club registry, presets, app settings.
- One SQLite file per tournament, extension `.openshuttle`: event log, entrants, stages, matches. Schema version 3 refuses files from earlier formats (`FILE_TOO_OLD`).
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
- **Two categories never share a sex**: one men's (MS or MD) and one women's (WS or WD); mixed doubles (XD) always runs alone (`CATEGORIES_SHARE_SEX`). Go lists the categories still allowed (`AvailableCategories`).
- **One independent format per category**: its own stages, win conditions, advancement rules, full standings.
- **Courts are shared**. The schedule (ADR 6) decides who plays where: *alternate* mode (round 1 of each category in turn, then round 2…, `First` opens) or *reserved* courts per category until it is over.
- **Bib numbers are unique across the tournament.** Pool names stay A, B, C… within each category.
- **Check-in is per player**: each player is checked in once. A pair is complete when both players are present. The bib is assigned at the first check-in of the pair. The check-in screen groups a pair's players; `CheckInEntrant` checks in every absent player of an entrant in one `EntrantCheckedIn` event (one undo).
- **The draw is per category.** Moving an entrant to a pool of the other category is refused (impossible action, not a warning).
- **UI**: every match, court, queue item and call shows a colored category badge (English: `MS`, `WS`, `MD`, `WD`, `XD`; French: `SH`, `SD`, `DH`, `DD`, `DM`). The control desk is a single view with a category filter on the queue. The display screen mixes both categories, with a very visible badge.
- **Full standings** require full-placement brackets. Otherwise, entrants losing in the same round are tied.
- **Players live inside their entrant**: categories are disjoint, so each player belongs to exactly one entrant. A player has one `Name` (first and last name typed together; events with `firstName`/`lastName` are joined on read) and a club. Screens always show full names. Same name twice is a warning (homonyms exist), never a refusal.
- **Player list import/export**: CSV with columns category (English or French short code), player 1, club 1, player 2, club 2. The frontend builds the export; Go parses the import (`;` or `,`, BOM, header line) and enters every valid row in one `EntrantsImported` event (IDs `e<seq>.<n>`, `p<seq>.<n>.<k>`), so one undo cancels it. Invalid or already entered rows are skipped and reported by line.
- **Clubs**: app.db keeps a club list (`Presets.Clubs`, managed on the Presets screen). The entry form picks a club from it or adds a new one; a club typed for a player joins the list.
- **IDs come from the event seq** (`e12`, `p12.1`): deterministic on replay and never reused, even after an undo of an undo.
- **Bibs** are recorded in the `PlayerCheckedIn` event and never reused: a removed entrant or a cancelled check-in keeps its number out of circulation. A cancelled check-in keeps the entrant's bib.
- **Removing a category with entrants is refused** (`CATEGORY_NOT_EMPTY`): it would silently drop them.
- **Format** (ADR 4, 5): per category, an ordered list of stages (`S1`, `S2`… never reused), each `pools` (a fixed number of pools or pool sizes by preference, tiebreak order, cross-pool comparison) or `bracket` (single elimination, full placement or not), with its win condition. The first stage is the entry stage, filled by the draw. Routing rules (`ranks`, `bestOfRank`, `best`, `rest`) apply in order and send entrants to a *later* stage; the others keep their final rank there, so the format is acyclic and phases follow. A bracket is final: it routes nobody on, and a new stage needs a pools stage to feed it (`BRACKET_IS_FINAL`). The whole format is one `FormatSet` event. A filled stage keeps its kind, its size and the rules feeding it (`STAGE_FILLED`). Once a match of the category is called or scored, its format is locked (`FORMAT_LOCKED`).
- **Pool sizes**: `PoolSizes` is a priority list (4, then 3, then 6 as a last resort); `engine.PlanPools` uses the fewest pools of the least wanted sizes, larger pools first, and `NO_POOL_SPLIT` when none fits. A fixed `Pools` count splits as evenly as possible. Entrants of one rank are compared across pools by wins, set difference, point difference; `sameMatches` first drops matches against entrants ranked below the smallest pool size.
- **Previews**: `tournament.PreviewFormat` simulates a format for N entrants; the Format screen and the preset editor share it and the `app-format-editor` component.
- **Presets** (win conditions, tiebreak orders, whole formats) live in app.db, shared by every tournament; edited on the Presets screen, outside any tournament (formats with a team-count simulator). Built-in ones (`b-…`) are read-only (`PRESET_READ_ONLY`): duplicate them to change them. Any preset can be disabled (`Presets.Disabled`): it stays on the Presets screen but is not offered in tournaments. The settings choose the default presets: the format a new category starts with (stored in its `CategoryAdded` event, the engine default when missing or invalid), and the win condition and tiebreak order of new stages and formats. Built-in presets play 2 sets of 15, cap 21. Applying a format preset copies it into the tournament (stage IDs renumbered from S1), only before the draw is confirmed.
- **Draw**: only present entrants (all players checked in). Seeds are a ranking set in one `SeedsSet` event (top seed first, the others unseeded), edited by drag and drop or arrows; they are placed in a snake (A, B, C, C, B, A…); the others in random order, each into the pool with the fewest same-club players. Go seeds the random generator; the `PoolsDrawn` event stores the resulting pools, never the seed. Drawing is refused only below two present entrants per pool or when no pool split fits; imbalance, same club and two seeds in a pool are warnings. A confirmed draw locks moves and the number of pools until reopened.
- **Drag and drop** uses pointer events, not HTML5 drag and drop (unreliable across WebKitGTK, WebView2 and WKWebView).
- **Pool matches are derived, not stored**: confirming a draw generates the round-robin matches (circle method, rounds interleaved across pools, IDs like `MD-A3`). Events record what happens to them: `MatchCalled`, `MatchEnded`, `ScoreRecorded`, `EntrantWithdrawn`. Event times are the call and end times. Once a match has started, the draw cannot be reopened and entrants cannot be removed (declare a forfeit).
- **Calling is refused only when impossible**: a match already in progress on that court, or an entrant already on another court. Resting is shown in the queue, never enforced: any waiting match of the queue can be called on a court with no match in progress ("Force" when an entrant rests or is absent).
- **Schedule** (ADR 6, 7): matches go in waves per category, phase after phase: wave k = round k of every pool and every bracket of the phase (stages of one phase play side by side), categories side by side. Before validation the desk moves blocks (`MoveScheduledBlock`, `ScheduleBlocksSet`: pools `MD/S1/A`, bracket rounds `MD/S2/R1`); within each wave blocks follow that order (default: the category opening, then pools with the most matches). `ValidateSchedule` freezes the match order (`ScheduleOrderSet`); then only single matches move (`MoveScheduledMatch`); `UnvalidateSchedule` (empty `ScheduleOrderSet`) goes back to blocks. Matches of stages filled later join the end. `State.Schedule(now)` forecasts courts and times: a free court takes the first match of the order whose players are off court (bracket rounds wait for the previous one, unfilled stages for their sources as stand-ins in wave order, reserved courts apply). Durations come from each win condition's `Estimate` (0: `sets × (2/3 × points + 1)` min). Rest is never waited for: unrested matches are flagged and the desk warned. The Schedule screen shows played matches greyed at their real court and times, and the teams' matches left (`MatchesLeft`); bracket rounds are named by size (eighth, quarter, semi, final).
- **Side panels** (tournament menu, right panels of Info, Schedule, Stages) fold; the state is a per-PC convenience in local storage (`Panels`).
- **Proposals** are the schedule's matches starting now on free or finished courts, never with a player on court. The queue follows the schedule order. The view depends on the time, so the control desk refetches every 15 s.
- **Corrections** (Stages screen, correction mode): edit or clear a score, declare or cancel a forfeit, swap two waiting matches, draw lots for a complete pool still tied after every other criterion (`LotsDrawn` stores the lots). A forfeit takes a second click and stays out of the Tab order of the score panel: Enter there always means "save the score".
- **Later stages** are filled (`StageFilled`) once every stage feeding them is complete with no tie left: pools by a snake over ranks keeping entrants of the same group apart, brackets with the seeding below (byes as `""`). Refilling is possible until a match of the stage starts. A bracket as entry stage is drawn with seeds on their standard slots, the others at random, then first-round club mates pulled apart. Seeding: pool rank, then pool; within a rank, each entrant takes the slot where it meets its pool mates as late as possible (never in round 1 when avoidable, winner and runner-up in opposite halves); byes go to the top seeds. Bracket matches are derived per sub-bracket (Start, Size): winners go on to places Start..Start+Size/2-1, losers (full placement only) to the rest. Without full placement, losers of a round share their places. Match IDs: `MD-A3` in the entry stage's pools, `MD-S2-A3` in a later pools stage, `MD-S2-5` in a bracket stage.
- **Forfeit scope**: a forfeit is recorded with the latest stage the entrant plays; every match of that stage and the later ones becomes a walkover, earlier stages keep their results.
- **Bracket corrections**: a result whose winner already played the next round cannot be flipped (`BRACKET_NEXT_STARTED`); a bracket cannot be refilled or have its slots swapped once a match started.
- **Full standings** (Results): follow the routing from the entry stage, rank by rank; a routed entrant brings its destination stage's standings (once), the entrants kept at a rank are tied across pools (byes compacted). Ranks stay 0 ("–") until decided; the category is "final" once every rank is.
- **Exports**: the frontend builds the CSV (it holds the translated headers; `;` in French, `,` in English) and Go writes it with a UTF-8 BOM through a save dialog. **Archiving** appends `TournamentArchived` (undoable), writes a copy of the file where the user chooses, then closes it.
- **Rounds** (timed matches): `RoundStarted` stores the composition (match → court) and the minutes; its event time starts the shared countdown. Composition: waiting timed matches in queue order, one per free court, nobody twice, entrants who sat out the previous round first; no minimum rest. `RoundEnded` ends every match of the round still on court. Timed matches are never proposed per court. The frontend renders the countdown from `endsAt`; at 0:00 it shows "time elapsed" and waits for the desk to end the round.
- **No keyboard shortcuts**: every action is a button. Enter and Escape keep their usual meaning inside forms.
- **Score JSON** is `{sets: [{a, b}], goldenPoint}`, win conditions `{kind, setsToWin, pointsPerSet, cap, minutes}`: these are stored in events, never rename them.

## Glossary (French UI → code)

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
| Priorité des terrains | CourtPriority (older files; now `ScheduleSet.First`) |
| Échéancier | Schedule |
| Terrains réservés | ScheduleReserved |

Use these names everywhere: types, functions, events, i18n keys, UI labels.

## Conventions

- All code, comments, commit messages, issues and docs in English.
- i18n keys in English, namespaced by screen: `controlDesk.callMatch`, `checkIn.markPresent`. ICU plurals. No hardcoded UI strings.
- French labels run 15 to 20 % longer than English: layouts must not break.
- Commits: Conventional Commits (`feat(engine): ...`, `fix(desktop): ...`), signed off for the DCO (`git commit -s`). License: AGPL-3.0-or-later.
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
`@bindings/*` path alias (`@bindings/apps/desktop`, `@bindings/internal/engine`…).

## Versions and releases

- Every app is versioned `MAJOR.MINOR.PATCH`: MAJOR is the compatibility generation shared by all apps (`version.Compat` in `src/internal/version`), MINOR the app's own version, PATCH security fixes and dependency updates only. See ADR 3 and `docs/releasing.md`.
- A release is a tag on a commit of `main`: `desktop-vX.Y.Z` (later `relay-v…`, `web-v…`). `.github/workflows/release-desktop.yml` checks it, stamps the version (`src/apps/desktop/build/set-version.sh`) and publishes the GitHub release. Never edit versions by hand in `main`: `version.txt` stays `dev`.
- Bump `version.Compat` only when tournament files or the relay protocol break compatibility.
- Renovate (`renovate.json`) keeps dependencies up to date. Keep the Wails Go module, `@wailsio/runtime` and the `WAILS_VERSION` of the release workflow on the same version (Renovate groups them).

## Design

The Claude Design mockup lives in `docs/tournoi-badminton-interactif/project/`
(`Tournoi Badminton v2.dc.html` is the reference). Recreate it, do not copy its
structure. Colors are CSS custom properties in `frontend/src/styles.scss`, taken
from the mockup. Category badges are colored by the category's slot in the
tournament (`--color-category-1`, `--color-category-2`), not by type.
The app logo is `frontend/public/logo.svg` (the shuttle alone, transparent; also `favicon.ico`); the app
icons in `build/` (`appicon.png`, `windows/icon.ico`, `darwin/icons.icns`,
DMG icon and background) are generated from it (`wails3 generate icons`).
The main window has no native title bar on Windows and Linux (`Frameless`):
the app's top bar drags the window (`--wails-draggable: drag`, controls
`no-drag`), maximises on double-click and draws full screen / minimise / maximise / close;
on macOS the native traffic lights sit over its left end
(`MacTitleBarHiddenInset`). The display window keeps its native frame; it opens
in a window (monitor button with a status dot in the top bar) and goes full
screen from its own button next to it.
Shared building blocks live in `styles.scss` (`.page-body`, `.page-head`,
`.card`, `.btn`/`.btn-primary`/`.btn-sm`, `.seg`/`.seg-option`, `.stepper`,
`.bib`, `.pill-*`): use them before adding component styles. Compare a screen
with the mockup by rendering both (the mockup opens in any browser).
