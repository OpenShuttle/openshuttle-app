package tournament

import (
	"math/bits"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament/blocks"
)

// AutoOrder is the order of the waiting matches before the schedule is
// validated: the matches of each block, in block order (ScheduleBlocks).
func (s State) AutoOrder() []MatchID {
	var out []MatchID
	for _, b := range s.blockOrder() {
		for _, id := range b.Matches {
			out = append(out, MatchID(id))
		}
	}
	return out
}

// ScheduleBlocks lists the blocks with waiting matches (a round of a pool,
// a bracket round), in the order they are played.
func (s State) ScheduleBlocks() []string {
	var out []string
	for _, b := range s.blockOrder() {
		out = append(out, b.Key)
	}
	return out
}

// blockOrder is the order of the blocks (blocks.Order): by default in waves,
// per category, phase after phase; once the desk moved a block, its order.
func (s State) blockOrder() []blocks.Block {
	return blocks.Order(s.categoryWaves(), s.openingOrder(), s.scheduleBlocks)
}

// categoryWaves cuts the waiting matches of each category into waves, phase
// after phase. Stages of the same phase (a main bracket and pools for the
// others, say) play side by side: wave k holds round k of each. Stages not
// filled yet bring the blocks of their stand-ins (no matches yet), so the
// desk orders them too.
func (s State) categoryWaves() map[engine.Category][][]blocks.Block {
	waves := map[engine.Category][][]blocks.Block{}
	standIns := s.standInWaves()
	for _, c := range s.Categories {
		phases := Phases(s.StageConfigs(c))
		byPhase := map[int][][]blocks.Block{}
		var order []int
		for _, st := range s.Stages(c) {
			p := phases[st.ID]
			stage := standIns[string(c)+"/"+st.ID]
			if st.Confirmed {
				stage = s.stageWaves(c, st)
			}
			if len(stage) == 0 {
				continue
			}
			if _, ok := byPhase[p]; !ok {
				order = append(order, p)
			}
			for k, w := range stage {
				for len(byPhase[p]) <= k {
					byPhase[p] = append(byPhase[p], nil)
				}
				byPhase[p][k] = append(byPhase[p][k], w...)
			}
		}
		slices.Sort(order)
		for _, p := range order {
			waves[c] = append(waves[c], byPhase[p]...)
		}
	}
	return waves
}

// stageWaves cuts the waiting matches of a filled stage into waves: in
// pools, wave k holds the k-th round left of every pool, the pools with the
// most matches first (pool order on a tie); in a bracket, wave k is round k,
// each part of the draw (the main draw, places 5 to 8…) a block, the main
// draw first.
func (s State) stageWaves(c engine.Category, st Stage) [][]blocks.Block {
	waiting := func(m Match) bool {
		return m.Category == c && m.Stage == st.ID && !m.ByeA && !m.ByeB && !m.Called() && !s.Decided(m)
	}
	if st.Kind == StageBracket {
		var waves [][]blocks.Block
		for _, m := range s.matches {
			if !waiting(m) {
				continue
			}
			l := s.bracketLevel(m)
			for len(waves) <= l {
				waves = append(waves, nil)
			}
			key := blocks.BracketKey(c, st.ID, l, m.Start)
			i := slices.IndexFunc(waves[l], func(b blocks.Block) bool { return b.Key == key })
			if i < 0 {
				waves[l] = append(waves[l], blocks.Block{Key: key, Group: key, Series: string(c) + "/" + st.ID, Round: l, Place: m.Start})
				i = len(waves[l]) - 1
			}
			waves[l][i].Matches = append(waves[l][i].Matches, string(m.ID))
		}
		blocks.SortByPlace(waves)
		return slices.DeleteFunc(waves, func(w []blocks.Block) bool { return len(w) == 0 })
	}
	total := map[int]int{}
	rounds := map[int]map[int][]string{} // pool → round → matches
	for _, m := range s.matches {
		if m.Category != c || m.Stage != st.ID || m.ByeA || m.ByeB {
			continue
		}
		total[m.Pool]++
		if !waiting(m) {
			continue
		}
		if rounds[m.Pool] == nil {
			rounds[m.Pool] = map[int][]string{}
		}
		rounds[m.Pool][m.Round] = append(rounds[m.Pool][m.Round], string(m.ID))
	}
	pools := make([]int, 0, len(rounds))
	for p := range rounds {
		pools = append(pools, p)
	}
	slices.SortFunc(pools, func(a, b int) int {
		if total[a] != total[b] {
			return total[b] - total[a]
		}
		return a - b
	})
	left := map[int][]int{} // pool → its rounds left, in order
	for _, p := range pools {
		for r := range rounds[p] {
			left[p] = append(left[p], r)
		}
		slices.Sort(left[p])
	}
	var waves [][]blocks.Block
	for k := 0; ; k++ {
		var wave []blocks.Block
		for _, p := range pools {
			if k < len(left[p]) {
				r := left[p][k]
				group := blocks.PoolGroup(c, st.ID, p)
				wave = append(wave, blocks.Block{Key: blocks.PoolKey(c, st.ID, p, r), Group: group, Series: group, Round: r, Matches: rounds[p][r]})
			}
		}
		if wave == nil {
			return waves
		}
		waves = append(waves, wave)
	}
}

// standInWaves cuts the stand-ins of each stage not filled yet into waves,
// by "<cat>/<stage>": wave k holds round k of every pool, or bracket round
// k (the main draw first). Blocks hold no matches.
func (s State) standInWaves() map[string][][]blocks.Block {
	out := map[string][][]blocks.Block{}
	seen := map[string]bool{}
	for _, it := range s.scheduleItems() {
		if it.Match != "" || seen[it.Key] {
			continue
		}
		seen[it.Key] = true
		k := string(it.Category) + "/" + it.Stage
		for len(out[k]) <= it.Level {
			out[k] = append(out[k], nil)
		}
		b := blocks.Block{Key: it.Key, Group: it.Key, Series: k, Round: it.Level, Place: it.PlaceFrom}
		if it.Pool >= 0 {
			b.Group = blocks.PoolGroup(it.Category, it.Stage, it.Pool)
			b.Series, b.Place = b.Group, it.Pool
		}
		out[k][it.Level] = append(out[k][it.Level], b)
	}
	for _, waves := range out {
		blocks.SortByPlace(waves)
	}
	return out
}

// ScheduleOrder is the order of the waiting matches: the validated order
// (matches added since, by stages filled later, follow in rule order), or
// the rule order while the schedule is not validated.
func (s State) ScheduleOrder() []MatchID {
	auto := s.AutoOrder()
	if s.scheduleFrozenAt.IsZero() {
		return auto
	}
	waiting := map[MatchID]bool{}
	for _, id := range auto {
		waiting[id] = true
	}
	var out []MatchID
	for _, id := range s.scheduleOrder {
		if waiting[id] {
			out = append(out, id)
			delete(waiting, id)
		}
	}
	for _, id := range auto {
		if waiting[id] {
			out = append(out, id)
		}
	}
	return out
}

// ScheduleValidatedAt is when the desk validated the schedule order, zero
// while the order follows the rules.
func (s State) ScheduleValidatedAt() time.Time { return s.scheduleFrozenAt }

// ValidateSchedule fixes the order of the waiting matches: the rule order
// now (validating again recomputes it, dropping the moves made by hand).
// Later, matches only move when the desk moves them.
func (s State) ValidateSchedule() (Change, error) {
	order := s.AutoOrder()
	if len(order) == 0 {
		return Change{}, engine.NewError(engine.CodeNothingToSchedule)
	}
	return Change{TypeScheduleOrderSet, ScheduleOrderSet{Order: order}}, nil
}

// MoveScheduledMatch moves a waiting match just before another one ("" at
// the end) in the validated order: once validated, matches move one by one.
func (s State) MoveScheduledMatch(id, before MatchID) (Change, error) {
	if s.scheduleFrozenAt.IsZero() {
		return Change{}, engine.NewError(engine.CodeScheduleNotValidated)
	}
	order := s.ScheduleOrder()
	if !slices.Contains(order, id) {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	if id == before {
		return Change{}, nil // dropped on itself: nothing moves
	}
	rest := slices.DeleteFunc(slices.Clone(order), func(m MatchID) bool { return m == id })
	at := len(rest)
	if before != "" {
		if at = slices.Index(rest, before); at < 0 {
			return Change{}, engine.NewError(engine.CodeMatchNotFound)
		}
	}
	next := slices.Insert(rest, at, id)
	if slices.Equal(next, order) {
		return Change{}, nil
	}
	return Change{TypeScheduleOrderSet, ScheduleOrderSet{Order: next}}, nil
}

// MoveScheduledBlock moves a block (a round of a pool, a bracket round)
// just before another one ("" at the end), before the schedule is
// validated. The rounds of a pool, or of a bracket, keep their order: the
// places they hold go to them first round first.
func (s State) MoveScheduledBlock(key, before string) (Change, error) {
	if !s.scheduleFrozenAt.IsZero() {
		return Change{}, engine.NewError(engine.CodeScheduleValidated)
	}
	order := s.blockOrder()
	next, ok := blocks.Move(order, key, before)
	if !ok {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	if key == before || slices.Equal(next, s.ScheduleBlocks()) {
		return Change{}, nil
	}
	return Change{TypeScheduleBlocksSet, ScheduleBlocksSet{Order: next}}, nil
}

// UnvalidateSchedule drops the validated order (and the matches moved by
// hand): the order follows the blocks again.
func (s State) UnvalidateSchedule() (Change, error) {
	if s.scheduleFrozenAt.IsZero() {
		return Change{}, nil
	}
	return Change{TypeScheduleOrderSet, ScheduleOrderSet{}}, nil
}

// MatchBlock is the schedule block of a match: its round of a pool
// ("MD/S1/A/2") or its bracket round ("MD/S2/R1").
func (s State) MatchBlock(m Match) string {
	if m.IsBracket() {
		return blocks.BracketKey(m.Category, m.Stage, s.bracketLevel(m), m.Start)
	}
	return blocks.PoolKey(m.Category, m.Stage, m.Pool, m.Round)
}

// BracketLevel is the round of a bracket match: 0 for the first round.
func (s State) BracketLevel(m Match) int { return s.bracketLevel(m) }

// bracketLevel is the round of a bracket match: 0 for the first round.
func (s State) bracketLevel(m Match) int {
	if !m.IsBracket() || m.Size == 0 {
		return 0
	}
	st := s.stage(m.Category, m.Stage)
	if st == nil {
		return 0
	}
	size := len(st.Slots())
	if size < m.Size {
		return 0
	}
	return bits.Len(uint(size/m.Size)) - 1
}
