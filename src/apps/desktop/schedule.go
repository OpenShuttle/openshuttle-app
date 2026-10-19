package main

import (
	"context"
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

// ScheduleBlockView is a block the desk can move up: a pool, or a round of
// a bracket, with when its matches are expected.
type ScheduleBlockView struct {
	Key string `json:"key"`
	CategoryRef
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	Pool       string               `json:"pool"`
	Round      int                  `json:"round"`
	Matches    int                  `json:"matches"`
	Start      time.Time            `json:"start"`
	End        time.Time            `json:"end"`
	Pinned     int                  `json:"pinned"` // position among the pinned blocks, from 1; 0 if not pinned
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
	Slots      []ScheduleSlotView      `json:"slots"`
	// Blocks in the order they start.
	Blocks []ScheduleBlockView `json:"blocks"`
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

// SetSchedulePins sets the blocks played first, in order.
func (s *TournamentService) SetSchedulePins(ctx context.Context, keys []string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetSchedulePins(keys) })
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
		MinRest: st.MinRest(), Reserved: []CourtReservation{}, Slots: []ScheduleSlotView{}, Blocks: []ScheduleBlockView{},
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
	blocks := map[string]int{}
	for _, slot := range sched.Slots {
		stage, _ := st.Stage(slot.Category, slot.Stage)
		entry := st.Stages(slot.Category)[0].ID == slot.Stage
		sv := ScheduleSlotView{
			Key: slot.Key, CategoryRef: CategoryRef{slot.Category, slots[slot.Category]},
			Stage: slot.Stage, StageName: stage.Name, StageKind: stage.Kind, EntryStage: entry,
			Round: slot.Level + 1, Court: slot.Court, Start: slot.Start, End: slot.End, Unrested: slot.Unrested,
		}
		if slot.Pool >= 0 {
			sv.Pool = engine.PoolName(slot.Pool)
		}
		if m, ok := st.Match(slot.Match); ok && slot.Match != "" {
			view := mv(m)
			sv.Match = &view
		}
		if sv.Unrested {
			v.Unrested++
		}
		v.Slots = append(v.Slots, sv)

		i, ok := blocks[slot.Key]
		if !ok {
			i = len(v.Blocks)
			blocks[slot.Key] = i
			b := ScheduleBlockView{
				Key: slot.Key, CategoryRef: sv.CategoryRef, Stage: sv.Stage, StageName: sv.StageName,
				StageKind: sv.StageKind, EntryStage: entry, Pool: sv.Pool, Start: slot.Start, End: slot.End,
			}
			if slot.Pool < 0 {
				b.Round = sv.Round
			}
			for p, key := range settings.Pinned {
				if key == slot.Key {
					b.Pinned = p + 1
				}
			}
			v.Blocks = append(v.Blocks, b)
		}
		b := &v.Blocks[i]
		b.Matches++
		if slot.End.After(b.End) {
			b.End = slot.End
		}
	}
	if !sched.End.IsZero() {
		end := sched.End
		v.End = &end
	}
	return v
}
