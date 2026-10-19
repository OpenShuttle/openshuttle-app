package main

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// ScheduleSlotView is a match of the schedule screen, on a court with its
// estimated times.
type ScheduleSlotView struct {
	Key string `json:"key"`
	CategoryRef
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	// Pool is "A", "B"… in a pools stage, "" in a bracket.
	Pool string `json:"pool"`
	// Round is the round of the pool or of the bracket, from 1.
	Round int `json:"round"`
	// Match is nil in a stage not filled yet: who plays is not known.
	Match    *MatchView `json:"match"`
	Court    int        `json:"court"`
	Start    time.Time  `json:"start"`
	End      time.Time  `json:"end"`
	Unrested bool       `json:"unrested"`
}

// ScheduleMatchView is a waiting match in schedule order, with the
// forecast and the matches each side has left in the stage.
type ScheduleMatchView struct {
	ScheduleSlotView
	// LeftA and LeftB count the matches each side has not started in the
	// stage, this one included.
	LeftA int `json:"leftA"`
	LeftB int `json:"leftB"`
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
	// Slots is the timeline: waiting matches and stand-ins of stages not
	// filled yet, by start time.
	Slots []ScheduleSlotView `json:"slots"`
	// ValidatedAt is when the desk validated the order; nil while it follows
	// the rules. Only a validated order can be changed by hand.
	ValidatedAt *time.Time `json:"validatedAt"`
	// Order lists the waiting matches in the order courts take them.
	Order []ScheduleMatchView `json:"order"`
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

// MoveScheduledPool moves every waiting match of the pool of a match just
// before another one ("" at the end).
func (s *TournamentService) MoveScheduledPool(ctx context.Context, of, before tournament.MatchID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) {
		m, ok := st.Match(of)
		if !ok || m.IsBracket() {
			return tournament.Change{}, engine.NewError(engine.CodeMatchNotFound)
		}
		return st.MoveScheduledPool(m.Category, m.Stage, m.Pool, before)
	})
}

// GetScheduleView returns the schedule screen.
func (s *TournamentService) GetScheduleView() ScheduleView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	now := s.now()
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
		Order: []ScheduleMatchView{}, Groups: []ScheduleGroupView{},
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
	slotView := func(slot tournament.ScheduleSlot) ScheduleSlotView {
		stage, _ := st.Stage(slot.Category, slot.Stage)
		sv := ScheduleSlotView{
			Key: slot.Key, CategoryRef: CategoryRef{slot.Category, slots[slot.Category]},
			Stage: slot.Stage, StageName: stage.Name, StageKind: stage.Kind,
			EntryStage: st.Stages(slot.Category)[0].ID == slot.Stage,
			Round:      slot.Level + 1, Court: slot.Court, Start: slot.Start, End: slot.End, Unrested: slot.Unrested,
		}
		if slot.Pool >= 0 {
			sv.Pool = engine.PoolName(slot.Pool)
		}
		if m, ok := st.Match(slot.Match); ok && slot.Match != "" {
			view := mv(m)
			sv.Match = &view
		}
		return sv
	}
	planned := map[tournament.MatchID]tournament.ScheduleSlot{}
	for _, slot := range sched.Slots {
		if slot.Unrested {
			v.Unrested++
		}
		v.Slots = append(v.Slots, slotView(slot))
		if slot.Match != "" {
			planned[slot.Match] = slot
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
	for _, id := range st.ScheduleOrder() {
		m, _ := st.Match(id)
		slot, ok := planned[id]
		if !ok {
			// Not placed by the forecast (its court never frees up for it):
			// listed without times.
			slot = tournament.ScheduleSlot{Category: m.Category, Stage: m.Stage, Pool: m.Pool, Level: m.Round, Match: id, A: m.A, B: m.B}
		}
		l := leftIn(m.Category, m.Stage)
		v.Order = append(v.Order, ScheduleMatchView{ScheduleSlotView: slotView(slot), LeftA: l[m.A], LeftB: l[m.B]})
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
