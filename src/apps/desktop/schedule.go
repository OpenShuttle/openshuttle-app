package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

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
	Matches    int                  `json:"matches"`
	Start      time.Time            `json:"start"`
	End        time.Time            `json:"end"`
	Unrested   int                  `json:"unrested"`
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
	// reorders before validation.
	Blocks []ScheduleBlockView `json:"blocks"`
	// Groups lists the pools and brackets still playing, with the matches
	// each team has left.
	Groups []ScheduleGroupView `json:"groups"`
	// End is when the last match is expected to end; nil with nothing left.
	End      *time.Time `json:"end"`
	Unrested int        `json:"unrested"`
}

// SetScheduleMode chooses how two categories share the courts: "alternate"
// or "reserved".
func (s *TournamentService) SetScheduleMode(ctx context.Context, mode tournament.ScheduleMode) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetScheduleMode(mode) })
}

// SetScheduleFirst chooses the category opening each round in alternate mode.
func (s *TournamentService) SetScheduleFirst(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetScheduleFirst(c) })
}

// SetCourtCategory reserves a court for a category ("" for the default share).
func (s *TournamentService) SetCourtCategory(ctx context.Context, court int, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetCourtCategory(court, c) })
}

// ValidateSchedule fixes the order of the waiting matches from the rules
// (again: recomputes it, dropping the moves made by hand).
func (s *TournamentService) ValidateSchedule(ctx context.Context) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ValidateSchedule() })
}

// MoveScheduledMatch moves a match just before another one ("" at the end).
func (s *TournamentService) MoveScheduledMatch(ctx context.Context, id, before tournament.MatchID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.MoveScheduledMatch(id, before) })
}

// MoveScheduledBlock moves a block (pool or bracket round) just before
// another one ("" at the end), before the schedule is validated.
func (s *TournamentService) MoveScheduledBlock(ctx context.Context, key, before string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.MoveScheduledBlock(key, before) })
}

// UnvalidateSchedule goes back to moving blocks, dropping the matches moved
// by hand.
func (s *TournamentService) UnvalidateSchedule(ctx context.Context) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.UnvalidateSchedule() })
}

// GetScheduleView returns the schedule screen.
func (s *TournamentService) GetScheduleView() ScheduleView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scheduleView(s.state, s.now())
}

// PreviewScheduledBlockMove returns the schedule screen as it would be with
// a block moved just before another one: what the desk sees while dragging.
// Nothing is saved.
func (s *TournamentService) PreviewScheduledBlockMove(key, before string) ScheduleView {
	return s.previewSchedule(func(st tournament.State) (tournament.Change, error) { return st.MoveScheduledBlock(key, before) })
}

// PreviewScheduledMatchMove returns the schedule screen as it would be with a
// match moved just before another one. Nothing is saved.
func (s *TournamentService) PreviewScheduledMatchMove(id, before tournament.MatchID) ScheduleView {
	return s.previewSchedule(func(st tournament.State) (tournament.Change, error) { return st.MoveScheduledMatch(id, before) })
}

// previewSchedule projects the log with the command's event added, in memory
// only. A refused or empty move previews the schedule as it is.
func (s *TournamentService) previewSchedule(cmd func(tournament.State) (tournament.Change, error)) ScheduleView {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	st := s.state
	if s.file == nil {
		return s.scheduleView(st, now)
	}
	if ch, err := cmd(st); err == nil && !ch.None() {
		if d, err := events.NewDraft(ch.Type, ch.Data, now, ""); err == nil {
			log := append(slices.Clone(s.log), events.Event{Seq: st.Version + 1, Type: d.Type, Data: d.Data, At: d.At})
			if p, err := tournament.Project(log); err == nil {
				st = p
			}
		}
	}
	return s.scheduleView(st, now)
}

func (s *TournamentService) scheduleView(st tournament.State, now time.Time) ScheduleView {
	settings := st.ScheduleSettings()
	sched := st.Schedule(now)
	mv := s.matchViewer(st)
	refs := categoryRefs(st)
	slots := map[engine.Category]int{}
	for _, r := range refs {
		slots[r.Category] = r.Slot
	}

	v := ScheduleView{
		Now: now, Mode: settings.Mode, First: settings.First, Categories: refs, Courts: st.Courts(),
		MinRest: st.MinRest(), Reserved: []CourtReservation{}, Slots: []ScheduleSlotView{},
		Blocks: []ScheduleBlockView{}, Groups: []ScheduleGroupView{},
	}
	if at := st.ScheduleValidatedAt(); !at.IsZero() {
		v.ValidatedAt = &at
	}
	if v.First == "" && len(refs) > 0 {
		v.First = refs[0].Category
	}
	for court := 1; court <= st.Courts(); court++ {
		if c, ok := sched.Reserved[court]; ok {
			_, manual := settings.Courts[court]
			v.Reserved = append(v.Reserved, CourtReservation{Number: court, Category: c, Manual: manual})
		}
	}
	left := map[string]map[engine.EntrantID]int{}
	leftIn := func(c engine.Category, stage string) map[engine.EntrantID]int {
		k := string(c) + "/" + stage
		if left[k] == nil {
			left[k] = st.MatchesLeft(c, stage)
		}
		return left[k]
	}
	validated := v.ValidatedAt != nil
	slotView := func(slot tournament.ScheduleSlot, state string) ScheduleSlotView {
		stage, _ := st.Stage(slot.Category, slot.Stage)
		sv := ScheduleSlotView{
			Key: slot.Key, State: state, CategoryRef: CategoryRef{slot.Category, slots[slot.Category]},
			Stage: slot.Stage, StageName: stage.Name, StageKind: stage.Kind,
			EntryStage: st.Stages(slot.Category)[0].ID == slot.Stage,
			Round:      slot.Level + 1, RoundSize: slot.RoundSize, Court: slot.Court, Start: slot.Start, End: slot.End,
			Unrested: slot.Unrested,
		}
		if slot.Pool >= 0 {
			sv.Pool = engine.PoolName(slot.Pool)
		}
		if m, ok := st.Match(slot.Match); ok && slot.Match != "" {
			view := mv(m)
			sv.Match = &view
			l := leftIn(m.Category, m.Stage)
			sv.LeftA, sv.LeftB = l[m.A], l[m.B]
		}
		// Matches move one by one once validated; before, blocks move in the
		// block list.
		sv.Movable = state == SlotPlanned && validated && slot.Match != ""
		return sv
	}
	// Matches played or playing, at their real court and times.
	for _, m := range st.Matches() {
		if !m.Called() || st.Walkover(m) {
			continue
		}
		slot := tournament.ScheduleSlot{
			Key: st.MatchBlock(m), Category: m.Category, Stage: m.Stage, Pool: m.Pool, Level: m.Round,
			Match: m.ID, A: m.A, B: m.B, Court: m.Court, Start: m.CalledAt, End: m.EndedAt, RoundSize: m.Size,
		}
		state := SlotPlayed
		if m.IsBracket() {
			slot.Level = st.BracketLevel(m)
		}
		if !m.Ended() {
			state = SlotPlaying
			slot.End = later(now, m.CalledAt.Add(time.Duration(st.WinCondition(m).EstimatedMinutes())*time.Minute))
		}
		v.Slots = append(v.Slots, slotView(slot, state))
	}
	blocks := map[string]*ScheduleBlockView{}
	for _, slot := range sched.Slots {
		if slot.Unrested {
			v.Unrested++
		}
		sv := slotView(slot, SlotPlanned)
		v.Slots = append(v.Slots, sv)
		if slot.Match == "" {
			continue // a stand-in: its stage is not filled yet
		}
		b := blocks[slot.Key]
		if b == nil {
			b = &ScheduleBlockView{
				Key: sv.Key, CategoryRef: sv.CategoryRef, Stage: sv.Stage, StageName: sv.StageName, StageKind: sv.StageKind,
				EntryStage: sv.EntryStage, Pool: sv.Pool, Round: sv.Round, RoundSize: sv.RoundSize, Start: slot.Start, End: slot.End,
			}
			blocks[slot.Key] = b
		}
		b.Matches++
		if slot.Start.Before(b.Start) {
			b.Start = slot.Start
		}
		b.End = later(b.End, slot.End)
		if slot.Unrested {
			b.Unrested++
		}
	}
	for _, k := range st.ScheduleBlocks() {
		if b := blocks[k]; b != nil {
			v.Blocks = append(v.Blocks, *b)
		}
	}
	// Teams with matches left, pool by pool (a bracket as one group).
	side := sideViewer(st)
	playing := map[engine.EntrantID]bool{}
	for _, m := range st.Matches() {
		if st.Playing(m) {
			playing[m.A], playing[m.B] = true, true
		}
	}
	for _, ref := range refs {
		for i, stage := range st.Stages(ref.Category) {
			if !stage.Confirmed {
				continue
			}
			l := leftIn(ref.Category, stage.ID)
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
					v.Groups = append(v.Groups, gv)
				}
			}
		}
	}
	if !sched.End.IsZero() {
		end := sched.End
		v.End = &end
	}
	return v
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
