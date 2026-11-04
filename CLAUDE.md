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
src/internal/tournament/ event types, projection, commands; pure sub-packages:
                         forecast/ (schedule simulation), blocks/ (round order)
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
- **File dialogs remember their folder** per context (`dialogContext` in `lastdirs.go`: player list import, CSV exports, opening a tournament, archive copy, new or duplicated tournament), as the `lastDir.<context>` settings in app.db, stored once a dialog returns a path. A dialog opens in that folder, else its first existing parent (`firstExistingDir`); a context with no folder yet takes the next one of its fallbacks (`dialogFallbacks`: import and export take each other's, then the tournament ones; open, new and archive take each other's), else the tournaments folder (Documents when unset). A new tournament opens in the tournaments folder set in Settings first, the folder last used otherwise.
- **Navigation**: going home keeps the tournament open (the top bar offers to resume it); opening another file replaces it. The home screen lists recent files with a summary saved in app.db when a file is opened or closed (`TournamentSummary`), live for the open one. A recent file can go to the trash (`TrashTournament`, the open one is closed first): a separate uncapped list in app.db (`trashedTournaments` setting), never in the file. From the trash view, `RestoreTournament` puts it back on top of the list, `DeleteTournament(path, deleteFile)` drops it, and with `deleteFile` removes the file and its `-wal`/`-shm` (backups kept; `DELETE_FAILED` keeps it in the trash). Opening a trashed file takes it out of the trash.
- **Sample tournament** (`CreateSampleTournament`, `sample.go`): from home, a new file in the tournaments folder (name translated by the frontend, numbered when taken) built through the service's commands on a clock 95 min back: two categories of fictional pairs, pools then a full-placement bracket, two rounds played, courts busy, a score pending. The same builder makes the frontend test fixtures.
- **Guide**: the top bar's "?" and the home first-run card open the website's guide in the browser (`core/guide.ts`, `/guide/` in French, `/en/guide/` otherwise). The app never needs it to work.
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
- **One instance per tournament file**: the app may run several times (two tournaments side by side), but `store.OpenTournament`/`CreateTournament` hold an exclusive OS lock on a sibling `<name>.openshuttle-lock` (`flock`; `LockFileEx` on Windows, file hidden) until `Close`; a second open, in this instance or another, fails with `FILE_IN_USE`. The OS drops the lock when the process dies, so a lock file left by a crash is only reused; `Close` deletes it. Never lock the database itself (SQLite's own locks; on Unix closing any handle drops the process's fcntl locks). Reading a file open elsewhere (`stateOf` for Duplicate) goes through `PeekTournament` (no lock, `query_only`, no migration). `RemoveTournament` (trash delete) takes the lock first. Opening the file already open here reopens it.
- **app.db is shared by every instance** (WAL, `busy_timeout`): a setting read, changed and written back (recent list, trash, presets) goes through `AppDB.Update`, one `BEGIN IMMEDIATE` transaction (`updateRecent`, `updatePresets`), so another instance's write is never lost. Single-key settings (`lastDir.*`, Settings screen) are last write wins.

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
- **Check-in is per player**: each player is checked in once. A pair is complete when both players are present. The bib is assigned at the first check-in of the pair (a player waiting for a partner gets none). The check-in screen groups a pair's players and lists the teams fully checked in last (`sortTeams`); `CheckInEntrant` checks in every absent player of an entrant in one `EntrantCheckedIn` event (one undo).
- **The draw is per category.** Moving an entrant to a pool of the other category is refused (impossible action, not a warning).
- **UI**: every match, court, queue item and call shows a colored category badge (English: `MS`, `WS`, `MD`, `WD`, `XD`; French: `SH`, `SD`, `DH`, `DD`, `DM`). The control desk is a single view with a category filter on the queue. The display screen mixes both categories, with a very visible badge.
- **Full standings** require full-placement brackets. Otherwise, entrants losing in the same round are tied.
- **Players live inside their entrant**: categories are disjoint, so each player belongs to exactly one entrant. A player has one `Name` (first and last name typed together; events with `firstName`/`lastName` are joined on read) and a club. Screens always show full names. Same name twice is a warning (homonyms exist), never a refusal.
- **Player list import/export**: the export is a CSV with columns category (English or French short code), player 1, club 1, player 2, club 2, built by the frontend. The import reads any CSV (OpenShuttle's export, HelloAsso and other registration tools, spreadsheets): `ReadImportTable` (`;`, `,` or tab, BOM, Windows-1252 from Excel), then an `ImportMapping` says what the columns hold (category column and what each of its values means, a value mapped to "" is ignored on purpose, others guessed by `GuessCategory`; each player's whole name or first and last name, club, `SameClub` for a partner). Go guesses it from the header (`GuessImportMapping`); the Entrants screen shows it with the first lines and a live `PreviewImport`. A doubles line with one player enters a player waiting for a partner. Every valid line is entered in one `EntrantsImported` event (IDs `e<seq>.<n>`, `p<seq>.<n>.<k>`), so one undo cancels it. Invalid or already entered lines (same players in any order: a pair registered by each partner) are skipped and reported by line. A club cell naming a listed club (name or acronym) is entered as its name; the others are listed by `PreviewImport` (`ImportPreview.Clubs`) and the desk chooses for each before importing: add it to the list (the default; a short word in capitals prefills the acronym, anything else the name), enter it as a listed club (`ImportMapping.Clubs`: value → club), or keep it as typed (not added to the list). `ImportEntrants(path, mapping, add)` adds the new clubs first.
- **Waiting for a partner**: in doubles, an entrant with one player waits for a partner (`Entrant.Waiting`): typed alone ("registered alone" box on the Entrants screen) or imported from a line with only one player. It is never complete, so never drawn (the Draw screen warns with the count, never blocks), and gets no bib at check-in. On the Entrants screen the desk pairs two waiting players of a category (`PairEntrants`, `EntrantsPaired`: the partner's player joins the first entrant, the partner entrant is gone), adds a partner typed on the spot (`AddPartner`, `PartnerAdded`, player `p<seq>.1`), or splits a pair (`SplitEntrant`, `EntrantSplit`: the player leaves as a new waiting entrant `e<seq>`, the other keeps the bib; refused once the pair has matches, a draft draw loses it). A pair formed with a player present gets the next bib at once, otherwise at its first check-in. Each is one undoable event. Codes: `NOT_WAITING_PARTNER`, `NOT_SAME_CATEGORY`, `NOT_A_PAIR`.
- **Clubs**: app.db keeps a club list (`Presets.Clubs`, managed on the Presets screen): each club a name and an optional acronym (`tournament.Club`; older lists of plain names read as clubs without acronym). Values match a club by its name or acronym, case, accents, spaces and punctuation ignored (`ClubKey`; a name wins over another club's acronym); a club whose name or acronym is another club's refuses it (`CLUB_EXISTS`), unless that club is fully covered by the new one ("AUVR" alone merges into Rillieux, AUVR). Players keep their club as a string in the file: the entry form picks a listed club or adds one (name, acronym), Go enters a club typed by name or acronym as the listed club's name (`listedClubs`), and an unknown club typed for a player joins the list. The open tournament resolves clubs through a `ClubBook` built from the list and the settings (`State.WithClubs`, never projected; refreshed and `tournament.updated` emitted when the list or settings change, `followClubs`): "AUVR" and "Rillieux" are one club for the draw's same-club placement and warnings and its badges (`ClubIDs`), even in older files. **Club display** (Settings, `clubDisplay`): by name (default) or by acronym, the name when there is none. Go resolves it into the views (`PlayerView.ClubShown`, `SideView.Clubs`, `ClubBadge.Name` with `Full` as tooltip); `PlayerView.Club` and `SideView.ClubNames` hold the name, which the CSV exports always write.
- **IDs come from the event seq** (`e12`, `p12.1`): deterministic on replay and never reused, even after an undo of an undo.
- **Bibs** are recorded in the `PlayerCheckedIn` event and never reused: a removed entrant or a cancelled check-in keeps its number out of circulation. A cancelled check-in keeps the entrant's bib.
- **Removing a category with entrants is refused** (`CATEGORY_NOT_EMPTY`): it would silently drop them.
- **Format** (ADR 4, 5): per category, an ordered list of stages (`S1`, `S2`… never reused), each `pools` (a fixed number of pools or pool sizes by preference, tiebreak order, cross-pool comparison) or `bracket` (single elimination, full placement or not), with its win condition. The first stage is the entry stage, filled by the draw. Routing rules (`ranks`, `bestOfRank`, `best`, `rest`) apply in order and send entrants to a *later* stage; the others keep their final rank there, so the format is acyclic and phases follow. A bracket is final: it routes nobody on, and a new stage needs a pools stage to feed it (`BRACKET_IS_FINAL`). The whole format is one `FormatSet` event. Each stage locks on its own (ADR 9): once a match of a stage is called or scored, it keeps everything but its name, routing and estimate (`STAGE_STARTED`), and the rules feeding it stay. A stage not started changes freely, and stages are added or removed after the started ones. A later stage filled but not started whose kind, size or feeding rules change is emptied in the same event (`FormatSet.Emptied`, carried to the filled stages it feeds; one undo) and the desk fills it again; the entry stage is reopened through the draw instead (`STAGE_FILLED`, `DRAW_CONFIRMED`). The Format screen greys the fields of a started stage and warns on a filled one.
- **Pool sizes**: `PoolSizes` is a priority list (4, then 3, then 6 as a last resort); `engine.PlanPools` uses the fewest pools of the least wanted sizes, larger pools first, and `NO_POOL_SPLIT` when none fits. A fixed `Pools` count splits as evenly as possible. Entrants of one rank are compared across pools by wins, set difference, point difference; `sameMatches` first drops matches against entrants ranked below the smallest pool size.
- **Previews**: `tournament.PreviewFormat` simulates a format for N entrants; the Format screen and the preset editor share it and the `app-format-editor` component.
- **Format preview base**: the Format screen previews each category for complete pairs (`paired`: entrants not waiting for a partner, doubles only), every entrant (`registered`) or the present ones (`present`, the ones the draw takes). Go counts and previews every base (`FormatCategory.Previews`); the choice is a per-PC convenience in local storage, hidden once the entry stage is filled, and never changes the draw.
- **Presets** (win conditions, tiebreak orders, whole formats) live in app.db, shared by every tournament; edited on the Presets screen, outside any tournament (formats with a team-count simulator). Built-in ones (`b-…`) are read-only (`PRESET_READ_ONLY`): duplicate them to change them. Any preset can be disabled (`Presets.Disabled`): it stays on the Presets screen but is not offered in tournaments. The settings choose the default presets: the format a new category starts with (stored in its `CategoryAdded` event, the engine default when missing or invalid), and the win condition and tiebreak order of new stages and formats. Built-in presets play 2 sets of 15, cap 21. Applying a format preset copies it into the tournament (stage IDs renumbered from S1), only before the draw is confirmed.
- **Draw**: only present entrants (all players checked in). Seeds are a ranking set in one `SeedsSet` event (top seed first, the others unseeded), edited by drag and drop or arrows; they are placed in a snake (A, B, C, C, B, A…); the others in random order, each into the pool with the fewest same-club players. Go seeds the random generator; the `PoolsDrawn` event stores the resulting pools, never the seed. Drawing is refused only below two present entrants per pool or when no pool split fits; imbalance, same club and two seeds in a pool are warnings. A confirmed draw locks moves and the number of pools until reopened. Until then the ranking stays editable on the Draw screen; changing it after the pools are drawn only reorders them, and `State.RankingChanged` (the entry stage keeps the ranking it was drawn with, `Stage.DrawnSeeds`) makes the screen ask to draw again. Once confirmed, the screen says to reopen the draw to change the ranking. The UI calls the seed ranking "Classement" / "Ranking" (code `Seed`). A draft pool is always sorted by ranking (ranked best first, then the others): `EntrantMoved.Order` and `SeedsSet.Groups` store the resulting pool order, since a pool's order sets its round-robin pairings and match IDs (older events without them append, as before). Each ranked entrant shows its level (`DrawEntrantView.Tier`: with P pools, ranks 1 to P are level 1…) as a rank chip colored `--color-tier-<n>`; the "N seeds in this pool" warning counts level 1 only. Clubs show as badges on the Draw screen (`ClubBadge`): Go gives each club a color index from its order of first appearance among the entrants (`State.Clubs`, `--color-club-1..10` in `styles.scss`) and counts the entrants of the pool holding it (each player's club counts, so a mixed-club pair has two badges); a club held twice or more in a pool is filled with `×n` (no separate "same club" tag), otherwise pale; no club, no badge.
- **Drag and drop** uses pointer events, not HTML5 drag and drop (unreliable across WebKitGTK, WebView2 and WKWebView).
- **Pool matches are derived, not stored**: confirming a draw generates the round-robin matches (circle method, rounds interleaved across pools, IDs like `MD-A3`). Events record what happens to them: `MatchCalled`, `MatchEnded`, `ScoreRecorded`, `EntrantWithdrawn`. Event times are the call and end times. Once a match has started, the draw cannot be reopened. Once the draw is confirmed, an entrant has matches (`State.InMatches`): it can no longer be removed or split (`ENTRANT_IN_POOLS`); the Entrants screen hides these actions (`EntrantView.Locked`) and points to a forfeit, names and clubs stay editable.
- **Calling is refused only when impossible**: a match already in progress on that court, or an entrant already on another court. Resting is shown in the queue, never enforced: any waiting match of the queue can be called on a court with no match in progress ("Force" when an entrant rests or is absent).
- **Schedule** (ADR 6, 7, 8): matches follow an order of blocks: a block is a round of a pool (`MD/S1/A/2`) or a round of a part of a bracket (`MD/S2/R2` main draw, `MD/S2/R2/P5` places 5 to 8 with full placement); stages not filled yet bring their blocks from their stand-ins, so the desk orders them too. Default order in waves per category, phase after phase: wave k = round k of every pool and every bracket of the phase (stages of one phase play side by side), categories side by side, the category opening first, then pools with the most matches. Before validation the desk reorders blocks in a list beside a read-only timeline (`MoveScheduledBlock`, `ScheduleBlocksSet` stores the whole order; the rounds of a pool or a bracket always keep their order; older files storing pools `MD/S1/A` rank them within each wave). `ValidateSchedule` freezes the match order (`ScheduleOrderSet`); then only single matches move on the timeline (`MoveScheduledMatch`); `UnvalidateSchedule` (empty `ScheduleOrderSet`) goes back to blocks. While dragging, the screen shows the result of the drop live (`PreviewScheduledBlockMove`, `PreviewScheduledMatchMove`: the log projected in memory with the move, nothing saved), the pointer working on what is shown and the screen settling between two moves. Matches of stages filled later join the end. `State.Schedule(now)` forecasts courts and times: a free court takes the first match of the order whose players are off court (bracket rounds wait for the previous one, unfilled stages for their sources as stand-ins in wave order, reserved courts apply). Durations come from each win condition's `Estimate` (0: `sets × (2/3 × points + 1)` min). Rest is never waited for: unrested matches are flagged and the desk warned. The Schedule screen shows played matches greyed at their real court and times, and the teams' matches left (`MatchesLeft`); bracket rounds are named by size (eighth, quarter, semi, final).
- **Day and breaks** (ADR 10): `TournamentDaySet` stores the date and the start and end hours (Info screen, each optional, local time). While no match is called the forecast starts at the start hour (`ForecastStart`, now once past), then now; proposals ignore it. A forecast ending after the end hour is flagged (`ScheduleView.Late`). Breaks are fixed periods of every court (`BreakAdded`, ID `k<seq>`, `BreakRemoved`), added on the Schedule screen: no match is planned across one (`forecast.Input.Breaks`), none is proposed during one. Codes: `INVALID_TIME`.
- **Schedule labels and colors**: each pool and each bracket stage gets a color index, numbered through the whole tournament so colors repeat as little as possible (`State.GroupStyles`, wrapping at `GroupColors` = 16, `--color-group-n`), carried by the category badge of its blocks and matches; pools also give their round count (`Rounds`). Each stage has an optional `Acronym` (max 3 characters, in `StageConfig`, editable once started; empty shows `stage.acronym.<kind>`: P, T in French, B in English). Rounds read `P A · 2/5` or `T · 1/4` (`shortRoundLabel`: `1/8`, `1/4`, `1/2`, `Finale`, `3e place`, `5e–8e`; English `R16`, `QF`, `SF`, `Final`, `3rd place`, `5th–8th`).
- **Side panels** (tournament menu, right panels of Info, Schedule, Stages, the Schedule's order of rounds) fold; the state is a per-PC convenience in local storage (`Panels`).
- **Category shown**: the category selectors (Entrants list and add form, Check-in, Control desk queue, Format, Stages, Results) share the last choice (`CategoryShown`), a per-PC convenience in local storage. Screens offering "All" take the last value chosen (All or a category), the others the last specific category; one the open tournament lacks falls back to the screen's default (All, or the first category). The Entrants add form keeps the list filter as it is.
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
| Classement (formerly tête de série) | Seed (the rank in the draw ranking) |
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
`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`.
On Linux: `libgtk-4-dev libwebkitgtk-6.0-dev libsoup-3.0-dev`.

```
cd src/apps/desktop && wails3 dev             # run the desktop app with hot reload
cd src/apps/desktop && wails3 build           # production binary in src/apps/desktop/bin
cd src/apps/desktop && wails3 generate bindings -clean=true -ts -i   # after changing a Go service
cd src && go test ./...                       # all Go tests (build the frontend first: the desktop package embeds frontend/dist)
cd src/apps/desktop/frontend && npm test      # frontend unit tests (Vitest)
cd src/apps/desktop/frontend && npm run i18n:check   # en.json and fr.json must share the same keys
cd src/apps/desktop && OPENSHUTTLE_FIXTURES=$PWD/frontend/src/testing/fixtures go test -run TestWriteFrontendFixtures .   # after changing a view
```

Frontend tests render each screen with Go faked: `src/testing/backend.ts`
(`FakeBackend`: a Wails transport answering each binding, by name, and recording
the calls) and `src/testing/render.ts` (real translations, `click`, `clickAll`,
`drag`…). Views come from `src/testing/fixtures/*.json`, written by the Go test
above from demo tournaments (fixed clock, seeded draw): regenerate them when a
view changes. `src/i18n/translations.spec.ts` checks the translation files
themselves: valid JSON, every message compiles (ICU), same keys and parameters
in en and fr. Every screen has a spec; coverage counts every file of `src/app`
(Sonar wants 80 % on new code).

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
DMG icon and background) are generated from it (`wails3 generate icons`);
`windows/appicon.ico`, a copy of `icon.ico`, is the icon of `.openshuttle` files
the NSIS installer registers (`fileAssociations` in `build/config.yml`).
The installer is per user (`INSTALL_SCOPE` defaults to `user` in `build/windows/Taskfile.yml`):
`%LOCALAPPDATA%\Programs\OpenShuttle`, HKCU registry, no administrator rights; it tells
users of an earlier machine-wide install (Program Files) how to remove it.
The main window has no native title bar on Windows and Linux (`Frameless`):
the app's top bar drags the window (`--wails-draggable: drag`, controls
`no-drag`), maximises on double-click and draws full screen / minimise / maximise / close;
on macOS the native traffic lights sit over its left end
(`MacTitleBarHiddenInset`). The display window keeps its native frame; it opens
in a window (monitor button with a status dot in the top bar) and goes full
screen from its own window: a button that shows only while the mouse moves
over it (hidden from the hall), or a double-click.
Shared building blocks live in `styles.scss` (`.page-body`, `.page-head`,
`.card`, `.btn`/`.btn-primary`/`.btn-sm`, `.seg`/`.seg-option`, `.stepper`,
`.bib`, `.pill-*`, `.empty`, `.empty-line`, `.alert`, `.columns`): use them before adding
component styles. Repeated patterns are mixins (`src/styles/_mixins.scss`:
`ellipsis`, `stack($gap)`, `row($gap, $wrap)`; `@use 'mixins' as *;`). Compare a screen
with the mockup by rendering both (the mockup opens in any browser).
