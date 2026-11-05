package tournament

import (
	"maps"
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

// ScheduleBlocksSet is the payload of TypeScheduleBlocksSet: the blocks in
// the order they are played, "MD/S1/A/2" for round 2 of pool A, "MD/S2/R1"
// for a bracket round. Files written before held pools ("MD/S1/A"): their
// order then applies within each wave.
type ScheduleBlocksSet struct {
	Order []string `json:"order"`
}

// ScheduleOrderSet is the payload of TypeScheduleOrderSet: the waiting
// matches in the order the courts take them. Its event time is when the
// schedule was validated or last changed. An empty order un-validates it.
type ScheduleOrderSet struct {
	Order []MatchID `json:"order"`
}

// legacyReserved is the mode of older files that gave each category its
// own courts ("reserved"): court rules replaced it, the courts reserved by
// hand read as only rules.
const legacyReserved = "reserved"

// ScheduleSet is the payload of TypeScheduleSet: the whole schedule setup.
type ScheduleSet struct {
	// Mode was "alternate" or "reserved" in older files; the categories now
	// always alternate, and court rules steer the courts.
	Mode string `json:"mode,omitempty"`
	// First opens every round ("" the first category).
	First engine.Category `json:"first,omitempty"`
	// Courts reserved courts for a category in reserved mode in files
	// written before court rules: each reads as an only rule (courtRules).
	Courts map[int]engine.Category `json:"courts,omitempty"`
	// Rules are each court's rules (ADR 12): priorities, only, never.
	Rules map[int][]CourtRule `json:"rules,omitempty"`
	// Pinned held blocks moved up by hand in files written before the
	// schedule could be validated; it is ignored.
	Pinned []string `json:"pinned,omitempty"`
}

// ScheduleSettings returns the schedule setup, defaults filled in.
func (s State) ScheduleSettings() ScheduleSet {
	out := s.schedule
	out.Mode = ""
	if !s.HasCategory(out.First) {
		out.First = ""
		// The court priority of older files picks the category first.
		if c := engine.Category(s.priority); s.HasCategory(c) {
			out.First = c
		}
	}
	out.Courts = nil
	out.Rules = s.courtRules(s.schedule)
	out.Pinned = nil
	return out
}

// SetScheduleFirst chooses the category opening each round.
func (s State) SetScheduleFirst(c engine.Category) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	return s.scheduleChange(func(p *ScheduleSet) { p.First = c })
}

func (s State) scheduleChange(edit func(*ScheduleSet)) (Change, error) {
	cur := s.ScheduleSettings()
	next := cur
	next.Rules = maps.Clone(cur.Rules)
	edit(&next)
	maps.DeleteFunc(next.Rules, func(_ int, rs []CourtRule) bool { return len(rs) == 0 })
	if next.First == cur.First && maps.EqualFunc(next.Rules, cur.Rules, slices.Equal) {
		return Change{}, nil
	}
	if len(next.Rules) == 0 {
		next.Rules = nil
	}
	return Change{TypeScheduleSet, next}, nil
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

// categoryOrder ranks the categories in each round: the first one set,
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

// openingOrder lists the categories in the order they open each round.
func (s State) openingOrder() []engine.Category {
	order := s.categoryOrder(s.ScheduleSettings())
	cats := slices.Clone(s.Categories)
	slices.SortStableFunc(cats, func(a, b engine.Category) int { return order[a] - order[b] })
	return cats
}
