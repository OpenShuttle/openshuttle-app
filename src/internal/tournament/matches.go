package tournament

import (
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

// MatchID identifies a match: category, stage (but the entry stage's),
// then pool and number, or the bracket match number: "MD-A3", "MD-S2-5".
type MatchID string

// Match is a match of a stage. Matches are not events: they are derived from
// the filled stages. Events record what happens to them.
type Match struct {
	ID       MatchID
	Category engine.Category
	Stage    string
	Pool     int // 0 is A; -1 for a bracket match
	Round    int // 0-based round within the pool
	A, B     engine.EntrantID

	// Bracket matches: the sub-bracket whose first round this match is. A and
	// B are "" until known; ByeA/ByeB mark byes.
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

// EntrantWithdrawn is the payload of TypeEntrantWithdrawn: a forfeit,
// declared during Stage. Every match of the entrant in that stage and the
// later ones becomes a walkover, played or not; earlier stages keep their
// results.
type EntrantWithdrawn struct {
	Entrant engine.EntrantID `json:"entrant"`
	Stage   string           `json:"stage"`
}

// IsBracket reports whether a match belongs to a bracket stage.
func (m Match) IsBracket() bool { return m.Pool < 0 }

// Matches returns every match, in schedule order: stage by stage, round by
// round with pools interleaved, categories in confirmation order.
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

// Withdrawn reports whether an entrant forfeited.
func (s State) Withdrawn(id engine.EntrantID) bool { return s.withdrawn[id] != "" }

// withdrawnBy reports whether an entrant forfeited in the stage at position
// si of its category, or before.
func (s State) withdrawnBy(id engine.EntrantID, c engine.Category, si int) bool {
	w := s.withdrawn[id]
	return w != "" && s.stageIndex(c, w) <= si
}

// withdrawnFrom reports whether a forfeit applies to a match of the entrant.
func (s State) withdrawnFrom(id engine.EntrantID, m Match) bool {
	return id != "" && s.withdrawnBy(id, m.Category, s.stageIndex(m.Category, m.Stage))
}

// Walkover reports whether a match is a walkover: one side forfeited in this
// stage or an earlier one.
func (s State) Walkover(m Match) bool {
	return s.Playable(m) && (s.withdrawnFrom(m.A, m) || s.withdrawnFrom(m.B, m))
}

// Decided reports whether a match has a result: a score or a walkover.
func (s State) Decided(m Match) bool { return m.Score != nil || s.Walkover(m) }

// Playing reports whether a match is in progress on its court.
func (s State) Playing(m Match) bool {
	return m.Called() && !m.Ended() && !s.Walkover(m)
}

// WinCondition returns the win condition of a match.
func (s State) WinCondition(m Match) engine.WinCondition {
	if st := s.stage(m.Category, m.Stage); st != nil {
		return st.Win
	}
	return DefaultPoolsStage("").Win
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
	if m.IsBracket() && s.nextStarted(m) {
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

// WithdrawEntrant declares a forfeit for an entrant, in the latest stage it
// plays.
func (s State) WithdrawEntrant(id engine.EntrantID) (Change, error) {
	e, ok := s.Entrant(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	stage, latest := "", -1
	for _, m := range s.matches {
		if m.Has(id) {
			if i := s.stageIndex(e.Category, m.Stage); i > latest {
				stage, latest = m.Stage, i
			}
		}
	}
	if stage == "" {
		return Change{}, engine.NewError(engine.CodeEntrantNotInPools)
	}
	if s.withdrawn[id] != "" {
		return Change{}, nil
	}
	return Change{TypeEntrantWithdrawn, EntrantWithdrawn{Entrant: id, Stage: stage}}, nil
}

// MatchesStarted reports whether a category has a match called, scored or
// walked over. Its draw can then no longer be reopened.
func (s State) MatchesStarted(c engine.Category) bool {
	return slices.ContainsFunc(s.matches, func(m Match) bool {
		return m.Category == c && (m.Called() || s.Decided(m))
	})
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
	s.withdrawn[p.Entrant] = p.Stage
	// A match in progress stops: the court is free again.
	for i, m := range s.matches {
		if m.Has(p.Entrant) && m.Called() && m.EndedAt.IsZero() {
			s.matches[i].EndedAt = at
		}
	}
}
