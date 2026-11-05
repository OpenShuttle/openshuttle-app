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
	// Pause is how long the item waits after the stages of After are over.
	Pause time.Duration
	// NotBefore is the earliest start: the real end of the stages feeding
	// the item's stage plus its pause, when they are over already.
	NotBefore time.Time
	// Ghost is the ID of a ghost match: Court stays empty for Minutes, nobody
	// plays. It is "" for a real match.
	Ghost string
	// Court is the only court that may take the item (0: any).
	Court int
	// AfterMatch is a match the item waits for: it starts once that match,
	// still to place, has ended.
	AfterMatch string
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
	// Breaks are pauses of every court: no match is placed across one.
	Breaks []Break
	// Rules steer each court: what it takes first, only, or never. A rule
	// ends once no item of its scope is left to place.
	Rules map[int][]Rule
}

// RuleKind is what a court rule does.
type RuleKind string

const (
	// RulePriority: the court takes a ready item of the scope before the
	// items ahead of it in the order (the lowest level first).
	RulePriority RuleKind = "priority"
	// RuleOnly: the court takes nothing else while the scope has items left
	// (with several, any of their scopes).
	RuleOnly RuleKind = "only"
	// RuleNever: the court never takes an item of the scope.
	RuleNever RuleKind = "never"
)

// Rule is a court rule: a kind, a level for a priority (1 first), a scope.
type Rule struct {
	Kind  RuleKind
	Level int
	Scope Scope
}

// Scope is a set of items: a category, a stage of it, a pool of a pools
// stage or a round of a bracket stage.
type Scope struct {
	Category engine.Category
	Stage    string // "" every stage
	// Pool is the pool's index plus one; 0 any pool.
	Pool int
	// RoundSize and PlaceFrom name a round of a bracket (2 and 1: the
	// final); 0 any round.
	RoundSize, PlaceFrom int
}

// Matches reports whether an item is in the scope.
func (sc Scope) Matches(it Item) bool {
	switch {
	case it.Category != sc.Category, sc.Stage != "" && it.Stage != sc.Stage:
		return false
	case sc.Pool > 0:
		return it.Pool == sc.Pool-1
	case sc.RoundSize > 0:
		return it.Pool < 0 && it.RoundSize == sc.RoundSize && (sc.PlaceFrom == 0 || it.PlaceFrom == sc.PlaceFrom)
	}
	return true
}

// Break is a pause of every court, from From to To.
type Break struct {
	From, To time.Time
}

// Slot is a placed match: on a court, from Start to End.
type Slot struct {
	Item
	Court int
	Start time.Time
	End   time.Time
	// Unrested: no rested match fitted the rules here. The desk is warned.
	Unrested bool
	// OffRules: no court the rules allow could ever take the item, so it was
	// placed against them.
	OffRules bool
}

// Result is the forecast: every placed match by start time, then court.
type Result struct {
	Slots []Slot
	// End is when the last match is expected to end (ghost matches left out).
	End time.Time
}

// Run places the items. Each court, as it frees up, takes the first item it
// can play: its players are off court, the bracket round before it is over,
// the stages it waits for are over and its pause after them too; on a
// reserved court, an item of the court's category; one its rules allow,
// the ready item of its highest priority scope first, in order on a tie.
// When the rules leave items no court can take, they are dropped and those
// items flagged OffRules. Rest is never waited for: an item whose players
// have not had their minimum rest is placed anyway and flagged Unrested. A court
// never starts a match that would run into a break: it waits for its end.
// A ghost match keeps its court empty in its turn, after the match it
// follows; it does not count in End.
func Run(in Input) Result {
	sim := newSim(in)
	var out Result
	pending := len(in.Items)
	for pending > 0 {
		c := sim.firstFreeCourt()
		if c == 0 {
			if sim.relax() {
				continue
			}
			break
		}
		t := sim.freeAt[c]
		if end, ok := sim.breakAt(t, 0); ok {
			sim.freeAt[c] = end
			continue
		}
		i, next := sim.pick(c, t)
		if i < 0 {
			if !sim.wait(c, t, next) && !sim.relax() {
				break
			}
			continue
		}
		pick := &sim.items[i]
		if end, ok := sim.breakAt(t, pick.Minutes); ok {
			sim.done[i] = false
			sim.freeAt[c] = end
			continue
		}
		out.Slots = append(out.Slots, sim.place(i, c, t))
		sim.revive(t)
		pending--
		if pick.Ghost == "" {
			out.End = later(out.End, out.Slots[len(out.Slots)-1].End)
		}
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
	// queued lists the matches among the items; ended gives the end of the
	// ones placed.
	queued map[string]bool
	ended  map[string]time.Time
	// rules flattens the court rules; ruleLeft counts the items of each
	// rule's scope not placed yet, itemRules the rules matching each item.
	rules     []courtRule
	ruleLeft  []int
	itemRules [][]int
	// relaxed: the rules left items no court could take; they no longer
	// apply. idleAt is when a court found nothing left for it.
	relaxed bool
	idleAt  []time.Time
}

// courtRule is a rule of a court.
type courtRule struct {
	Rule
	court int
}

func newSim(in Input) *sim {
	s := &sim{
		in: in, items: in.Items, done: make([]bool, len(in.Items)),
		freeAt:    make([]time.Time, in.Courts+1),
		busyUntil: map[engine.EntrantID]time.Time{}, lastEnd: map[engine.EntrantID]time.Time{},
		stageEnd: map[string]time.Time{}, levelEnd: map[string]time.Time{}, left: map[string]int{},
		queued: map[string]bool{}, ended: map[string]time.Time{},
		itemRules: make([][]int, len(in.Items)), idleAt: make([]time.Time, in.Courts+1),
	}
	for c := 1; c <= in.Courts; c++ {
		for _, r := range in.Rules[c] {
			s.rules = append(s.rules, courtRule{r, c})
		}
	}
	s.ruleLeft = make([]int, len(s.rules))
	for i, it := range in.Items {
		if it.Ghost != "" {
			continue
		}
		for r, cr := range s.rules {
			if cr.Scope.Matches(it) {
				s.itemRules[i] = append(s.itemRules[i], r)
				s.ruleLeft[r]++
			}
		}
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
		if it.Ghost != "" {
			continue
		}
		if it.Match != "" {
			s.queued[it.Match] = true
		}
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

// firstFreeCourt is the court free first, 0 without courts. Of courts free
// together, one with an only rule running goes first, then one with a
// priority running, then the lowest number: the courts set for a scope get
// its items.
func (s *sim) firstFreeCourt() int {
	c := 0
	for i := 1; i <= s.in.Courts; i++ {
		if c == 0 || s.freeAt[i].Before(s.freeAt[c]) || (s.freeAt[i].Equal(s.freeAt[c]) && s.claim(i) < s.claim(c)) {
			c = i
		}
	}
	return c
}

// claim ranks a court among courts free together: 0 with an only rule
// running, 1 with a priority running, 2 otherwise.
func (s *sim) claim(c int) int {
	out := 2
	if s.relaxed {
		return out
	}
	for r, cr := range s.rules {
		if cr.court != c || s.ruleLeft[r] == 0 {
			continue
		}
		switch cr.Kind {
		case RuleOnly:
			return 0
		case RulePriority:
			out = 1
		}
	}
	return out
}

// pick is the item court c starts at t: of the ready items it may take, the
// one of its highest priority scope, the first in order on a tie; else -1
// and the earliest time an item it waits for could start (zero when none is
// known).
func (s *sim) pick(c int, t time.Time) (int, time.Time) {
	next := time.Time{}
	best, bestLevel := -1, 0
	for i := range s.items {
		it := &s.items[i]
		if s.done[i] || (it.Court != 0 && it.Court != c) || (it.Ghost == "" && (!s.allowed(c, it.Category) || !s.allowedByRules(c, i))) {
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
		if level := s.level(c, i); best < 0 || level < bestLevel {
			best, bestLevel = i, level
		}
	}
	if best >= 0 {
		s.done[best] = true
	}
	return best, next
}

// allowedByRules reports whether court c's rules let it take item i: no
// never rule of the court holds it, and when the court has only rules
// still running, one of them holds it.
func (s *sim) allowedByRules(c, i int) bool {
	if s.relaxed {
		return true
	}
	return s.ruleAllows(c, i)
}

// ruleAllows applies court c's running rules to item i.
func (s *sim) ruleAllows(c, i int) bool {
	only, inOnly := false, false
	for r, cr := range s.rules {
		if cr.court != c || s.ruleLeft[r] == 0 {
			continue
		}
		holds := slices.Contains(s.itemRules[i], r)
		switch cr.Kind {
		case RuleNever:
			if holds {
				return false
			}
		case RuleOnly:
			only = true
			inOnly = inOnly || holds
		}
	}
	return !only || inOnly
}

// level is item i's priority on court c: the lowest level of the court's
// running priority rules holding it. A ghost match keeps its turn on a
// court with priorities.
func (s *sim) level(c, i int) int {
	level, any := math.MaxInt, false
	for r, cr := range s.rules {
		if cr.court != c || cr.Kind != RulePriority || s.ruleLeft[r] == 0 {
			continue
		}
		any = true
		if slices.Contains(s.itemRules[i], r) {
			level = min(level, cr.Level)
		}
	}
	if s.items[i].Ghost != "" && any {
		return math.MinInt
	}
	return level
}

// revive wakes the courts that found nothing left at t, once an item is
// placed: items waiting for it become known, and a rule may have ended.
func (s *sim) revive(t time.Time) {
	for c := 1; c <= s.in.Courts; c++ {
		if s.freeAt[c].Equal(never) {
			s.freeAt[c] = later(s.idleAt[c], t)
		}
	}
}

// relax drops the rules once they leave items no court can take: the
// courts left idle by them take items again from when they went idle. false
// when nothing changes.
func (s *sim) relax() bool {
	if s.relaxed || len(s.rules) == 0 {
		return false
	}
	s.relaxed = true
	for c := 1; c <= s.in.Courts; c++ {
		if s.freeAt[c].Equal(never) && !s.idleAt[c].IsZero() {
			s.freeAt[c] = s.idleAt[c]
		}
	}
	return true
}

// breakAt reports whether a match of that many minutes starting at t runs
// into a break, and when the latest such break ends.
func (s *sim) breakAt(t time.Time, minutes int) (time.Time, bool) {
	end := t.Add(time.Duration(minutes) * time.Minute)
	var out time.Time
	for _, b := range s.in.Breaks {
		if b.To.After(t) && (b.From.Before(end) || (minutes == 0 && !b.From.After(t))) {
			out = later(out, b.To)
		}
	}
	return out, !out.IsZero()
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
	if !at.IsZero() {
		at = at.Add(it.Pause)
	}
	at = later(at, it.NotBefore)
	if a := it.AfterMatch; a != "" && s.queued[a] {
		end, placed := s.ended[a]
		if !placed {
			return at, false
		}
		at = later(at, end)
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
	s.freeAt[c], s.idleAt[c] = never, t // nothing left for this court
	for _, f := range s.freeAt[1:] {
		if !f.Equal(never) {
			return true
		}
	}
	return false
}

// never marks a court with nothing left to play.
var never = time.Unix(math.MaxInt32, 0)

// place puts item i on court c at t.
func (s *sim) place(i, c int, t time.Time) Slot {
	it := &s.items[i]
	end := t.Add(time.Duration(it.Minutes) * time.Minute)
	slot := Slot{Item: *it, Court: c, Start: t, End: end, Unrested: !s.rested(it.A, t) || !s.rested(it.B, t)}
	s.freeAt[c] = end
	if it.Ghost != "" {
		return slot
	}
	slot.OffRules = s.relaxed && !s.ruleAllows(c, i)
	for _, r := range s.itemRules[i] {
		s.ruleLeft[r]--
	}
	if it.Match != "" {
		s.ended[it.Match] = end
	}
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
