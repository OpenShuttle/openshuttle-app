// Package forecast simulates the rest of a tournament: courts take the
// matches of an ordered list as they free up, and each match gets a court and
// estimated times. The tournament builds the input (the matches left in
// schedule order, what is on court, who played when); this package only
// places them. Pure: the time is passed in.
package forecast

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// Item is a match to place.
type Item struct {
	// Key is the block the match belongs to: its round of a pool or of a
	// bracket.
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
	// PlaceFrom is, in a bracket, the first place its part of the bracket
	// plays for: 1 for the main draw, 5 for places 5 to 8 (0 in pools).
	PlaceFrom int
	// Match is "" for a stand-in of a stage not filled yet: who plays is not
	// known (A and B are then placeholders, or "").
	Match   string
	A, B    engine.EntrantID
	Minutes int
	// After lists the stages of the category that must be over first (a
	// stage not filled yet waits for its sources).
	After []string
}

// OnCourt is a match being played, until its (estimated) end.
type OnCourt struct {
	Court    int
	Category engine.Category
	Stage    string
	Level    int
	A, B     engine.EntrantID
	End      time.Time
}

// Played is a match over: its players rest from its end.
type Played struct {
	A, B engine.EntrantID
	End  time.Time
}

// Input is what the simulation starts from.
type Input struct {
	Now    time.Time
	Courts int
	// Rest is the minimum rest between two matches of an entrant.
	Rest time.Duration
	// Items are the matches to place, in the order courts take them.
	Items   []Item
	OnCourt []OnCourt
	Played  []Played
	// Reserved gives a court to a category (reserved mode); a category over
	// gives its courts to the other.
	Reserved map[int]engine.Category
}

// Slot is a placed match: on a court, from Start to End.
type Slot struct {
	Item
	Court int
	Start time.Time
	End   time.Time
	// Unrested: no rested match fitted the rules here. The desk is warned.
	Unrested bool
}

// Result is the forecast: every placed match by start time, then court.
type Result struct {
	Slots []Slot
	// End is when the last match is expected to end.
	End time.Time
}

// Run places the items. Each court, as it frees up, takes the first item it
// can play: its players are off court, the bracket round before it is over,
// the stages it waits for are over; on a reserved court, an item of the
// court's category. Rest is never waited for: an item whose players have not
// had their minimum rest is placed anyway and flagged Unrested.
func Run(in Input) Result {
	sim := newSim(in)
	var out Result
	pending := len(in.Items)
	for pending > 0 {
		c := sim.firstFreeCourt()
		if c == 0 {
			break
		}
		t := sim.freeAt[c]
		pick, next := sim.pick(c, t)
		if pick == nil {
			if !sim.wait(c, t, next) {
				break
			}
			continue
		}
		out.Slots = append(out.Slots, sim.place(pick, c, t))
		pending--
		out.End = later(out.End, out.Slots[len(out.Slots)-1].End)
	}
	slices.SortStableFunc(out.Slots, func(a, b Slot) int {
		if r := a.Start.Compare(b.Start); r != 0 {
			return r
		}
		return a.Court - b.Court
	})
	return out
}

// sim is the state of the simulation.
type sim struct {
	in        Input
	items     []Item
	done      []bool
	freeAt    []time.Time
	busyUntil map[engine.EntrantID]time.Time
	lastEnd   map[engine.EntrantID]time.Time
	stageEnd  map[string]time.Time // latest end of a stage's matches: "<cat>/<stage>"
	levelEnd  map[string]time.Time // "<cat>/<stage>/<level>"
	// left counts the items not placed yet per category, stage and bracket
	// round, to know when they are over.
	left map[string]int
}

func newSim(in Input) *sim {
	s := &sim{
		in: in, items: in.Items, done: make([]bool, len(in.Items)),
		freeAt:    make([]time.Time, in.Courts+1),
		busyUntil: map[engine.EntrantID]time.Time{}, lastEnd: map[engine.EntrantID]time.Time{},
		stageEnd: map[string]time.Time{}, levelEnd: map[string]time.Time{}, left: map[string]int{},
	}
	for i := range s.freeAt {
		s.freeAt[i] = in.Now
	}
	for _, m := range in.OnCourt {
		if m.Court >= 1 && m.Court <= in.Courts && m.End.After(s.freeAt[m.Court]) {
			s.freeAt[m.Court] = m.End
		}
		for _, id := range []engine.EntrantID{m.A, m.B} {
			s.busyUntil[id] = m.End
		}
		sk := stageKey(m.Category, m.Stage)
		s.stageEnd[sk] = later(s.stageEnd[sk], m.End)
		lk := levelKey(sk, m.Level)
		s.levelEnd[lk] = later(s.levelEnd[lk], m.End)
	}
	for _, m := range in.Played {
		for _, id := range []engine.EntrantID{m.A, m.B} {
			s.lastEnd[id] = later(s.lastEnd[id], m.End)
		}
	}
	for _, it := range in.Items {
		s.left[string(it.Category)]++
		s.left[stageKey(it.Category, it.Stage)]++
		if it.Pool < 0 {
			s.left[levelKey(stageKey(it.Category, it.Stage), it.Level)]++
		}
	}
	return s
}

func stageKey(c engine.Category, stage string) string { return string(c) + "/" + stage }

func levelKey(stageKey string, level int) string { return fmt.Sprintf("%s/%d", stageKey, level) }

// firstFreeCourt is the court free first, 0 without courts.
func (s *sim) firstFreeCourt() int {
	c := 0
	for i := 1; i <= s.in.Courts; i++ {
		if c == 0 || s.freeAt[i].Before(s.freeAt[c]) {
			c = i
		}
	}
	return c
}

// pick is the first item court c can start at t; else nil and the earliest
// time an item it waits for could start (zero when none is known).
func (s *sim) pick(c int, t time.Time) (*Item, time.Time) {
	next := time.Time{}
	for i := range s.items {
		it := &s.items[i]
		if s.done[i] || !s.allowed(c, it.Category) {
			continue
		}
		at, known := s.ready(it)
		if !known {
			continue
		}
		if at.After(t) {
			if next.IsZero() || at.Before(next) {
				next = at
			}
			continue
		}
		s.done[i] = true
		return it, next
	}
	return nil, next
}

// allowed reports whether court c may take a match of category cat.
func (s *sim) allowed(c int, cat engine.Category) bool {
	r, ok := s.in.Reserved[c]
	return !ok || r == cat || s.left[string(r)] == 0
}

// ready returns when an item can start; false while it depends on items not
// placed yet.
func (s *sim) ready(it *Item) (time.Time, bool) {
	at := time.Time{}
	for _, src := range it.After {
		k := stageKey(it.Category, src)
		if s.left[k] > 0 {
			return at, false
		}
		at = later(at, s.stageEnd[k])
	}
	if it.Pool < 0 {
		sk := stageKey(it.Category, it.Stage)
		for l := 0; l < it.Level; l++ {
			k := levelKey(sk, l)
			if s.left[k] > 0 {
				return at, false
			}
			at = later(at, s.levelEnd[k])
		}
	}
	for _, id := range []engine.EntrantID{it.A, it.B} {
		if id != "" {
			at = later(at, s.busyUntil[id])
		}
	}
	return at, true
}

// wait moves court c, idle at t, to the next time something can change: an
// item becoming ready (next), or another court freeing up. false when nothing
// is left for any court.
func (s *sim) wait(c int, t, next time.Time) bool {
	for i := 1; i <= s.in.Courts; i++ {
		if s.freeAt[i].After(t) && (next.IsZero() || s.freeAt[i].Before(next)) {
			next = s.freeAt[i]
		}
	}
	if !next.IsZero() {
		s.freeAt[c] = next
		return true
	}
	s.freeAt[c] = never // nothing left for this court
	for _, f := range s.freeAt[1:] {
		if !f.Equal(never) {
			return true
		}
	}
	return false
}

// never marks a court with nothing left to play.
var never = time.Unix(math.MaxInt32, 0)

// place puts an item on court c at t.
func (s *sim) place(it *Item, c int, t time.Time) Slot {
	end := t.Add(time.Duration(it.Minutes) * time.Minute)
	slot := Slot{Item: *it, Court: c, Start: t, End: end, Unrested: !s.rested(it.A, t) || !s.rested(it.B, t)}
	s.freeAt[c] = end
	for _, id := range []engine.EntrantID{it.A, it.B} {
		if id != "" {
			s.busyUntil[id], s.lastEnd[id] = end, end
		}
	}
	sk := stageKey(it.Category, it.Stage)
	s.left[sk]--
	s.left[string(it.Category)]--
	s.stageEnd[sk] = later(s.stageEnd[sk], end)
	if it.Pool < 0 {
		lk := levelKey(sk, it.Level)
		s.left[lk]--
		s.levelEnd[lk] = later(s.levelEnd[lk], end)
	}
	return slot
}

// rested reports whether an entrant had its minimum rest by t.
func (s *sim) rested(id engine.EntrantID, t time.Time) bool {
	return id == "" || s.lastEnd[id].IsZero() || t.Sub(s.lastEnd[id]) >= s.in.Rest
}

// ReserveCourts gives each court its category in reserved mode: the courts
// reserved by hand (manual), the others shared by the matches left in each
// category (in court order, the first category first, at least one court
// each).
func ReserveCourts(courts int, manual map[int]engine.Category, categories []engine.Category, left map[engine.Category]int) map[int]engine.Category {
	out := map[int]engine.Category{}
	if len(categories) < 2 {
		return out
	}
	total := 0
	for _, c := range categories {
		total += left[c]
	}
	var free []int
	for i := 1; i <= courts; i++ {
		if c, ok := manual[i]; ok {
			out[i] = c
		} else {
			free = append(free, i)
		}
	}
	if total == 0 || len(free) == 0 {
		return out
	}
	first := categories[0]
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
			out[i] = categories[1]
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
