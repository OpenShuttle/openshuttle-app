package tournament

import (
	"fmt"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament/forecast"
)

const (
	// TypeGhostMatchAdded records a ghost match: a court left empty for a
	// while, in its turn in the schedule.
	TypeGhostMatchAdded events.Type = "GhostMatchAdded"
	// TypeGhostMatchRemoved removes a ghost match.
	TypeGhostMatchRemoved events.Type = "GhostMatchRemoved"
)

// MaxGhostMinutes is the longest ghost match.
const MaxGhostMinutes = 600

// GhostMatch is a court left empty for Minutes, after the match After (""
// for as soon as the court is free). It only shapes the schedule's forecast:
// the control desk never sees it.
type GhostMatch struct {
	ID      string
	Court   int
	Minutes int
	After   MatchID
	// AddedAt is when the desk added it: a ghost following no match is due
	// from then.
	AddedAt time.Time
}

// GhostMatchAdded is the payload of TypeGhostMatchAdded. The ghost's ID comes
// from the event seq ("g12").
type GhostMatchAdded struct {
	Court   int     `json:"court"`
	Minutes int     `json:"minutes"`
	After   MatchID `json:"after,omitempty"`
}

// GhostMatchRemoved is the payload of TypeGhostMatchRemoved.
type GhostMatchRemoved struct {
	ID string `json:"id"`
}

// AddGhostMatch leaves a court empty for some minutes after a match ("": as
// soon as it is free).
func (s State) AddGhostMatch(court, minutes int, after MatchID) (Change, error) {
	if court < 1 || court > s.Courts() {
		return Change{}, engine.NewError(engine.CodeInvalidCourt)
	}
	if minutes < 1 || minutes > MaxGhostMinutes {
		return Change{}, engine.NewError(engine.CodeInvalidTime)
	}
	if _, ok := s.Match(after); after != "" && !ok {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	return Change{TypeGhostMatchAdded, GhostMatchAdded{Court: court, Minutes: minutes, After: after}}, nil
}

// RemoveGhostMatch removes a ghost match.
func (s State) RemoveGhostMatch(id string) (Change, error) {
	if !slices.ContainsFunc(s.ghosts, func(g GhostMatch) bool { return g.ID == id }) {
		return Change{}, engine.NewError(engine.CodeInvalidSetting)
	}
	return Change{TypeGhostMatchRemoved, GhostMatchRemoved{ID: id}}, nil
}

func (s *State) applyGhostMatchAdded(p GhostMatchAdded, seq uint64, at time.Time) {
	s.ghosts = append(s.ghosts, GhostMatch{ID: fmt.Sprintf("g%d", seq), Court: p.Court, Minutes: p.Minutes, After: p.After, AddedAt: at})
}

func (s *State) applyGhostMatchRemoved(p GhostMatchRemoved) {
	s.ghosts = slices.DeleteFunc(s.ghosts, func(g GhostMatch) bool { return g.ID == p.ID })
}

// GhostMatches returns the ghost matches still ahead, in the order they were
// added. A ghost is over once the desk calls a match on its court after it
// was due: after its match ended, or after it was added when it follows no
// match (or a match no longer played). A court no longer there drops it.
func (s State) GhostMatches() []GhostMatch {
	out := []GhostMatch{}
	for _, g := range s.ghosts {
		if g.Court > s.Courts() {
			continue
		}
		due, _, _ := s.ghostDue(g)
		if !due.IsZero() && slices.ContainsFunc(s.matches, func(m Match) bool {
			return m.Court == g.Court && !m.CalledAt.Before(due)
		}) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// ghostDue tells when a ghost match is due: zero while the match it follows
// is waiting (queued: it is placed after it) or playing (playing: the
// match).
func (s State) ghostDue(g GhostMatch) (due time.Time, queued bool, playing *Match) {
	m, ok := s.Match(g.After)
	switch {
	case g.After == "" || !ok:
	case s.Playing(m):
		return time.Time{}, false, &m
	case m.Ended():
		return m.EndedAt, false, nil
	case !m.Called() && !s.Decided(m):
		return time.Time{}, true, nil
	}
	return g.AddedAt, false, nil
}

// withGhosts puts the ghost matches in the items: right after the match each
// follows, the others first.
func (s State) withGhosts(items []forecast.Item, now time.Time) []forecast.Item {
	var first []forecast.Item
	after := map[string][]forecast.Item{}
	for _, g := range s.GhostMatches() {
		it := forecast.Item{Ghost: g.ID, Court: g.Court, Minutes: g.Minutes}
		due, queued, playing := s.ghostDue(g)
		switch {
		case queued:
			it.AfterMatch = string(g.After)
			after[it.AfterMatch] = append(after[it.AfterMatch], it)
			continue
		case playing != nil:
			it.NotBefore = s.estimatedEnd(*playing, now)
		case g.After != "":
			it.NotBefore = due
		}
		first = append(first, it)
	}
	out := first
	for _, it := range items {
		out = append(out, it)
		if it.Match != "" {
			out = append(out, after[it.Match]...)
		}
	}
	return out
}

// estimatedEnd is when a match on court should end: at its estimate, any
// minute when running late.
func (s State) estimatedEnd(m Match, now time.Time) time.Time {
	end := m.CalledAt.Add(time.Duration(s.WinCondition(m).EstimatedMinutes()) * time.Minute)
	if !end.After(now) {
		end = now.Add(time.Minute)
	}
	return end
}
