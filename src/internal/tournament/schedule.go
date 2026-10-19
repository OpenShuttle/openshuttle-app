package tournament

import (
	"fmt"
	"math"
	"math/bits"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	// TypeScheduleSet records how two categories share the courts.
	TypeScheduleSet events.Type = "ScheduleSet"
	// TypeScheduleOrderSet records the validated order of the waiting
	// matches, then every move the desk makes.
	TypeScheduleOrderSet events.Type = "ScheduleOrderSet"
	// TypeScheduleBlocksSet records the order of the blocks (pools, bracket
	// rounds) the desk set before validating.
	TypeScheduleBlocksSet events.Type = "ScheduleBlocksSet"
)

// ScheduleBlocksSet is the payload of TypeScheduleBlocksSet: block keys,
// "MD/S1/A" for a pool, "MD/S2/R1" for a bracket round, first played first
// within each wave.
type ScheduleBlocksSet struct {
	Order []string `json:"order"`
}

// ScheduleOrderSet is the payload of TypeScheduleOrderSet: the waiting
// matches in the order the courts take them. Its event time is when the
// schedule was validated or last changed. An empty order un-validates it.
type ScheduleOrderSet struct {
	Order []MatchID `json:"order"`
}

// ScheduleMode is how two categories share the courts.
type ScheduleMode string

const (
	// ScheduleAlternate plays the categories in turn: round 1 of the first
	// category, round 1 of the other, round 2 of the first…
	ScheduleAlternate ScheduleMode = "alternate"
	// ScheduleReserved gives each category its own courts until it is over.
	ScheduleReserved ScheduleMode = "reserved"
)

// ScheduleSet is the payload of TypeScheduleSet: the whole schedule setup.
type ScheduleSet struct {
	Mode ScheduleMode `json:"mode"`
	// First opens every round in alternate mode ("" the first category).
	First engine.Category `json:"first,omitempty"`
	// Courts reserves courts for a category (reserved mode); courts left out
	// follow the default share.
	Courts map[int]engine.Category `json:"courts,omitempty"`
	// Pinned held blocks moved up by hand in files written before the
	// schedule could be validated; it is ignored.
	Pinned []string `json:"pinned,omitempty"`
}

// ScheduleSettings returns the schedule setup, defaults filled in.
func (s State) ScheduleSettings() ScheduleSet {
	out := s.schedule
	if out.Mode == "" {
		out.Mode = ScheduleAlternate
	}
	if !s.HasCategory(out.First) {
		out.First = ""
		// The court priority of older files picks the category first.
		if c := engine.Category(s.priority); s.HasCategory(c) {
			out.First = c
		}
	}
	out.Courts = map[int]engine.Category{}
	for court, c := range s.schedule.Courts {
		if court >= 1 && court <= s.Courts() && s.HasCategory(c) {
			out.Courts[court] = c
		}
	}
	out.Pinned = nil
	return out
}

// SetScheduleMode chooses how two categories share the courts.
func (s State) SetScheduleMode(mode ScheduleMode) (Change, error) {
	if mode != ScheduleAlternate && mode != ScheduleReserved {
		return Change{}, engine.NewError(engine.CodeInvalidSetting)
	}
	return s.scheduleChange(func(p *ScheduleSet) { p.Mode = mode })
}

// SetScheduleFirst chooses the category opening each round in alternate mode.
func (s State) SetScheduleFirst(c engine.Category) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	return s.scheduleChange(func(p *ScheduleSet) { p.First = c })
}

// SetCourtCategory reserves a court for a category in reserved mode; ""
// gives the court back to the default share.
func (s State) SetCourtCategory(court int, c engine.Category) (Change, error) {
	if court < 1 || court > s.Courts() {
		return Change{}, engine.NewError(engine.CodeInvalidCourt)
	}
	if c != "" && !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	return s.scheduleChange(func(p *ScheduleSet) {
		if c == "" {
			delete(p.Courts, court)
		} else {
			p.Courts[court] = c
		}
	})
}

func (s State) scheduleChange(edit func(*ScheduleSet)) (Change, error) {
	cur := s.ScheduleSettings()
	next := cur
	next.Courts = map[int]engine.Category{}
	for k, v := range cur.Courts {
		next.Courts[k] = v
	}
	edit(&next)
	if next.Mode == cur.Mode && next.First == cur.First && mapsEqual(next.Courts, cur.Courts) {
		return Change{}, nil
	}
	if len(next.Courts) == 0 {
		next.Courts = nil
	}
	return Change{TypeScheduleSet, next}, nil
}

func mapsEqual(a, b map[int]engine.Category) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func (s *State) applyScheduleSet(p ScheduleSet) {
	s.schedule = p
}

func (s *State) applyScheduleOrderSet(p ScheduleOrderSet, at time.Time) {
	s.scheduleOrder = slices.Clone(p.Order)
	s.scheduleFrozenAt = at
	if len(p.Order) == 0 {
		s.scheduleFrozenAt = time.Time{}
	}
}

func (s *State) applyScheduleBlocksSet(p ScheduleBlocksSet) {
	s.scheduleBlocks = slices.Clone(p.Order)
}

// ScheduleSlot is a match of the schedule: on a court, from Start to End.
type ScheduleSlot struct {
	// Key is the block the match belongs to: its pool or its bracket round.
	Key      string
	Category engine.Category
	Stage    string
	Pool     int // -1 in a bracket
	// Level is the round of the pool, or the round of the bracket (0 the
	// first).
	Level int
	// RoundSize is, in a bracket, how many entrants the round starts with
	// (8 for quarter-finals, 2 for a final).
	RoundSize int
	// Match is "" in a stage not filled yet: who plays is not known.
	Match MatchID
	A, B  engine.EntrantID
	Court int
	Start time.Time
	End   time.Time
	// Unrested: no rested match fitted the rules here. The desk is warned: it
	// calls the match anyway or changes the order.
	Unrested bool
}

// Schedule is the forecast of the matches left: every waiting match, and
// the matches of stages not filled yet, on a court with estimated times.
type Schedule struct {
	Slots []ScheduleSlot
	// End is when the last match is expected to end.
	End time.Time
	// Reserved is the category of each court in reserved mode.
	Reserved map[int]engine.Category
}

// schedItem is a match to place.
type schedItem struct {
	phase      int
	stageIdx   int
	roundSize  int
	key        string
	cat        engine.Category
	stage      string
	pool       int
	level      int
	match      MatchID
	a, b       engine.EntrantID
	minutes    int
	order      int
	unfilledOn []string // stages that must be over first (stage not filled yet)
	done       bool
}

// noRest stands for "never played": as rested as can be.
const noRest = 24 * time.Hour

// Schedule forecasts the rest of the tournament from now. Matches follow the
// schedule order (ScheduleOrder), then the stand-ins of stages not filled
// yet. Each court, as it frees up, takes the first match of the order it can
// play: its players are off court, the bracket round before it is over, the
// stages feeding a stand-in are over; in reserved mode, a match of the
// court's category. Rest is never waited for: a match whose players have not
// had their minimum rest is placed anyway and flagged Unrested.
func (s State) Schedule(now time.Time) Schedule {
	items := s.orderedItems()
	settings := s.ScheduleSettings()
	rest := time.Duration(s.MinRest()) * time.Minute

	courts := s.Courts()
	freeAt := make([]time.Time, courts+1)
	for i := range freeAt {
		freeAt[i] = now
	}
	busyUntil := map[engine.EntrantID]time.Time{}
	lastEnd := map[engine.EntrantID]time.Time{}
	stageEnd := map[string]time.Time{} // latest end of a stage's matches
	levelEnd := map[string]time.Time{} // "<cat>/<stage>/<level>"
	for _, m := range s.matches {
		key := string(m.Category) + "/" + m.Stage
		switch {
		case s.Playing(m):
			end := m.CalledAt.Add(time.Duration(s.WinCondition(m).EstimatedMinutes()) * time.Minute)
			if !end.After(now) {
				// Running late: it ends any minute, never "already".
				end = now.Add(time.Minute)
			}
			if m.Court >= 1 && m.Court <= courts && end.After(freeAt[m.Court]) {
				freeAt[m.Court] = end
			}
			for _, id := range []engine.EntrantID{m.A, m.B} {
				busyUntil[id] = end
			}
			stageEnd[key] = later(stageEnd[key], end)
			lk := fmt.Sprintf("%s/%d", key, s.bracketLevel(m))
			levelEnd[lk] = later(levelEnd[lk], end)
		case m.Ended():
			for _, id := range []engine.EntrantID{m.A, m.B} {
				lastEnd[id] = later(lastEnd[id], m.EndedAt)
			}
		}
	}

	// What is left per category, stage and bracket round, to know when they
	// are over.
	left := map[string]int{}
	for _, it := range items {
		left[string(it.cat)]++
		left[string(it.cat)+"/"+it.stage]++
		if it.pool < 0 {
			left[fmt.Sprintf("%s/%s/%d", it.cat, it.stage, it.level)]++
		}
	}
	reserved := s.reservedCourts(settings, items)

	// ready returns when an item can start; false while it depends on
	// matches not placed yet.
	ready := func(it *schedItem) (time.Time, bool) {
		at := time.Time{}
		for _, src := range it.unfilledOn {
			k := string(it.cat) + "/" + src
			if left[k] > 0 {
				return at, false
			}
			at = later(at, stageEnd[k])
		}
		if it.pool < 0 {
			for l := 0; l < it.level; l++ {
				k := fmt.Sprintf("%s/%s/%d", it.cat, it.stage, l)
				if left[k] > 0 {
					return at, false
				}
				at = later(at, levelEnd[k])
			}
		}
		for _, id := range []engine.EntrantID{it.a, it.b} {
			if id != "" {
				at = later(at, busyUntil[id])
			}
		}
		return at, true
	}
	rested := func(id engine.EntrantID, t time.Time) bool {
		return id == "" || lastEnd[id].IsZero() || t.Sub(lastEnd[id]) >= rest
	}

	var out Schedule
	out.Reserved = reserved
	pending := len(items)
	for pending > 0 {
		// The court free first takes the next match.
		c := 0
		for i := 1; i <= courts; i++ {
			if c == 0 || freeAt[i].Before(freeAt[c]) {
				c = i
			}
		}
		if c == 0 {
			break
		}
		t := freeAt[c]
		allowed := func(cat engine.Category) bool {
			r, ok := reserved[c]
			// A category over gives its courts to the other.
			return !ok || r == cat || left[string(r)] == 0
		}
		var pick *schedItem
		next := time.Time{}
		for i := range items {
			it := &items[i]
			if it.done || !allowed(it.cat) {
				continue
			}
			at, known := ready(it)
			if !known {
				continue
			}
			if at.After(t) {
				if next.IsZero() || at.Before(next) {
					next = at
				}
				continue
			}
			pick = it
			break
		}
		if pick == nil {
			// Wait for the next court or player to free up.
			for i := 1; i <= courts; i++ {
				if freeAt[i].After(t) && (next.IsZero() || freeAt[i].Before(next)) {
					next = freeAt[i]
				}
			}
			if next.IsZero() {
				freeAt[c] = time.Unix(math.MaxInt32, 0) // nothing left for this court
				if allIdle(freeAt) {
					break
				}
				continue
			}
			freeAt[c] = next
			continue
		}

		end := t.Add(time.Duration(pick.minutes) * time.Minute)
		out.Slots = append(out.Slots, ScheduleSlot{
			Key: pick.key, Category: pick.cat, Stage: pick.stage, Pool: pick.pool, Level: pick.level,
			RoundSize: pick.roundSize, Match: pick.match, A: pick.a, B: pick.b, Court: c, Start: t, End: end,
			Unrested: !rested(pick.a, t) || !rested(pick.b, t),
		})
		pick.done = true
		pending--
		freeAt[c] = end
		for _, id := range []engine.EntrantID{pick.a, pick.b} {
			if id != "" {
				busyUntil[id], lastEnd[id] = end, end
			}
		}
		sk := string(pick.cat) + "/" + pick.stage
		left[sk]--
		left[string(pick.cat)]--
		stageEnd[sk] = later(stageEnd[sk], end)
		if pick.pool < 0 {
			lk := fmt.Sprintf("%s/%d", sk, pick.level)
			left[lk]--
			levelEnd[lk] = later(levelEnd[lk], end)
		}
		out.End = later(out.End, end)
	}
	slices.SortStableFunc(out.Slots, func(a, b ScheduleSlot) int {
		if r := a.Start.Compare(b.Start); r != 0 {
			return r
		}
		return a.Court - b.Court
	})
	return out
}

// orderedItems lists the matches to place: the waiting matches in schedule
// order, then the stand-ins of stages not filled yet.
func (s State) orderedItems() []schedItem {
	all := s.scheduleItems()
	byMatch := map[MatchID]schedItem{}
	var out []schedItem
	for _, it := range all {
		if it.match != "" {
			byMatch[it.match] = it
		}
	}
	for _, id := range s.ScheduleOrder() {
		if it, ok := byMatch[id]; ok {
			out = append(out, it)
		}
	}
	// Stand-ins of stages not filled yet: phase by phase, wave by wave
	// (round k of every stage of the phase), like the waiting matches.
	order := s.categoryOrder(s.ScheduleSettings())
	var stand []schedItem
	for _, it := range all {
		if it.match == "" {
			stand = append(stand, it)
		}
	}
	slices.SortStableFunc(stand, func(a, b schedItem) int {
		return slices.Compare(
			[]int{a.phase, a.level, order[a.cat], a.stageIdx, a.pool},
			[]int{b.phase, b.level, order[b.cat], b.stageIdx, b.pool})
	})
	return append(out, stand...)
}

// AutoOrder is the order of the waiting matches before the schedule is
// validated. Per category, stage by stage, the matches go in waves: in a
// pools stage, wave k holds round k of every pool; in a bracket, wave k is
// round k. Waves of both categories go side by side (wave 1 of each, then
// wave 2…), and within a wave the blocks (pools, bracket rounds) follow the
// block order: by default the category opening first, then the pools with
// the most matches (pool order on a tie), so everyone waits about as long.
func (s State) AutoOrder() []MatchID {
	waves := s.categoryWaves()
	rank := s.blockRanks(waves)
	var out []MatchID
	for k := 0; ; k++ {
		var step []waveBlock
		for _, c := range s.Categories {
			if k < len(waves[c]) {
				step = append(step, waves[c][k]...)
			}
		}
		if step == nil {
			return out
		}
		slices.SortStableFunc(step, func(a, b waveBlock) int { return rank[a.key] - rank[b.key] })
		for _, b := range step {
			out = append(out, b.matches...)
		}
	}
}

// categoryWaves cuts the waiting matches of each category into waves, phase
// after phase. Stages of the same phase (a main bracket and pools for the
// others, say) play side by side: wave k holds round k of each.
func (s State) categoryWaves() map[engine.Category][][]waveBlock {
	waves := map[engine.Category][][]waveBlock{}
	for _, c := range s.Categories {
		phases := Phases(s.StageConfigs(c))
		byPhase := map[int][][]waveBlock{}
		var order []int
		for _, st := range s.Stages(c) {
			if !st.Confirmed {
				continue
			}
			p := phases[st.ID]
			if _, ok := byPhase[p]; !ok {
				order = append(order, p)
			}
			for k, w := range s.stageWaves(c, st) {
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

// waveBlock is the part of a wave one block plays: a round of a pool, or a
// bracket round.
type waveBlock struct {
	key     string
	matches []MatchID
}

// blockRanks ranks the blocks: those the desk ordered first, in its order,
// then the others in default order (the category opening first, then each
// category's blocks as its waves bring them).
func (s State) blockRanks(waves map[engine.Category][][]waveBlock) map[string]int {
	order := s.categoryOrder(s.ScheduleSettings())
	cats := slices.Clone(s.Categories)
	slices.SortStableFunc(cats, func(a, b engine.Category) int { return order[a] - order[b] })
	rank := map[string]int{}
	for i, k := range s.scheduleBlocks {
		rank[k] = i
	}
	n := len(s.scheduleBlocks)
	for _, c := range cats {
		for _, wave := range waves[c] {
			for _, b := range wave {
				if _, ok := rank[b.key]; !ok {
					rank[b.key] = n
					n++
				}
			}
		}
	}
	return rank
}

// ScheduleBlocks lists the blocks with waiting matches, in block order.
func (s State) ScheduleBlocks() []string {
	waves := s.categoryWaves()
	rank := s.blockRanks(waves)
	var out []string
	for _, ws := range waves {
		for _, wave := range ws {
			for _, b := range wave {
				if !slices.Contains(out, b.key) {
					out = append(out, b.key)
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b string) int { return rank[a] - rank[b] })
	return out
}

// stageWaves cuts the waiting matches of a filled stage into waves: in
// pools, wave k holds the k-th round left of every pool; in a bracket, wave
// k is round k. Blocks are in pool order (ranked later).
func (s State) stageWaves(c engine.Category, st Stage) [][]waveBlock {
	waiting := func(m Match) bool {
		return m.Category == c && m.Stage == st.ID && !m.ByeA && !m.ByeB && !m.Called() && !s.Decided(m)
	}
	if st.Kind == StageBracket {
		var waves [][]waveBlock
		for _, m := range s.matches {
			if !waiting(m) {
				continue
			}
			l := s.bracketLevel(m)
			for len(waves) <= l {
				waves = append(waves, []waveBlock{{key: fmt.Sprintf("%s/%s/R%d", c, st.ID, len(waves)+1)}})
			}
			waves[l][0].matches = append(waves[l][0].matches, m.ID)
		}
		return slices.DeleteFunc(waves, func(w []waveBlock) bool { return len(w[0].matches) == 0 })
	}
	total := map[int]int{}
	rounds := map[int]map[int][]MatchID{} // pool → round → matches
	for _, m := range s.matches {
		if m.Category != c || m.Stage != st.ID || m.ByeA || m.ByeB {
			continue
		}
		total[m.Pool]++
		if !waiting(m) {
			continue
		}
		if rounds[m.Pool] == nil {
			rounds[m.Pool] = map[int][]MatchID{}
		}
		rounds[m.Pool][m.Round] = append(rounds[m.Pool][m.Round], m.ID)
	}
	// Default block order within the category: the pools with the most
	// matches first, pool order on a tie.
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
	var waves [][]waveBlock
	for k := 0; ; k++ {
		var wave []waveBlock
		for _, p := range pools {
			if k < len(left[p]) {
				wave = append(wave, waveBlock{key: fmt.Sprintf("%s/%s/%s", c, st.ID, engine.PoolName(p)), matches: rounds[p][left[p][k]]})
			}
		}
		if wave == nil {
			return waves
		}
		waves = append(waves, wave)
	}
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
	return s.moveScheduled(func(m Match) bool { return m.ID == id }, before)
}

// MoveScheduledBlock moves a block (a pool, a bracket round) just before
// another one ("" at the end) in the block order, before the schedule is
// validated: within each wave, its matches then come before the other's.
func (s State) MoveScheduledBlock(key, before string) (Change, error) {
	if !s.scheduleFrozenAt.IsZero() {
		return Change{}, engine.NewError(engine.CodeScheduleValidated)
	}
	blocks := s.ScheduleBlocks()
	if !slices.Contains(blocks, key) || (before != "" && !slices.Contains(blocks, before)) {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	if key == before {
		return Change{}, nil
	}
	rest := slices.DeleteFunc(slices.Clone(blocks), func(k string) bool { return k == key })
	at := len(rest)
	if before != "" {
		at = slices.Index(rest, before)
	}
	next := slices.Concat(rest[:at], []string{key}, rest[at:])
	if slices.Equal(next, blocks) {
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

func (s State) moveScheduled(moved func(Match) bool, before MatchID) (Change, error) {
	if s.scheduleFrozenAt.IsZero() {
		return Change{}, engine.NewError(engine.CodeScheduleNotValidated)
	}
	order := s.ScheduleOrder()
	var block, rest []MatchID
	for _, id := range order {
		if m, ok := s.Match(id); ok && moved(m) {
			block = append(block, id)
		} else {
			rest = append(rest, id)
		}
	}
	if len(block) == 0 {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	at := len(rest)
	if before != "" {
		if slices.Contains(block, before) {
			// Dropped on itself: nothing moves.
			return Change{}, nil
		}
		at = slices.Index(rest, before)
		if at < 0 {
			return Change{}, engine.NewError(engine.CodeMatchNotFound)
		}
	}
	next := slices.Concat(rest[:at], block, rest[at:])
	if slices.Equal(next, order) {
		return Change{}, nil
	}
	return Change{TypeScheduleOrderSet, ScheduleOrderSet{Order: next}}, nil
}

// MatchesLeft counts, per entrant, the matches of a stage it has not started.
func (s State) MatchesLeft(c engine.Category, stage string) map[engine.EntrantID]int {
	out := map[engine.EntrantID]int{}
	for _, m := range s.matches {
		if m.Category != c || m.Stage != stage || m.ByeA || m.ByeB || m.Called() || s.Decided(m) {
			continue
		}
		for _, id := range []engine.EntrantID{m.A, m.B} {
			if id != "" {
				out[id]++
			}
		}
	}
	return out
}

// Proposals gives each free or finished court the match the schedule starts
// there now. Timed matches are called by rounds, never one court at a time.
func (s State) Proposals(now time.Time) map[int]ScheduleSlot {
	out := map[int]ScheduleSlot{}
	playing := map[int]bool{}
	for _, c := range s.CourtStates() {
		playing[c.Number] = c.State == CourtPlaying
	}
	for _, slot := range s.Schedule(now).Slots {
		if !slot.Start.Equal(now) || playing[slot.Court] || slot.Match == "" {
			continue
		}
		if m, ok := s.Match(slot.Match); ok && !s.Timed(m) && s.Playable(m) && !s.onCourt(m.A) && !s.onCourt(m.B) {
			out[slot.Court] = slot
		}
	}
	return out
}

// onCourt reports whether an entrant is playing a match.
func (s State) onCourt(id engine.EntrantID) bool {
	return id != "" && slices.ContainsFunc(s.matches, func(m Match) bool { return m.Has(id) && s.Playing(m) })
}

// scheduleItems lists the matches left: waiting matches of filled stages,
// and stand-in matches for stages not filled yet.
func (s State) scheduleItems() []schedItem {
	var items []schedItem
	for _, c := range s.Categories {
		stages := s.Stages(c)
		configs := s.StageConfigs(c)
		filled := map[string][]int{}
		for _, st := range stages {
			if st.Confirmed {
				filled[st.ID] = groupSizes(st.Groups)
			}
		}
		n := len(s.Present(c))
		if n == 0 {
			for _, e := range s.Entrants {
				if e.Category == c {
					n++
				}
			}
		}
		preview := PreviewFormat(configs, n, filled)
		phases := Phases(configs)
		for i, st := range stages {
			win := st.Win.EstimatedMinutes()
			if st.Confirmed {
				for _, m := range s.matches {
					if m.Category != c || m.Stage != st.ID || m.ByeA || m.ByeB || m.Called() || s.Decided(m) {
						continue
					}
					it := schedItem{cat: c, stage: st.ID, phase: phases[st.ID], stageIdx: i, pool: m.Pool, match: m.ID,
						a: m.A, b: m.B, minutes: win, roundSize: m.Size}
					if m.IsBracket() {
						it.level = s.bracketLevel(m)
						it.key = fmt.Sprintf("%s/%s/R%d", c, st.ID, it.level+1)
					} else {
						it.level = m.Round
						it.key = fmt.Sprintf("%s/%s/%s", c, st.ID, engine.PoolName(m.Pool))
					}
					items = append(items, it)
				}
				continue
			}
			// Not filled yet: stand-ins, after the stages feeding it.
			p := preview.Stages[i]
			var after []string
			if i > 0 {
				after = s.Sources(c, st.ID)
				if len(after) == 0 {
					continue // nothing feeds it: nobody will play it
				}
			}
			base := schedItem{cat: c, stage: st.ID, phase: phases[st.ID], stageIdx: i, minutes: win, unfilledOn: after}
			if st.Kind == StagePools {
				for pi, size := range p.Sizes {
					for r, round := range engine.RoundRobinSchedule(size) {
						for _, pair := range round {
							it := base
							it.pool, it.level = pi, r
							it.key = fmt.Sprintf("%s/%s/%s", c, st.ID, engine.PoolName(pi))
							it.a = engine.EntrantID(fmt.Sprintf("~%s/%s/%d/%d", c, st.ID, pi, pair[0]))
							it.b = engine.EntrantID(fmt.Sprintf("~%s/%s/%d/%d", c, st.ID, pi, pair[1]))
							items = append(items, it)
						}
					}
				}
				continue
			}
			if p.Entrants < 2 {
				continue
			}
			size := engine.NextPowerOfTwo(p.Entrants)
			byes := size - p.Entrants
			for _, node := range subBrackets(size, st.FullPlacement) {
				level := bits.Len(uint(size/node.Size)) - 1
				for range node.Size / 2 {
					if level == 0 && byes > 0 {
						byes--
						continue
					}
					it := base
					it.pool, it.level, it.roundSize = -1, level, node.Size
					it.key = fmt.Sprintf("%s/%s/R%d", c, st.ID, level+1)
					items = append(items, it)
				}
			}
		}
	}
	return items
}

// MatchBlock is the schedule block of a match: its pool ("MD/S1/A") or its
// bracket round ("MD/S2/R1").
func (s State) MatchBlock(m Match) string {
	if m.IsBracket() {
		return fmt.Sprintf("%s/%s/R%d", m.Category, m.Stage, s.bracketLevel(m)+1)
	}
	return fmt.Sprintf("%s/%s/%s", m.Category, m.Stage, engine.PoolName(m.Pool))
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

// reservedCourts gives each court its category in reserved mode: the courts
// reserved by hand, the others shared by the matches left (in court order,
// first category first).
func (s State) reservedCourts(settings ScheduleSet, items []schedItem) map[int]engine.Category {
	out := map[int]engine.Category{}
	if settings.Mode != ScheduleReserved || len(s.Categories) < 2 {
		return out
	}
	courts := s.Courts()
	left := map[engine.Category]int{}
	for _, it := range items {
		left[it.cat]++
	}
	total := 0
	for _, c := range s.Categories {
		total += left[c]
	}
	free := []int{}
	for i := 1; i <= courts; i++ {
		if c, ok := settings.Courts[i]; ok {
			out[i] = c
		} else {
			free = append(free, i)
		}
	}
	if total == 0 || len(free) == 0 {
		return out
	}
	first := s.Categories[0]
	share := int(math.Round(float64(courts*left[first]) / float64(total)))
	share = min(max(share, 1), courts-1)
	have := 0
	for _, c := range out {
		if c == first {
			have++
		}
	}
	for _, i := range free {
		if have < share {
			out[i] = first
			have++
		} else {
			out[i] = s.Categories[1]
		}
	}
	return out
}

// categoryOrder ranks the categories for alternate mode: the first one set,
// then the others in tournament order.
func (s State) categoryOrder(settings ScheduleSet) map[engine.Category]int {
	out := map[engine.Category]int{}
	n := 0
	if settings.First != "" {
		out[settings.First] = n
		n++
	}
	for _, c := range s.Categories {
		if _, ok := out[c]; !ok {
			out[c] = n
			n++
		}
	}
	return out
}

func groupSizes(groups [][]engine.EntrantID) []int {
	var sizes []int
	for _, g := range groups {
		n := 0
		for _, id := range g {
			if id != "" {
				n++
			}
		}
		sizes = append(sizes, n)
	}
	return sizes
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func allIdle(freeAt []time.Time) bool {
	for _, t := range freeAt[1:] {
		if t.Unix() != math.MaxInt32 {
			return false
		}
	}
	return true
}
