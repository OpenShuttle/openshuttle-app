package tournament

import (
	"fmt"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeStageFilled  events.Type = "StageFilled"
	TypeSlotsSwapped events.Type = "SlotsSwapped"
)

// StageFilled is the payload of TypeStageFilled: the entrants of a later
// stage, from the ranks routed to it. Groups are pools, or a single group of
// bracket slots ("" is a bye). The stage starts at once.
type StageFilled struct {
	Category engine.Category      `json:"category"`
	Stage    string               `json:"stage"`
	Groups   [][]engine.EntrantID `json:"groups"`
}

// SlotsSwapped is the payload of TypeSlotsSwapped: two first-round slots of a
// bracket stage trade places.
type SlotsSwapped struct {
	Category engine.Category `json:"category"`
	Stage    string          `json:"stage"`
	A        int             `json:"a"`
	B        int             `json:"b"`
}

// RankGroup is the entrants sharing a rank of a stage: one per pool at a pool
// position, or the entrants of a bracket place (several when tied).
type RankGroup struct {
	Rank     int
	Entrants []engine.EntrantID
	// Decided is set once the rank is final in this stage.
	Decided bool
}

// StageRanks returns the ranks of a filled stage, best first. Entrants whose
// bracket place is not known yet come last with Rank 0.
func (s State) StageRanks(c engine.Category, id string) []RankGroup {
	st := s.stage(c, id)
	if st == nil || !st.Confirmed {
		return nil
	}
	if st.Kind == StagePools {
		decided := s.stageComplete(c, st)
		var out []RankGroup
		for p := range st.Groups {
			for i, row := range s.PoolStandings(c, id, p).Rows {
				for len(out) <= i {
					out = append(out, RankGroup{Rank: len(out) + 1, Decided: decided})
				}
				out[i].Entrants = append(out[i].Entrants, row.Entrant)
			}
		}
		return out
	}
	byRank := map[int]*RankGroup{}
	var ranks []int
	placed := map[engine.EntrantID]bool{}
	for _, p := range s.Places(c, id) {
		if byRank[p.From] == nil {
			byRank[p.From] = &RankGroup{Rank: p.From, Decided: true}
			ranks = append(ranks, p.From)
		}
		byRank[p.From].Entrants = append(byRank[p.From].Entrants, p.Entrant)
		placed[p.Entrant] = true
	}
	slices.Sort(ranks)
	var out []RankGroup
	for _, r := range ranks {
		out = append(out, *byRank[r])
	}
	var pending []engine.EntrantID
	for _, e := range st.Slots() {
		if e != "" && !placed[e] {
			pending = append(pending, e)
		}
	}
	if len(pending) > 0 {
		out = append(out, RankGroup{Entrants: pending})
	}
	return out
}

// stageComplete reports whether every match of a confirmed stage is decided
// and, for pools, no tie waits for lots.
func (s State) stageComplete(c engine.Category, st *Stage) bool {
	if !st.Confirmed {
		return false
	}
	for _, m := range s.matches {
		if m.Category == c && m.Stage == st.ID && !m.ByeA && !m.ByeB && !s.Decided(m) {
			// A bracket match waiting for a side is not decided either.
			return false
		}
	}
	if st.Kind == StagePools {
		for p := range st.Groups {
			for _, row := range s.PoolStandings(c, st.ID, p).Rows {
				if row.DrawNeeded {
					return false
				}
			}
		}
	}
	return true
}

// StageComplete reports whether a stage is over: its ranks are final.
func (s State) StageComplete(c engine.Category, id string) bool {
	st := s.stage(c, id)
	return st != nil && s.stageComplete(c, st)
}

// Routed returns the entrants routed to a stage, from the current ranks of
// the stages feeding it: Rank is their rank there, Pool tells their group
// apart (stage position and pool), so pool mates are kept apart. Forfeited
// entrants are left out.
func (s State) Routed(c engine.Category, id string) []engine.BracketEntrant {
	target := s.stageIndex(c, id)
	var out []engine.BracketEntrant
	for si, src := range s.formats[c] {
		if si >= target || !slices.Contains(src.Routing, id) || !src.Confirmed {
			continue
		}
		for _, g := range s.StageRanks(c, src.ID) {
			if g.Rank == 0 || src.RouteOf(g.Rank) != id {
				continue
			}
			for _, e := range g.Entrants {
				if s.withdrawnBy(e, c, si) {
					continue
				}
				out = append(out, engine.BracketEntrant{ID: e, Rank: g.Rank, Pool: si*MaxPools + s.groupOf(src, e)})
			}
		}
	}
	return out
}

func (s State) groupOf(st *Stage, id engine.EntrantID) int {
	for g, group := range st.Groups {
		if slices.Contains(group, id) {
			return g
		}
	}
	return 0
}

// CanFill reports whether a later stage can be filled: every stage feeding
// it is over. FillStage says why not.
func (s State) CanFill(c engine.Category, id string) bool {
	_, err := s.FillStage(c, id)
	return err == nil
}

// FillStage fills a later stage from the ranks routed to it. Refilling is
// possible until a match of the stage starts.
func (s State) FillStage(c engine.Category, id string) (Change, error) {
	i := s.stageIndex(c, id)
	switch {
	case i < 0:
		return Change{}, engine.NewError(engine.CodeStageNotFound)
	case i == 0:
		// The entry stage is filled by the draw.
		return Change{}, engine.NewError(engine.CodeInvalidFormat)
	case s.stageStarted(c, id):
		return Change{}, engine.NewError(engine.CodeBracketStarted)
	}
	sources := s.Sources(c, id)
	if len(sources) == 0 {
		return Change{}, engine.NewError(engine.CodeNotEnoughEntrants)
	}
	for _, src := range sources {
		st := s.stage(c, src)
		if !st.Confirmed || !s.stageComplete(c, st) {
			for p := range st.Groups {
				for _, row := range s.PoolStandings(c, src, p).Rows {
					if row.DrawNeeded {
						return Change{}, engine.NewError(engine.CodeTieUnresolved)
					}
				}
			}
			return Change{}, engine.NewError(engine.CodePoolsNotComplete)
		}
	}
	routed := s.Routed(c, id)
	st := s.formats[c][i]
	var groups [][]engine.EntrantID
	if st.Kind == StagePools {
		if len(routed) < 2*st.Pools {
			return Change{}, engine.NewError(engine.CodeNotEnoughEntrants)
		}
		groups = engine.FillPools(routed, st.Pools)
	} else {
		if len(routed) < 2 {
			return Change{}, engine.NewError(engine.CodeNotEnoughEntrants)
		}
		groups = [][]engine.EntrantID{engine.SeedBracket(routed)}
	}
	return Change{TypeStageFilled, StageFilled{Category: c, Stage: id, Groups: groups}}, nil
}

// SwapSlots swaps two first-round slots of a bracket stage, to correct the
// seeding before the bracket starts.
func (s State) SwapSlots(c engine.Category, id string, a, b int) (Change, error) {
	st := s.stage(c, id)
	slots := []engine.EntrantID(nil)
	if st != nil {
		slots = st.Slots()
	}
	switch {
	case st == nil:
		return Change{}, engine.NewError(engine.CodeStageNotFound)
	case slots == nil:
		return Change{}, engine.NewError(engine.CodeBracketNotDrawn)
	case a < 0 || b < 0 || a >= len(slots) || b >= len(slots):
		return Change{}, engine.NewError(engine.CodeInvalidSlot)
	case s.stageStarted(c, id):
		return Change{}, engine.NewError(engine.CodeBracketStarted)
	case a == b:
		return Change{}, nil
	}
	return Change{TypeSlotsSwapped, SlotsSwapped{Category: c, Stage: id, A: a, B: b}}, nil
}

func (s *State) applyStageFilled(p StageFilled, at time.Time) {
	st := s.stage(p.Category, p.Stage)
	if st == nil {
		return
	}
	st.Groups = cloneGroups(p.Groups)
	st.Confirmed, st.ConfirmedAt = true, at
	s.generateStageMatches(p.Category, st)
}

func (s *State) applySlotsSwapped(p SlotsSwapped) {
	st := s.stage(p.Category, p.Stage)
	if st == nil {
		return
	}
	slots := st.Slots()
	if p.A < len(slots) && p.B < len(slots) {
		slots[p.A], slots[p.B] = slots[p.B], slots[p.A]
		if st.Confirmed {
			s.generateStageMatches(p.Category, st)
		}
	}
}

// generateStageMatches builds the matches of a stage from its groups: a
// round-robin per pool (rounds interleaved across pools), or the bracket
// tree. Matches keep the order of the format: courts take earlier stages
// first, whatever order stages were filled in.
func (s *State) generateStageMatches(c engine.Category, st *Stage) {
	s.dropStageMatches(c, st.ID)
	prefix := string(c) + "-"
	if st != s.entry(c) {
		prefix += st.ID + "-"
	}
	if st.Kind == StageBracket {
		s.generateBracketMatches(c, st, prefix)
	} else {
		s.generatePoolMatches(c, st, prefix)
	}
	order := func(m Match) int {
		if m.Category != c {
			return 0
		}
		return s.stageIndex(c, m.Stage)
	}
	slices.SortStableFunc(s.matches, func(a, b Match) int {
		if a.Category != c || b.Category != c {
			return 0
		}
		return order(a) - order(b)
	})
}

func (s *State) generatePoolMatches(c engine.Category, st *Stage, prefix string) {
	schedules := make([][][][2]int, len(st.Groups))
	maxRounds := 0
	for p, pool := range st.Groups {
		schedules[p] = engine.RoundRobinSchedule(len(pool))
		maxRounds = max(maxRounds, len(schedules[p]))
	}
	counts := make([]int, len(st.Groups))
	for r := range maxRounds {
		for p, pool := range st.Groups {
			if r >= len(schedules[p]) {
				continue
			}
			for _, pair := range schedules[p][r] {
				counts[p]++
				s.matches = append(s.matches, Match{
					ID:       MatchID(fmt.Sprintf("%s%s%d", prefix, engine.PoolName(p), counts[p])),
					Category: c,
					Stage:    st.ID,
					Pool:     p,
					Round:    r,
					A:        pool[pair[0]],
					B:        pool[pair[1]],
				})
			}
		}
	}
}

func (s *State) dropStageMatches(c engine.Category, id string) {
	s.matches = slices.DeleteFunc(s.matches, func(m Match) bool { return m.Category == c && m.Stage == id })
}

func cloneGroups(groups [][]engine.EntrantID) [][]engine.EntrantID {
	out := make([][]engine.EntrantID, len(groups))
	for i, g := range groups {
		out[i] = slices.Clone(g)
		if out[i] == nil {
			out[i] = []engine.EntrantID{}
		}
	}
	return out
}
