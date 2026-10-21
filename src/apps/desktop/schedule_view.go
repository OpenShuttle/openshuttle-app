package main

import (
	"fmt"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// scheduleView builds the schedule screen of a state at a time.
func (s *TournamentService) scheduleView(st tournament.State, now time.Time) ScheduleView {
	settings := st.ScheduleSettings()
	sched := st.Schedule(now)
	b := scheduleBuilder{st: st, now: now, mv: s.matchViewer(st), refs: categoryRefs(st), left: map[string]map[engine.EntrantID]int{}}
	v := ScheduleView{
		Now: now, Mode: settings.Mode, First: settings.First, Categories: b.refs, Courts: st.Courts(),
		MinRest: st.MinRest(), Reserved: []CourtReservation{}, Slots: []ScheduleSlotView{},
		Blocks: []ScheduleBlockView{}, Groups: b.groups(),
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
			_, manual := settings.Courts[court]
			v.Reserved = append(v.Reserved, CourtReservation{Number: court, Category: c, Manual: manual})
		}
	}
	v.Slots = b.calledSlots()
	planned, blocks := b.plannedSlots(sched.Slots)
	v.Slots = append(v.Slots, planned...)
	v.Blocks = blocks
	for _, sl := range sched.Slots {
		if sl.Unrested {
			v.Unrested++
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
		PlaceFrom: sl.PlaceFrom, Unrested: sl.Unrested,
	}
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
		sv := b.slot(sl, SlotPlanned)
		out = append(out, sv)
		bv := byKey[sl.Key]
		if bv == nil {
			bv = &ScheduleBlockView{
				Key: sv.Key, CategoryRef: sv.CategoryRef, Stage: sv.Stage, StageName: sv.StageName, StageKind: sv.StageKind,
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
				if stage.Kind == tournament.StagePools {
					gv.Pool = engine.PoolName(g)
				}
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
	// Pool is "A", "B"… in a pools stage, "" in a bracket.
	Pool string `json:"pool"`
	// Round is the round of the pool or of the bracket, from 1. RoundSize is,
	// in a bracket, how many entrants the round starts with (8: quarter-finals).
	Round     int `json:"round"`
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
	Pool       string               `json:"pool"`
	Round      int                  `json:"round"`
	RoundSize  int                  `json:"roundSize"`
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
	Pool       string               `json:"pool"`
	Teams      []ScheduleTeamView   `json:"teams"`
}

// CourtReservation is the category a court plays in reserved mode.
type CourtReservation struct {
	Number   int             `json:"number"`
	Category engine.Category `json:"category"`
	// Manual: reserved by hand, else by the default share.
	Manual bool `json:"manual"`
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
	// End is when the last match is expected to end; nil with nothing left.
	End      *time.Time `json:"end"`
	Unrested int        `json:"unrested"`
}
