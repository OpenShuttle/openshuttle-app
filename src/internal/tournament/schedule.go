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

// TypeScheduleSet records how the schedule shares the courts and the blocks
// the desk moved up.
const TypeScheduleSet events.Type = "ScheduleSet"

// ScheduleMode is how two categories share the courts.
type ScheduleMode string

const (
	// ScheduleAlternate plays the categories in turn: round 1 of the first
	// category, round 1 of the other, round 2 of the first…
	ScheduleAlternate ScheduleMode = "alternate"
	// ScheduleReserved gives each category its own courts until it is over.
	ScheduleReserved ScheduleMode = "reserved"
)

// MaxPinned bounds the blocks moved up by hand.
const MaxPinned = 200

// ScheduleSet is the payload of TypeScheduleSet: the whole schedule setup.
type ScheduleSet struct {
	Mode ScheduleMode `json:"mode"`
	// First opens every round in alternate mode ("" the first category).
	First engine.Category `json:"first,omitempty"`
	// Courts reserves courts for a category (reserved mode); courts left out
	// follow the default share.
	Courts map[int]engine.Category `json:"courts,omitempty"`
	// Pinned are block keys played before the others, in this order: a pool
	// ("MD/S1/A") or a bracket round ("MD/S2/R1").
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
	out.Pinned = slices.Clone(s.schedule.Pinned)
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

// SetSchedulePins sets the blocks played first, in order. Unknown or
// finished blocks are harmless: the schedule skips them.
func (s State) SetSchedulePins(keys []string) (Change, error) {
	var pinned []string
	for _, k := range keys {
		if k != "" && !slices.Contains(pinned, k) {
			pinned = append(pinned, k)
		}
	}
	if len(pinned) > MaxPinned {
		return Change{}, engine.NewError(engine.CodeInvalidSetting)
	}
	return s.scheduleChange(func(p *ScheduleSet) { p.Pinned = pinned })
}

func (s State) scheduleChange(edit func(*ScheduleSet)) (Change, error) {
	cur := s.ScheduleSettings()
	next := cur
	next.Courts = map[int]engine.Category{}
	for k, v := range cur.Courts {
		next.Courts[k] = v
	}
	next.Pinned = slices.Clone(cur.Pinned)
	edit(&next)
	if next.Mode == cur.Mode && next.First == cur.First && slices.Equal(next.Pinned, cur.Pinned) && mapsEqual(next.Courts, cur.Courts) {
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
	key        string
	cat        engine.Category
	stage      string
	phase      int
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

// Schedule forecasts the rest of the tournament from now. Courts take the
// next match as they free up:
//   - a bracket round starts once the round before it is over, and a stage
//     not filled yet once the stages feeding it are over;
//   - pinned blocks go first, in their order;
//   - with two categories, alternate mode plays round 1 of each category,
//     then round 2…; reserved mode keeps each category on its courts until
//     it is over;
//   - within a category, a pool keeps its court going as long as it can,
//     pools with the most rounds left go first, and within a pool the match
//     whose entrants rested the longest goes first (earliest round on a tie).
//
// Rest is never waited for: when no rested match fits the rules, the best
// one is placed anyway and flagged Unrested.
func (s State) Schedule(now time.Time) Schedule {
	items := s.scheduleItems()
	settings := s.ScheduleSettings()
	rest := time.Duration(s.MinRest()) * time.Minute
	catOrder := s.categoryOrder(settings)

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
			levelEnd[fmt.Sprintf("%s/%d", key, s.bracketLevel(m))] = later(levelEnd[fmt.Sprintf("%s/%d", key, s.bracketLevel(m))], end)
		case m.Ended():
			for _, id := range []engine.EntrantID{m.A, m.B} {
				lastEnd[id] = later(lastEnd[id], m.EndedAt)
			}
		}
	}

	// What is left per stage and per bracket round, to know when they are over.
	left := map[string]int{}
	for _, it := range items {
		left[string(it.cat)]++
		left[string(it.cat)+"/"+it.stage]++
		if it.pool < 0 {
			left[fmt.Sprintf("%s/%s/%d", it.cat, it.stage, it.level)]++
		}
	}
	reserved := s.reservedCourts(settings, items)
	// The pool of the latest call goes on, in the simulation as on court.
	lastPool := map[engine.Category]string{}
	lastCall := map[engine.Category]uint64{}
	for _, m := range s.matches {
		if m.Called() && !m.IsBracket() && m.callSeq >= lastCall[m.Category] {
			lastCall[m.Category] = m.callSeq
			lastPool[m.Category] = fmt.Sprintf("%s/%s/%s", m.Category, m.Stage, engine.PoolName(m.Pool))
		}
	}

	// ready returns when an item can start (zero if it depends on matches not
	// placed yet).
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
	// roundsLeft counts the rounds a pool still has to play, per decision.
	roundsLeft := func() map[string]int {
		seen := map[string]bool{}
		out := map[string]int{}
		for _, it := range items {
			if it.done || it.pool < 0 {
				continue
			}
			if k := fmt.Sprintf("%s#%d", it.key, it.level); !seen[k] {
				seen[k] = true
				out[it.key]++
			}
		}
		return out
	}
	restOf := func(id engine.EntrantID, t time.Time) time.Duration {
		if id == "" || lastEnd[id].IsZero() {
			return noRest
		}
		return t.Sub(lastEnd[id])
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
			if !ok || r == cat {
				return true
			}
			// A category over gives its courts to the other.
			return left[string(r)] == 0
		}
		var cands []*schedItem
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
			cands = append(cands, it)
		}
		if len(cands) == 0 {
			// Wait for the next court or entrant to free up.
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

		// Hard order first: pinned blocks, then rounds in alternate mode.
		group := func(it *schedItem) [3]int {
			pin := slices.Index(settings.Pinned, it.key)
			if pin < 0 {
				pin = MaxPinned
			}
			if settings.Mode == ScheduleAlternate && len(s.Categories) > 1 {
				return [3]int{pin, it.phase*1000 + it.level, catOrder[it.cat]}
			}
			return [3]int{pin, 0, 0}
		}
		best := group(cands[0])
		for _, it := range cands[1:] {
			if g := group(it); lessKey(g[:], best[:]) {
				best = g
			}
		}
		var inGroup, rested []*schedItem
		for _, it := range cands {
			if group(it) != best {
				continue
			}
			inGroup = append(inGroup, it)
			if restOf(it.a, t) >= rest && restOf(it.b, t) >= rest {
				rested = append(rested, it)
			}
		}
		pool := rested
		if len(pool) == 0 {
			pool = inGroup
		}
		// Soft order: stage, same pool, pools with most rounds left, rest.
		rounds := roundsLeft()
		soft := func(it *schedItem) []int {
			k := []int{it.phase, catOrder[it.cat]}
			if it.pool >= 0 {
				same := 1
				if lastPool[it.cat] == it.key {
					same = 0
				}
				avg := (restOf(it.a, t) + restOf(it.b, t)) / 2
				return append(k, 0, same, -rounds[it.key], it.pool, -int(avg/time.Second), it.level, it.order)
			}
			return append(k, 1, it.level, it.order)
		}
		pick := pool[0]
		for _, it := range pool[1:] {
			if lessKey(soft(it), soft(pick)) {
				pick = it
			}
		}

		end := t.Add(time.Duration(pick.minutes) * time.Minute)
		out.Slots = append(out.Slots, ScheduleSlot{
			Key: pick.key, Category: pick.cat, Stage: pick.stage, Pool: pick.pool, Level: pick.level,
			Match: pick.match, A: pick.a, B: pick.b, Court: c, Start: t, End: end, Unrested: len(rested) == 0,
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
		} else {
			lastPool[pick.cat] = pick.key
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
		phases := Phases(configs)
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
		for i, st := range stages {
			win := st.Win.EstimatedMinutes()
			if st.Confirmed {
				for _, m := range s.matches {
					if m.Category != c || m.Stage != st.ID || m.ByeA || m.ByeB || m.Called() || s.Decided(m) {
						continue
					}
					it := schedItem{cat: c, stage: st.ID, phase: phases[st.ID], pool: m.Pool, match: m.ID,
						a: m.A, b: m.B, minutes: win, order: len(items)}
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
			base := schedItem{cat: c, stage: st.ID, phase: phases[st.ID], minutes: win, unfilledOn: after}
			if st.Kind == StagePools {
				for pi, size := range p.Sizes {
					for r, round := range engine.RoundRobinSchedule(size) {
						for _, pair := range round {
							it := base
							it.pool, it.level, it.order = pi, r, len(items)
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
					it.pool, it.level, it.order = -1, level, len(items)
					it.key = fmt.Sprintf("%s/%s/R%d", c, st.ID, level+1)
					items = append(items, it)
				}
			}
		}
	}
	return items
}

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

func lessKey(a, b []int) bool {
	return slices.Compare(a, b) < 0
}

func allIdle(freeAt []time.Time) bool {
	for _, t := range freeAt[1:] {
		if t.Unix() != math.MaxInt32 {
			return false
		}
	}
	return true
}
