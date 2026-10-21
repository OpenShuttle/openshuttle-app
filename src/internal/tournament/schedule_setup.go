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
	if next.Mode == cur.Mode && next.First == cur.First && maps.Equal(next.Courts, cur.Courts) {
		return Change{}, nil
	}
	if len(next.Courts) == 0 {
		next.Courts = nil
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

// openingOrder lists the categories in the order they open each round.
func (s State) openingOrder() []engine.Category {
	order := s.categoryOrder(s.ScheduleSettings())
	cats := slices.Clone(s.Categories)
	slices.SortStableFunc(cats, func(a, b engine.Category) int { return order[a] - order[b] })
	return cats
}
