package main

import (
	"fmt"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament/blocks"
)

// scheduleView builds the schedule screen of a state at a time.
func (s *TournamentService) scheduleView(st tournament.State, now time.Time) ScheduleView {
	settings := st.ScheduleSettings()
	sched := st.ScheduleWithGhosts(now)
	b := scheduleBuilder{
		st: st, now: now, mv: s.matchViewer(st), refs: categoryRefs(st), left: map[string]map[engine.EntrantID]int{},
		styles: st.GroupStyles(),
	}
	v := ScheduleView{
		Now: now, Mode: settings.Mode, First: settings.First, Categories: b.refs, Courts: st.Courts(),
		MinRest: st.MinRest(), Reserved: []CourtReservation{}, Slots: []ScheduleSlotView{},
		Blocks: []ScheduleBlockView{}, Groups: b.groups(), Highlights: b.highlights(), Breaks: []BreakView{}, Ghosts: b.ghosts(sched.Slots),
	}
	for _, br := range st.BreakTimes(now) {
		v.Breaks = append(v.Breaks, BreakView{ID: br.ID, From: br.From, To: br.To})
	}
	if start := st.ForecastStart(now); start.After(now) {
		v.Start = &start
	}
	if end := st.DayEnd(now); !end.IsZero() {
		v.DayEnd = &end
		v.Late = sched.End.After(end)
	}
	if at := st.ScheduleValidatedAt(); !at.IsZero() {
		v.ValidatedAt = &at
		b.validated = true
	}
	if v.First == "" && len(b.refs) > 0 {
		v.First = b.refs[0].Category
	}
	for court := 1; court <= st.Courts(); court++ {
		if c, ok := sched.Reserved[court]; ok {
			v.Reserved = append(v.Reserved, CourtReservation{Number: court, Category: c})
		}
	}
	v.CourtRules, v.Scopes = b.courtRules(settings.Rules)
	v.Slots = b.calledSlots()
	planned, blocks := b.plannedSlots(sched.Slots)
	v.Slots = append(v.Slots, planned...)
	v.Blocks = blocks
	for _, sl := range sched.Slots {
		if sl.Unrested {
			v.Unrested++
		}
		if sl.OffRules {
			v.OffRules++
		}
	}
	if !sched.End.IsZero() {
		end := sched.End
		v.End = &end
	}
	return v
}

// scheduleBuilder builds the parts of a schedule view.
type scheduleBuilder struct {
	st        tournament.State
	now       time.Time
	mv        func(tournament.Match) MatchView
	refs      []CategoryRef
	validated bool
	left      map[string]map[engine.EntrantID]int // matches left per "<cat>/<stage>"
	styles    map[string]tournament.GroupStyle
}

// style is the color, acronym and round count of a pool or bracket stage.
func (b *scheduleBuilder) style(c engine.Category, stage tournament.Stage, pool int) (string, int, int) {
	gs := tournament.GroupStyleOf(b.styles, c, stage.ID, pool)
	return stage.Acronym, gs.Color, gs.Rounds
}

// leftIn counts, per entrant, the matches it has left in a stage.
func (b *scheduleBuilder) leftIn(c engine.Category, stage string) map[engine.EntrantID]int {
	k := string(c) + "/" + stage
	if b.left[k] == nil {
		b.left[k] = b.st.MatchesLeft(c, stage)
	}
	return b.left[k]
}

func (b *scheduleBuilder) slotOf(c engine.Category) int {
	for _, r := range b.refs {
		if r.Category == c {
			return r.Slot
		}
	}
	return 0
}

// slot is the view of a schedule slot.
func (b *scheduleBuilder) slot(sl tournament.ScheduleSlot, state string) ScheduleSlotView {
	st := b.st
	stage, _ := st.Stage(sl.Category, sl.Stage)
	sv := ScheduleSlotView{
		Key: sl.Key, State: state, CategoryRef: CategoryRef{sl.Category, b.slotOf(sl.Category)},
		Stage: sl.Stage, StageName: stage.Name, StageKind: stage.Kind,
		EntryStage: st.Stages(sl.Category)[0].ID == sl.Stage,
		Round:      sl.Level + 1, RoundSize: sl.RoundSize, Court: sl.Court, Start: sl.Start, End: sl.End,
		PlaceFrom: sl.PlaceFrom, Unrested: sl.Unrested, OffRules: sl.OffRules,
	}
	sv.Acronym, sv.Color, sv.Rounds = b.style(sl.Category, stage, sl.Pool)
	sv.Group = groupKey(sl.Category, sl.Stage, sl.Pool)
	if sl.Pool >= 0 {
		sv.Pool = engine.PoolName(sl.Pool)
	} else if sl.PlaceFrom > 0 {
		sv.PlaceTo = sl.PlaceFrom + sl.RoundSize - 1
	}
	if m, ok := st.Match(sl.Match); ok && sl.Match != "" {
		view := b.mv(m)
		sv.Match = &view
		l := b.leftIn(m.Category, m.Stage)
		sv.LeftA, sv.LeftB = l[m.A], l[m.B]
	}
	// Matches move one by one once validated; before, blocks move in the
	// block list.
	sv.Movable = state == SlotPlanned && b.validated && sl.Match != ""
	return sv
}

// calledSlots are the matches played or playing, at their real court and
// times; a match playing ends at its estimate, or now when running late.
func (b *scheduleBuilder) calledSlots() []ScheduleSlotView {
	st := b.st
	var out []ScheduleSlotView
	for _, m := range st.Matches() {
		if !m.Called() || st.Walkover(m) {
			continue
		}
		sl := tournament.ScheduleSlot{
			Key: st.MatchBlock(m), Category: m.Category, Stage: m.Stage, Pool: m.Pool, Level: m.Round,
			Match: m.ID, A: m.A, B: m.B, Court: m.Court, Start: m.CalledAt, End: m.EndedAt, RoundSize: m.Size, PlaceFrom: m.Start,
		}
		state := SlotPlayed
		if m.IsBracket() {
			sl.Level = st.BracketLevel(m)
		}
		if !m.Ended() {
			state = SlotPlaying
			sl.End = later(b.now, m.CalledAt.Add(time.Duration(st.WinCondition(m).EstimatedMinutes())*time.Minute))
		}
		out = append(out, b.slot(sl, state))
	}
	return out
}

// plannedSlots are the forecast's slots, and its blocks in block order with
// their matches' times.
func (b *scheduleBuilder) plannedSlots(slots []tournament.ScheduleSlot) ([]ScheduleSlotView, []ScheduleBlockView) {
	var out []ScheduleSlotView
	byKey := map[string]*ScheduleBlockView{}
	for _, sl := range slots {
		if sl.Ghost != "" {
			continue
		}
		sv := b.slot(sl, SlotPlanned)
		out = append(out, sv)
		bv := byKey[sl.Key]
		if bv == nil {
			bv = &ScheduleBlockView{
				Key: sv.Key, CategoryRef: sv.CategoryRef, Stage: sv.Stage, StageName: sv.StageName, StageKind: sv.StageKind,
				Acronym: sv.Acronym, Color: sv.Color, Rounds: sv.Rounds, Group: sv.Group,
				EntryStage: sv.EntryStage, Pool: sv.Pool, Round: sv.Round, RoundSize: sv.RoundSize, PlaceFrom: sv.PlaceFrom,
				PlaceTo: sv.PlaceTo, Pending: sl.Match == "", Start: sl.Start, End: sl.End,
			}
			byKey[sl.Key] = bv
		}
		bv.Matches++
		if sl.Start.Before(bv.Start) {
			bv.Start = sl.Start
		}
		bv.End = later(bv.End, sl.End)
		if sl.Unrested {
			bv.Unrested++
		}
	}
	blocks := []ScheduleBlockView{}
	for _, k := range b.st.ScheduleBlocks() {
		if bv := byKey[k]; bv != nil {
			blocks = append(blocks, *bv)
		}
	}
	return out, blocks
}

// ghosts are the ghost matches still ahead, at their forecast times.
func (b *scheduleBuilder) ghosts(slots []tournament.ScheduleSlot) []GhostMatchView {
	out := []GhostMatchView{}
	for _, g := range b.st.GhostMatches() {
		gv := GhostMatchView{ID: g.ID, Court: g.Court, Minutes: g.Minutes}
		if m, ok := b.st.Match(g.After); ok && g.After != "" {
			view := b.mv(m)
			gv.After = &view
		}
		if i := slices.IndexFunc(slots, func(sl tournament.ScheduleSlot) bool { return sl.Ghost == g.ID }); i >= 0 {
			start, end := slots[i].Start, slots[i].End
			gv.Start, gv.End = &start, &end
		}
		out = append(out, gv)
	}
	return out
}

// groups lists the teams with matches left, pool by pool (a bracket as one
// group).
func (b *scheduleBuilder) groups() []ScheduleGroupView {
	st := b.st
	side := sideViewer(st)
	playing := map[engine.EntrantID]bool{}
	for _, m := range st.Matches() {
		if st.Playing(m) {
			playing[m.A], playing[m.B] = true, true
		}
	}
	out := []ScheduleGroupView{}
	for _, ref := range b.refs {
		for i, stage := range st.Stages(ref.Category) {
			if !stage.Confirmed {
				continue
			}
			l := b.leftIn(ref.Category, stage.ID)
			for g, group := range stage.Groups {
				gv := ScheduleGroupView{
					CategoryRef: ref, Stage: stage.ID, StageName: stage.Name, StageKind: stage.Kind, EntryStage: i == 0,
					Teams: []ScheduleTeamView{},
				}
				pool := -1
				if stage.Kind == tournament.StagePools {
					gv.Pool, pool = engine.PoolName(g), g
				}
				gv.Acronym, gv.Color, _ = b.style(ref.Category, stage, pool)
				gv.Key = fmt.Sprintf("%s/%s/%s", ref.Category, stage.ID, gv.Pool)
				for _, id := range group {
					if id == "" || l[id] == 0 {
						continue
					}
					sv := side(id)
					gv.Teams = append(gv.Teams, ScheduleTeamView{Entrant: id, Bib: sv.Bib, Names: sv.Names, Left: l[id], Playing: playing[id]})
				}
				if len(gv.Teams) > 0 {
					out = append(out, gv)
				}
			}
		}
	}
	return out
}

// groupKey is the key of a pool ("MD/S1/A") or of a bracket stage ("MD/S2"),
// as in State.GroupStyles.
func groupKey(c engine.Category, stage string, pool int) string {
	if pool >= 0 {
		return blocks.PoolGroup(c, stage, pool)
	}
	return string(c) + "/" + stage
}

// highlights lists every pool and bracket stage of the tournament, in
// category, stage and pool order: the ones the desk can highlight. Stages
// not filled yet bring the pools their preview plans.
func (b *scheduleBuilder) highlights() []ScheduleHighlightView {
	out := []ScheduleHighlightView{}
	for _, ref := range b.refs {
		for i, stage := range b.st.Stages(ref.Category) {
			hv := ScheduleHighlightView{
				CategoryRef: ref, Stage: stage.ID, StageName: stage.Name, StageKind: stage.Kind,
				EntryStage: i == 0, Acronym: stage.Acronym,
			}
			if stage.Kind != tournament.StagePools {
				hv.Key = groupKey(ref.Category, stage.ID, -1)
				hv.Color = b.styles[hv.Key].Color
				out = append(out, hv)
				continue
			}
			for pool := 0; ; pool++ {
				key := groupKey(ref.Category, stage.ID, pool)
				gs, ok := b.styles[key]
				if !ok {
					break
				}
				hv.Key, hv.Pool, hv.Color = key, engine.PoolName(pool), gs.Color
				out = append(out, hv)
			}
		}
	}
	return out
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// Slot states on the schedule screen.
const (
	SlotPlayed  = "played"  // over on court: real times
	SlotPlaying = "playing" // on court: real start, estimated end
	SlotPlanned = "planned" // waiting: estimated times
)

// ScheduleSlotView is a match of the schedule screen, on a court with its
// times: real for matches played or playing, estimated for the others.
type ScheduleSlotView struct {
	// Key is the match's block: its pool ("MD/S1/A") or bracket round
	// ("MD/S2/R1").
	Key   string `json:"key"`
	State string `json:"state"`
	// Movable: the desk can drag it: its block before validation, the match
	// itself after.
	Movable bool `json:"movable"`
	CategoryRef
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	// Acronym is the stage's short name typed by the desk ("" for the kind's
	// default). Color is the pool's, or the bracket's, color index
	// (--color-group-n).
	Acronym string `json:"acronym"`
	Color   int    `json:"color"`
	// Pool is "A", "B"… in a pools stage, "" in a bracket.
	Pool string `json:"pool"`
	// Group is the key of its pool or bracket stage, as in
	// ScheduleView.Highlights.
	Group string `json:"group"`
	// Round is the round of the pool or of the bracket, from 1. Rounds is how
	// many rounds the pool plays (0 in a bracket). RoundSize is, in a
	// bracket, how many entrants the round starts with (8: quarter-finals).
	Round     int `json:"round"`
	Rounds    int `json:"rounds"`
	RoundSize int `json:"roundSize"`
	// PlaceFrom..PlaceTo are, in a bracket, the places its part of the
	// bracket plays for (1..4: semi-finals; 5..8: places 5 to 8).
	PlaceFrom int `json:"placeFrom"`
	PlaceTo   int `json:"placeTo"`
	// Match is nil in a stage not filled yet: who plays is not known.
	Match    *MatchView `json:"match"`
	Court    int        `json:"court"`
	Start    time.Time  `json:"start"`
	End      time.Time  `json:"end"`
	Unrested bool       `json:"unrested"`
	// OffRules: the court rules left no court for the match, so it is
	// placed against them.
	OffRules bool `json:"offRules"`
	// LeftA and LeftB count the matches each side has not started in the
	// stage (a waiting match counts itself).
	LeftA int `json:"leftA"`
	LeftB int `json:"leftB"`
}

// ScheduleBlockView is a block of the order set before validation: a round
// of a pool, or a bracket round, with its matches' estimated times.
type ScheduleBlockView struct {
	Key string `json:"key"`
	CategoryRef
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	Acronym    string               `json:"acronym"`
	Color      int                  `json:"color"`
	Pool       string               `json:"pool"`
	// Group is the key of its pool or bracket stage.
	Group     string `json:"group"`
	Round     int    `json:"round"`
	Rounds    int    `json:"rounds"`
	RoundSize int    `json:"roundSize"`
	// PlaceFrom..PlaceTo are, in a bracket, the places its part plays for.
	PlaceFrom int `json:"placeFrom"`
	PlaceTo   int `json:"placeTo"`
	// Pending: its stage is not filled yet, who plays is not known.
	Pending  bool      `json:"pending"`
	Matches  int       `json:"matches"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Unrested int       `json:"unrested"`
}

// ScheduleTeamView is an entrant with the matches it has left in a stage.
type ScheduleTeamView struct {
	Entrant engine.EntrantID `json:"entrant"`
	Bib     int              `json:"bib"`
	Names   []string         `json:"names"`
	Left    int              `json:"left"`
	Playing bool             `json:"playing"`
}

// ScheduleGroupView is a pool, or a bracket, of a stage still playing.
type ScheduleGroupView struct {
	Key string `json:"key"`
	CategoryRef
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	Acronym    string               `json:"acronym"`
	Color      int                  `json:"color"`
	Pool       string               `json:"pool"`
	Teams      []ScheduleTeamView   `json:"teams"`
}

// ScheduleHighlightView is a pool or a bracket stage the desk can highlight
// on the schedule: its matches and blocks carry its key as Group.
type ScheduleHighlightView struct {
	Key string `json:"key"`
	CategoryRef
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	// Acronym is the stage's short name ("" for the kind's default); Color
	// the pool's or bracket's color index.
	Acronym string `json:"acronym"`
	Color   int    `json:"color"`
	// Pool is "A", "B"… in a pools stage, "" for a bracket.
	Pool string `json:"pool"`
}

// CourtReservation is the category a court without an only rule plays in
// reserved mode, by the default share.
type CourtReservation struct {
	Number   int             `json:"number"`
	Category engine.Category `json:"category"`
}

// CourtScopeView is what a court rule targets, with what names it: a
// category, a stage, a pool or a bracket round.
type CourtScopeView struct {
	Scope tournament.CourtScope `json:"scope"`
	CategoryRef
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	// Acronym is the stage's short name typed by the desk ("" for the
	// kind's default); Color the pool's or bracket's color index (0 for a
	// category or a pools stage).
	Acronym string `json:"acronym"`
	Color   int    `json:"color"`
	// PlaceTo is, for a bracket round, the last place its part plays for.
	PlaceTo int `json:"placeTo"`
	// Left: the scope still has matches to play; a rule ends with it.
	Left bool `json:"left"`
}

// CourtRuleView is a rule of a court.
type CourtRuleView struct {
	Kind  tournament.CourtRuleKind `json:"kind"`
	Level int                      `json:"level"`
	Scope CourtScopeView           `json:"scope"`
}

// CourtRulesView is a court with its rules: only rules, priorities by
// level, never rules.
type CourtRulesView struct {
	Number int             `json:"number"`
	Rules  []CourtRuleView `json:"rules"`
}

// ScheduleView is the schedule screen: the forecast of every match left.
type ScheduleView struct {
	Now        time.Time               `json:"now"`
	Mode       tournament.ScheduleMode `json:"mode"`
	First      engine.Category         `json:"first"`
	Categories []CategoryRef           `json:"categories"`
	Courts     int                     `json:"courts"`
	MinRest    int                     `json:"minRest"`
	Reserved   []CourtReservation      `json:"reserved"`
	// CourtRules lists every court with its rules (ADR 12); Scopes what a
	// rule can target, category by category.
	CourtRules []CourtRulesView `json:"courtRules"`
	Scopes     []CourtScopeView `json:"scopes"`
	// Slots is the timeline: matches played and playing, then waiting
	// matches and stand-ins of stages not filled yet, by start time.
	Slots []ScheduleSlotView `json:"slots"`
	// ValidatedAt is when the desk validated the order; nil while it follows
	// the blocks. Blocks move before validation, matches after.
	ValidatedAt *time.Time `json:"validatedAt"`
	// Blocks is the block order, the first played first: what the desk
	// reorders before validation. Blocks of stages not filled yet are
	// pending.
	Blocks []ScheduleBlockView `json:"blocks"`
	// Groups lists the pools and brackets still playing, with the matches
	// each team has left.
	Groups []ScheduleGroupView `json:"groups"`
	// Highlights lists every pool and bracket stage, stand-ins of stages
	// not filled yet included: the desk highlights their matches on the
	// timeline and their blocks (nothing is stored).
	Highlights []ScheduleHighlightView `json:"highlights"`
	// End is when the last match is expected to end; nil with nothing left.
	End      *time.Time `json:"end"`
	Unrested int        `json:"unrested"`
	// OffRules counts the matches placed against the court rules: no court
	// they allow could take them.
	OffRules int `json:"offRules"`
	// Start is the tournament's start when the forecast starts there (no
	// match called yet, start still ahead); nil when it starts now.
	Start *time.Time `json:"start"`
	// DayEnd is the tournament's end hour, nil when not set; Late: the
	// forecast ends after it.
	DayEnd *time.Time `json:"dayEnd"`
	Late   bool       `json:"late"`
	// Breaks are the pauses of every court, earliest first.
	Breaks []BreakView `json:"breaks"`
	// Ghosts are the ghost matches still ahead: courts left empty in their
	// turn, in the order they were added.
	Ghosts []GhostMatchView `json:"ghosts"`
}

// GhostMatchView is a court left empty for a while, after a match (nil: as
// soon as the court is free), at its forecast times (nil when not placed).
type GhostMatchView struct {
	ID      string     `json:"id"`
	Court   int        `json:"court"`
	Minutes int        `json:"minutes"`
	After   *MatchView `json:"after"`
	Start   *time.Time `json:"start"`
	End     *time.Time `json:"end"`
}

// BreakView is a pause of every court on the tournament's day.
type BreakView struct {
	ID   string    `json:"id"`
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// courtRules builds every court's rules and what a rule can target.
func (b *scheduleBuilder) courtRules(rules map[int][]tournament.CourtRule) ([]CourtRulesView, []CourtScopeView) {
	left := map[tournament.CourtScope]bool{}
	scopes := []CourtScopeView{}
	for _, sc := range b.st.CourtScopes() {
		left[sc] = true
		scopes = append(scopes, b.scope(sc, true))
	}
	courts := []CourtRulesView{}
	for court := 1; court <= b.st.Courts(); court++ {
		cv := CourtRulesView{Number: court, Rules: []CourtRuleView{}}
		for _, r := range rules[court] {
			running, ok := left[r.Scope]
			if !ok {
				running = b.st.CourtScopeLeft(r.Scope)
			}
			cv.Rules = append(cv.Rules, CourtRuleView{Kind: r.Kind, Level: r.Level, Scope: b.scope(r.Scope, running)})
		}
		courts = append(courts, cv)
	}
	return courts, scopes
}

// scope names a court rule's scope.
func (b *scheduleBuilder) scope(sc tournament.CourtScope, left bool) CourtScopeView {
	v := CourtScopeView{Scope: sc, CategoryRef: CategoryRef{sc.Category, b.slotOf(sc.Category)}, Left: left}
	if sc.Stage == "" {
		return v
	}
	stage, _ := b.st.Stage(sc.Category, sc.Stage)
	v.StageName, v.StageKind, v.Acronym = stage.Name, stage.Kind, stage.Acronym
	v.EntryStage = b.st.Stages(sc.Category)[0].ID == sc.Stage
	switch {
	case sc.Pool != "":
		v.Color = b.styles[string(sc.Category)+"/"+sc.Stage+"/"+sc.Pool].Color
	case stage.Kind == tournament.StageBracket:
		v.Color = tournament.GroupStyleOf(b.styles, sc.Category, sc.Stage, -1).Color
	}
	if sc.Round > 0 {
		v.PlaceTo = sc.From + sc.Round - 1
	}
	return v
}
