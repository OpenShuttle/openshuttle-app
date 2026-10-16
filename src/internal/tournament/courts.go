package tournament

import (
	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeCourtsSet        events.Type = "CourtsSet"
	TypeMinRestSet       events.Type = "MinRestSet"
	TypeCourtPrioritySet events.Type = "CourtPrioritySet"
)

// Defaults and limits of the court settings.
const (
	DefaultCourts  = 4
	MaxCourts      = 20
	DefaultMinRest = 10 // minutes
	MaxMinRest     = 60
)

// PriorityBalanced gives free courts to the least advanced category, by share
// of matches played. Any other priority is a category code: that category first.
const PriorityBalanced = "balanced"

// CourtsSet is the payload of TypeCourtsSet.
type CourtsSet struct {
	Courts int `json:"courts"`
}

// MinRestSet is the payload of TypeMinRestSet.
type MinRestSet struct {
	Minutes int `json:"minutes"`
}

// CourtPrioritySet is the payload of TypeCourtPrioritySet.
type CourtPrioritySet struct {
	Priority string `json:"priority"`
}

// Courts returns the number of courts.
func (s State) Courts() int {
	if s.courts == 0 {
		return DefaultCourts
	}
	return s.courts
}

// MinRest returns the minimum rest between two matches of an entrant, in minutes.
func (s State) MinRest() int {
	if s.minRest == nil {
		return DefaultMinRest
	}
	return *s.minRest
}

// CourtPriority returns PriorityBalanced or the category played first.
func (s State) CourtPriority() string {
	if s.priority == "" || (s.priority != PriorityBalanced && !s.HasCategory(engine.Category(s.priority))) {
		return PriorityBalanced
	}
	return s.priority
}

// SetCourts sets the number of courts. A court with a match in progress
// cannot be removed.
func (s State) SetCourts(n int) (Change, error) {
	if n < 1 || n > MaxCourts {
		return Change{}, engine.NewError(engine.CodeInvalidCourts)
	}
	if n == s.Courts() {
		return Change{}, nil
	}
	for _, c := range s.CourtStates() {
		if c.Number > n && c.State == CourtPlaying {
			return Change{}, engine.NewError(engine.CodeCourtBusy)
		}
	}
	return Change{TypeCourtsSet, CourtsSet{Courts: n}}, nil
}

// SetMinRest sets the minimum rest, in minutes.
func (s State) SetMinRest(minutes int) (Change, error) {
	if minutes < 0 || minutes > MaxMinRest {
		return Change{}, engine.NewError(engine.CodeInvalidRest)
	}
	if minutes == s.MinRest() {
		return Change{}, nil
	}
	return Change{TypeMinRestSet, MinRestSet{Minutes: minutes}}, nil
}

// SetCourtPriority sets the court priority. It only reorders proposals: it
// never reserves a court.
func (s State) SetCourtPriority(p string) (Change, error) {
	if p != PriorityBalanced && !s.HasCategory(engine.Category(p)) {
		return Change{}, engine.NewError(engine.CodeInvalidPriority)
	}
	if p == s.CourtPriority() {
		return Change{}, nil
	}
	return Change{TypeCourtPrioritySet, CourtPrioritySet{Priority: p}}, nil
}
