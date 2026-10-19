# OpenShuttle: functional scope, v1

This is the functional reference for v1. Architecture decisions live in
[`docs/adr`](../adr); rules for contributors in
[`CONTRIBUTING.md`](../../CONTRIBUTING.md) and [`CLAUDE.md`](../../CLAUDE.md).

## Vision and v1 scope

An open source application to run a non-sanctioned badminton club tournament
from start to finish, on one PC, with no connection at all. It is inspired by
ShuttleCup but stands out with formats made of freely chained stages, a
schedule that drives the courts, and a control desk designed to correct the
tournament live.

**In v1**

- 100 % local desktop application: one control desk PC and a second screen
  for the hall, with no network and no printing.
- One or two match categories per tournament (for example men's doubles and
  women's doubles), with no player in both, on shared courts.
- Formats made of stages, each with its own win condition.
- Reusable presets, edited outside any tournament; a tournament is prepared
  beforehand and can still change on the day.
- Check-in, draw, court management, schedule and match calls.
- Score entry at the desk, starting from the court concerned.
- Manual changes to pools and brackets during the tournament, with undo and
  history.
- Full standings at the end.
- English and French user interface from v1.

**Not in v1**: anything going through the network (following and scoring on
phones, publishing online), more than two categories at once, a player
entered in two categories.

## Personas

Three profiles use the application, but only the first two operate it. They
are often volunteers, sometimes not at ease with computers, working in a
noisy gym under pressure.

| Persona | Context | Main need | What gets in their way |
| --- | --- | --- | --- |
| Organizer | At home, the days before, in a quiet place | Prepare the tournament: entrants, format, courts, timing | A rigid format, typing everything again from one tournament to the next |
| Control desk | On the day, in the gym, often 2 people on 1 PC | Keep the courts busy, enter scores, handle the unexpected | An action blocked by the tool, a mistake that cannot be undone |
| Players and audience | In the hall, never touching the app | Know when and on which court they play, follow pools and brackets | A screen unreadable from 10 m, a missed call |

The organizer and the control desk are often the same person at two moments.
The audience only sees the display screen.

## Glossary

Use these terms as is, in the code and in the user interface. The French
column is the French UI label.

| English (code and UI) | French | Definition |
| --- | --- | --- |
| Tournament | Tournoi | A one-day (or longer) event for one or two match categories, with its entrants, its courts and one format per category |
| Category | Catégorie | The kind of match: men's singles, women's singles, men's doubles, women's doubles, mixed doubles. Short code shown everywhere: MS, WS, MD, WD, XD (French: SH, SD, DH, DD, DM) |
| Entrant | Participant | A player in singles, a pair in doubles |
| Bib number | Dossard | Number given to each entrant at check-in, shown next to the name everywhere |
| Format | Format | The chain of stages of a category |
| Preset | Preset | A saved format, win condition or tiebreak order, reused from one tournament to the next |
| Stage | Phase | A step of the format: a set of pools or a bracket |
| Pool | Poule | Group of entrants who all play each other, with its own standings |
| Bracket | Tableau | Single elimination tree (main, consolation, places 5-8…) |
| Consolation bracket | Tableau consolante | Bracket of the entrants not qualified for the main one |
| Advancement rule | Routage | Rule sending entrants of a stage to a later one by rank |
| Win condition | Condition de victoire | How a match of a stage is won: sets, points, cap, or time; with its estimated time on court |
| Full placement | Placement complet | Bracket where every place is played, for standings without ties |
| Golden point | Point en or | Single deciding point played when a timed match ends in a tie |
| Round (timed) | Rotation | Set of timed matches started and whistled together on every court |
| Pending score | Score en attente | Match over whose score is not entered yet; the court stays usable |
| Schedule | Échéancier | Forecast of every match left, court by court, which also sets the order of the calls |
| Walkover | WO | Match won because the opponent forfeited, with no score |
| Check-in | Pointage | Confirming on the day which entrants are present |
| Draw | Tirage | Placing the entrants into the pools |
| Seed | Tête de série | Entrant placed so as not to meet the other favourites too early |
| Control desk | Table de marque | The station running the tournament on the day |
| Match call | Appel | Announcing a match on a court, shown on the display screen |
| Display screen | Écran d'appel | Second window, on the hall's screen or projector |

## Tournament model

A format is a graph of stages. Each stage has its entrants and its win
condition, and advancement rules send entrants to a later stage by rank
([ADR 4](../adr/0004-flexible-format.md), [ADR 5](../adr/0005-pool-sizes-and-routing-rules.md)).

**Example format (22 entrants)**

1. Stage 1: pools of 4, else of 5 (here 5, 5, 4, 4, 4), round robin,
   2 sets of 15.
2. Ranks 1 to 3 of each pool, plus the best 4th compared across pools → main
   bracket (16 entrants, full placement) → places 1 to 16.
3. Everyone else → consolation bracket (6 entrants, full placement) →
   places 17 to 22.
4. The brackets end to end give the full standings, places 1 to 22.

**Settings of a stage**

| Setting | Possible values |
| --- | --- |
| Kind | Pools (round robin) or single elimination bracket |
| Size | Pool sizes in order of preference (4, else 3, 6 as a last resort), or a fixed number of pools; a bracket takes the entrants routed to it |
| Win condition | Sets to win, points per set, point cap, or timed match (duration + golden point); estimated time on court |
| Tiebreaks (pools) | Order of the criteria, see Business rules |
| Comparing across pools | Raw results, or the same number of matches (drops matches against the lowest ranks of larger pools) |
| Standings (brackets) | Full placement (every place played) or tied by round lost |
| Advancement rules | In order: given ranks, the N best of a rank, the N best, the rest |

- **A bracket is final**: it sends nobody to a later stage. A new stage needs
  a pools stage to feed it.
- **Full standings** are complete only if every bracket has full placement.
  Otherwise, entrants losing in the same round are tied.
- **One format per category**: with two categories, each has its own format.
- **Presets** are edited on the Presets screen, outside any tournament, with a
  simulation for a given number of teams (pools, matches, rounds, places).
  Built-in presets are read-only (duplicate them to change them) and every
  preset can be disabled so tournaments no longer offer it. The settings
  choose the default format, win condition and tiebreak order.
- **The format is locked** as soon as a match of the category is called or
  scored.

## Business rules

These rules are decided for v1.

**Forfeits**

- **In a pool**: a forfeit turns every match of the entrant into a walkover,
  played or not. The win counts for the opponent, but these matches are left
  out of set and point calculations. The entrant forfeiting ranks last in
  the pool.
- **In a bracket**: a forfeit counts as a loss, the opponent goes on.
- The interface shows "WO" instead of the score.

**Pool tiebreaks**

Criteria applied in order, configurable per stage:

1. Wins
2. Head-to-head (between exactly two tied entrants)
3. Set difference
4. Point difference
5. Random draw

Criteria use differences, never ratios, so walkovers stay neutral.

**Timed matches**

- The leader at the whistle wins. A tie is decided by a golden point.
- A timed stage is played in **rounds**: every court starts and ends
  together. The desk starts the round, then whistles its end; the countdown is
  only a guide and nothing changes state without the desk.
- At the end of a round, every match goes to pending scores. The next round
  can start without waiting for the scores.
- An entrant plays at most once per round; those who sat out go first. The
  minimum rest does not apply.
- Timed matches are only available when the tournament has a single category.

**End of a match (sets and points)**

- The desk declares the end of a match ("End" on the court, or entering the
  score directly).
- A finished court can take the next match before the score is entered: the
  finished match goes to the pending scores. Rest counts from the real end of
  the match, not from score entry.

**Schedule** ([ADR 6](../adr/0006-schedule.md), [ADR 7](../adr/0007-validated-schedule-order.md))

- The schedule puts every waiting match in order and forecasts courts and
  times from the estimated time on court of each win condition (2 sets of 21:
  30 min, 2 sets of 15: 22 min, 1 set of 21: 15 min, editable).
- **Automatic order**, so everyone waits about as long: round by round, the
  pools with the most matches first, then the others in turn. With pools of
  5, 5, 4, 4 (A, B, C, D): A, B, C, D for rounds 1 to 3, then A, B for
  rounds 4 and 5. Brackets go round by round: no semi-final while a
  quarter-final is still playing.
- **Two categories** share the courts in one of two modes: *in turn* (round 1
  of one category, round 1 of the other, then round 2…) or *reserved courts*
  (each category keeps its courts until it is over).
- The desk **validates** the schedule once: the order is then fixed, and the
  desk moves a single match or a whole pool by drag and drop. Recomputing
  starts again from the automatic order.
- Courts take the matches in that order, as soon as both sides are off court.
  When players have not had their minimum rest, the desk is warned: it calls
  the match anyway, or moves matches in the schedule.
- The schedule shows how many matches each team has left in the stage.

**Two categories at once**

- Two categories at most, with no player in both. One men's category and one
  women's; mixed doubles always runs alone.
- Each category has its own draw, pools, brackets and full standings.

**Manual changes**

The desk can at any time move an entrant, edit or clear a score, swap two
matches, declare a forfeit, reorder the schedule. Every action is recorded
in a log: it can be undone, and the history shows who changed what and when.
The tool warns about inconsistencies but never blocks an action that is
possible.

## Running a tournament

A tournament goes through six steps.

1. **Preparation (beforehand)**: the organizer creates the tournament,
   chooses one or two categories, imports or types the entrants (one name
   field per player, a club picked from the club list), starts from a preset
   or builds the format, and sets the number of courts and the minimum rest.
2. **Check-in (on the day, arrivals)**: the desk marks players present one by
   one (a player checked in once is checked in for the entrant), hands out
   bibs, removes absentees and adds late entries. A pair is ready when both
   players are checked in.
3. **Draw**: pools of each category from the present entrants only, with seeds
   ranked by the desk (drag and drop or arrows) and placed in a snake, and
   club mates kept apart. The desk can adjust by hand before confirming.
4. **Matches**: the schedule offers the next match for each free or finished
   court. The desk calls it and it appears on the display screen. At the end,
   the desk ends the match, then enters the score at once or later from the
   pending scores. In a timed stage, the desk starts and whistles whole
   rounds.
5. **Next stage**: once the stages feeding it are complete, a later stage is
   filled by its advancement rules. The desk checks it and can correct it
   before play.
6. **Closing**: full standings, CSV export, archive of the tournament file on
   the PC.

## Screens

**Navigation**

- **Outside a tournament**, a home screen: recent tournaments (one file per
  tournament), "New tournament", "Open a file", then Presets and Settings.
- **Inside a tournament**, a side navigation following the steps in two
  groups: Preparation (Info, Entrants, Format) then On the day (Check-in,
  Draw, Schedule, Control desk, Stages, Results). The current step stands out,
  finished steps are ticked, everything stays clickable.
- **Top bar, always there**: tournament name and categories, time, latest
  action with an "Undo" button, display screen controls (open, choose the
  monitor).

**Control desk dashboard**

Where the desk spends 90 % of the day. Two areas side by side:

- **Court grid** (one card per court, category code on every match), with
  three distinct states:
  - playing: entrants, pool, time played, "End" button;
  - free: next match proposed and "Call" button;
  - finished: the match over (click to enter its score) and the next match
    proposed, with "Enter score" and "Start next".
  A proposal whose players have not rested enough is flagged, with "Call
  anyway" and a link to the schedule.
- **In a timed stage**, a round banner above the grid: shared countdown,
  "Whistle", then "Start next round". No countdown per court.
- **Right column**, top to bottom: pending scores (court, match, end time,
  highlighted after 10 min), the queue of next matches with a category filter
  and why a match is not ready (player resting for X min, player busy on
  another court), then alerts needing a decision (pool complete, stage to
  fill).

Score entry is not a separate screen: a panel opens from the court, with both
sides, as many set fields as the win condition needs, and the "WO" and
"Golden point" buttons when they apply.

**Screen list**

| # | Screen | Persona | Content | Key actions |
| --- | --- | --- | --- | --- |
| 1 | Home | Organizer | Recent tournaments, Presets, Settings | Create, open a file, duplicate |
| 2 | Presets | Organizer | Formats with a preview of their graph, win conditions, tiebreak orders, clubs | Create, edit, duplicate, disable, delete; simulate a format for N teams |
| 3 | Tournament info | Organizer | Name, one or two categories, number of courts, minimum rest | Edit |
| 4 | Entrants | Organizer | Entrants (player or pair) per category, club | Add, edit, import and export CSV |
| 5 | Format | Organizer | One tab per category; the stage graph in the middle, the selected stage's settings on the right, the preview for N entrants below | Add and link stages, configure, apply or save a preset |
| 6 | Check-in | Control desk | Large list per player with search, category, partner, bib, counters "40 / 42 players · 20 / 21 complete pairs" | Mark present in one click, add on the spot |
| 7 | Draw | Control desk | One section per category, one column per pool, entrant cards, warning badges (same club), seed ranking | Drag and drop, redraw, confirm |
| 8 | Schedule | Control desk | Timeline per court with estimated times, order of the matches, matches left per team, how two categories share the courts | Validate, move a match or a whole pool, recompute, reserve courts |
| 9 | Control desk | Control desk | Dashboard above, with the score panel | Call, end, start next, enter a score, declare a forfeit, start and whistle a round |
| 10 | Stages | Control desk | Category, then one tab per stage: pool standings or bracket tree | "Correction" mode to move, swap, edit; history |
| 11 | Results | Control desk | Full standings per category | Export, archive |
| 12 | Display screen | Players and audience | Second full-screen window, both categories mixed with a very visible code: called matches in very large type with their court, grid of courts in play, automatic rotation between pools and brackets. In a timed stage: the courts of the round and a large shared countdown | None, display only |
| 13 | Settings | Organizer | Language, display screen monitor, defaults (courts, rest, default presets), tournaments folder | Edit |

## UX principles and hardware constraints

The interface must stay usable by a stressed volunteer, on a laptop, in a gym.

**Hardware**

- A laptop (13 to 15 inch screen), keyboard and mouse, touch not guaranteed.
- A second screen or projector over HDMI, read from 10 m or more.
- No network, local or internet, and no printing.
- Desktop application (Wails) with a web interface (Angular): two native
  windows, the main one and the display screen.

**Principles**

- **Never block**: the tool warns, the desk decides. Every action can be
  undone.
- **Dashboard first**: on the day, everything starts from the control desk,
  and frequent actions take one or two clicks.
- **A court's state reads at a glance**: playing, free, finished, told apart by
  colour and label.
- **A category reads at a glance**: coloured code (MD, WD…) next to every
  match, everywhere.
- **Visual restraint**: backgrounds and neutrals in cool greys and white;
  colour is kept for states (playing, free, finished, alert) and categories.
- **Readable from afar**: the display screen uses very large type, strong
  contrast and a prominent court number.
- **Bib + full name** everywhere, to tell homonyms apart.
- **Glossary vocabulary** only.
- **Bilingual**: English and French. French runs 15 to 20 % longer: buttons
  and labels must cope.
- **No keyboard shortcuts**: every action is a button. Enter and Escape keep
  their usual meaning in forms.

## Out of v1 scope, ideas for v2

v2 adds an optional online mode, never making the tournament depend on the
network. Not to be built for v1, but kept in mind so as not to close doors.

- **Local first**: the control desk PC stays the source of truth. If the
  internet goes down, the tournament goes on and only online following
  freezes.
- **One-way publishing**: the PC pushes a public view (pools, brackets,
  scores, calls) to a self-hostable relay server.
- **Following and scoring on phones**: access by QR code, over the
  audience's mobile data or an optional local network.
- **Pseudonyms**: no real names online. Per tournament: bib, nickname chosen at
  check-in, random team name, or first name + initial.
- **Automatic purge** of the online tournament 7 days after it ends.
- **More than two categories at once**, and a player entered in several
  categories (conflicts and rest across categories).
- **Player registry** shared across tournaments, and a hall plan with
  numbered courts that can be disabled.
