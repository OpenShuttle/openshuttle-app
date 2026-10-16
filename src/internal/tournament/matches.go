package tournament

import (
	"fmt"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeMatchCalled      events.Type = "MatchCalled"
	TypeMatchEnded       events.Type = "MatchEnded"
	TypeScoreRecorded    events.Type = "ScoreRecorded"
	TypeEntrantWithdrawn events.Type = "EntrantWithdrawn"
)

// MatchID identifies a match: category, pool and number, like "MD-A3".
type MatchID string

// Match is a pool match. Matches are not events: they are derived from the
// confirmed draw. Events record what happens to them.
type Match struct {
	ID       MatchID
	Category engine.Category
	Pool     int // 0 is A; -1 for a bracket match
	Round    int // 0-based round within the pool
	A, B     engine.EntrantID

	// Bracket matches: the bracket and the sub-bracket whose first round
	// this match is. A and B are "" until known; ByeA/ByeB mark byes.
	Bracket     string
	Start, Size int
	Index       int
	ByeA, ByeB  bool

	Court    int // 0 until called
	CalledAt time.Time
	callSeq  uint64 // seq of the call event: orders calls strictly
	// EndedAt is set by the desk ending the match, or by its score when
	// entered first. Rest counts from it.
	EndedAt time.Time
	Score   *engine.Score
}

// Called reports whether the match has been called on a court.
func (m Match) Called() bool { return m.Court > 0 }

// Ended reports whether the match is over on court.
func (m Match) Ended() bool { return !m.EndedAt.IsZero() }

// Has reports whether an entrant plays this match.
func (m Match) Has(id engine.EntrantID) bool { return m.A == id || m.B == id }

// MatchCalled is the payload of TypeMatchCalled. The call time is the event time.
type MatchCalled struct {
	Match MatchID `json:"match"`
	Court int     `json:"court"`
}

// MatchEnded is the payload of TypeMatchEnded. The end time is the event time.
type MatchEnded struct {
	Match MatchID `json:"match"`
}

// ScoreRecorded is the payload of TypeScoreRecorded. A new score for the same
// match replaces the previous one.
type ScoreRecorded struct {
	Match MatchID      `json:"match"`
	Score engine.Score `json:"score"`
}

// EntrantWithdrawn is the payload of TypeEntrantWithdrawn: a forfeit. In a
// pool, all the entrant's matches become walkovers, played or not.
type EntrantWithdrawn struct {
	Entrant engine.EntrantID `json:"entrant"`
	// Scope is ScopePools or ScopeBrackets; empty in pools for older files.
	Scope string `json:"scope,omitempty"`
}

// Matches returns every pool match, in schedule order: round by round,
// pools interleaved, categories in confirmation order.
func (s State) Matches() []Match {
	return slices.Clone(s.matches)
}

// Match returns a match by ID.
func (s State) Match(id MatchID) (Match, bool) {
	i := s.matchIndex(id)
	if i < 0 {
		return Match{}, false
	}
	return s.matches[i], true
}

func (s State) matchIndex(id MatchID) int {
	return slices.IndexFunc(s.matches, func(m Match) bool { return m.ID == id })
}

// Forfeit scopes: a forfeit in the pools turns every pool match of the
// entrant into a walkover; a forfeit once the brackets are drawn only loses
// the bracket matches, pool results stay.
const (
	ScopePools    = "pools"
	ScopeBrackets = "brackets"
)

// Withdrawn reports whether an entrant forfeited.
func (s State) Withdrawn(id engine.EntrantID) bool { return s.withdrawn[id] != "" }

// Walkover reports whether a match is a walkover: one side withdrew.
func (s State) Walkover(m Match) bool {
	if m.Bracket == "" {
		return s.withdrawn[m.A] == ScopePools || s.withdrawn[m.B] == ScopePools
	}
	return s.Playable(m) && (s.withdrawn[m.A] != "" || s.withdrawn[m.B] != "")
}

// Decided reports whether a match has a result: a score or a walkover.
func (s State) Decided(m Match) bool { return m.Score != nil || s.Walkover(m) }

// Playing reports whether a match is in progress on its court.
func (s State) Playing(m Match) bool {
	return m.Called() && !m.Ended() && !s.Walkover(m)
}

// WinCondition returns the win condition of a match.
func (s State) WinCondition(m Match) engine.WinCondition {
	if m.Bracket != "" {
		if b := s.bracket(m.Category, m.Bracket); b != nil {
			return b.Win
		}
	}
	return s.PoolStage(m.Category).Win
}

// CallMatch calls a match on a court. The court must not have a match in
// progress, and neither entrant may be playing elsewhere: both are physically
// impossible. Calling on a finished court sends its match to pending scores.
func (s State) CallMatch(id MatchID, court int) (Change, error) {
	m, ok := s.Match(id)
	switch {
	case !ok:
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	case !s.Playable(m):
		return Change{}, engine.NewError(engine.CodeMatchNotReady)
	case s.Walkover(m):
		return Change{}, engine.NewError(engine.CodeMatchWalkover)
	case m.Called() || m.Score != nil:
		return Change{}, engine.NewError(engine.CodeMatchStarted)
	case court < 1 || court > s.Courts():
		return Change{}, engine.NewError(engine.CodeInvalidCourt)
	}
	for _, c := range s.CourtStates() {
		if c.Number == court && c.State == CourtPlaying {
			return Change{}, engine.NewError(engine.CodeCourtBusy)
		}
	}
	for _, other := range s.matches {
		if s.Playing(other) && (other.Has(m.A) || other.Has(m.B)) {
			return Change{}, engine.NewError(engine.CodeEntrantPlaying)
		}
	}
	return Change{TypeMatchCalled, MatchCalled{Match: id, Court: court}}, nil
}

// EndMatch ends a match in progress. The score can be entered later.
func (s State) EndMatch(id MatchID) (Change, error) {
	m, ok := s.Match(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	if !s.Playing(m) {
		return Change{}, engine.NewError(engine.CodeMatchNotPlaying)
	}
	return Change{TypeMatchEnded, MatchEnded{Match: id}}, nil
}

// RecordScore records or corrects the score of a match. A match in progress
// ends with it.
func (s State) RecordScore(id MatchID, score engine.Score) (Change, error) {
	m, ok := s.Match(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	if !s.Playable(m) {
		return Change{}, engine.NewError(engine.CodeMatchNotReady)
	}
	if s.Walkover(m) {
		return Change{}, engine.NewError(engine.CodeMatchWalkover)
	}
	winner, err := engine.ValidateScore(s.WinCondition(m), score)
	if err != nil {
		return Change{}, err
	}
	if m.Bracket != "" && s.nextStarted(m) {
		was := s.winnerOf(m)
		now := m.A
		if winner == engine.SideB {
			now = m.B
		}
		if was != now {
			// The next round was played by the other entrant.
			return Change{}, engine.NewError(engine.CodeBracketNextStarted)
		}
	}
	score.Sets = slices.Clone(score.Sets)
	return Change{TypeScoreRecorded, ScoreRecorded{Match: id, Score: score}}, nil
}

// WithdrawEntrant declares a forfeit for an entrant of the pools.
func (s State) WithdrawEntrant(id engine.EntrantID) (Change, error) {
	if _, ok := s.Entrant(id); !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	if !slices.ContainsFunc(s.matches, func(m Match) bool { return m.Has(id) }) {
		return Change{}, engine.NewError(engine.CodeEntrantNotInPools)
	}
	if s.withdrawn[id] != "" {
		return Change{}, nil
	}
	scope := ScopePools
	if slices.ContainsFunc(s.matches, func(m Match) bool { return m.Bracket != "" && m.Has(id) }) {
		scope = ScopeBrackets
	}
	return Change{TypeEntrantWithdrawn, EntrantWithdrawn{Entrant: id, Scope: scope}}, nil
}

// MatchesStarted reports whether a category has a match called, scored or
// walked over. Its draw can then no longer be reopened.
func (s State) MatchesStarted(c engine.Category) bool {
	return slices.ContainsFunc(s.matches, func(m Match) bool {
		return m.Category == c && (m.Called() || s.Decided(m))
	})
}

// generateMatches builds the round-robin matches of a confirmed draw.
func (s *State) generateMatches(c engine.Category) {
	d := s.draws[c]
	if d == nil {
		return
	}
	schedules := make([][][][2]int, len(d.Pools))
	maxRounds := 0
	for p, pool := range d.Pools {
		schedules[p] = engine.RoundRobinSchedule(len(pool))
		maxRounds = max(maxRounds, len(schedules[p]))
	}
	counts := make([]int, len(d.Pools))
	for r := range maxRounds {
		for p, pool := range d.Pools {
			if r >= len(schedules[p]) {
				continue
			}
			for _, pair := range schedules[p][r] {
				counts[p]++
				s.matches = append(s.matches, Match{
					ID:       MatchID(fmt.Sprintf("%s-%s%d", c, engine.PoolName(p), counts[p])),
					Category: c,
					Pool:     p,
					Round:    r,
					A:        pool[pair[0]],
					B:        pool[pair[1]],
				})
			}
		}
	}
}

func (s *State) dropMatches(c engine.Category) {
	s.matches = slices.DeleteFunc(s.matches, func(m Match) bool { return m.Category == c && m.Bracket == "" })
}

func (s *State) applyMatchCalled(p MatchCalled, at time.Time, seq uint64) {
	if i := s.matchIndex(p.Match); i >= 0 {
		s.matches[i].Court = p.Court
		s.matches[i].CalledAt = at
		s.matches[i].callSeq = seq
	}
}

func (s *State) applyMatchEnded(p MatchEnded, at time.Time) {
	if i := s.matchIndex(p.Match); i >= 0 && s.matches[i].EndedAt.IsZero() {
		s.matches[i].EndedAt = at
	}
}

func (s *State) applyScoreRecorded(p ScoreRecorded, at time.Time) {
	if i := s.matchIndex(p.Match); i >= 0 {
		score := p.Score
		s.matches[i].Score = &score
		if s.matches[i].EndedAt.IsZero() {
			s.matches[i].EndedAt = at
		}
	}
}

func (s *State) applyEntrantWithdrawn(p EntrantWithdrawn, at time.Time) {
	if s.withdrawn == nil {
		s.withdrawn = map[engine.EntrantID]string{}
	}
	scope := p.Scope
	if scope == "" {
		scope = ScopePools
	}
	s.withdrawn[p.Entrant] = scope
	// A match in progress stops: the court is free again.
	for i, m := range s.matches {
		if m.Has(p.Entrant) && m.Called() && m.EndedAt.IsZero() {
			s.matches[i].EndedAt = at
		}
	}
}
