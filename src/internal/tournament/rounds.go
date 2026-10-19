package tournament

import (
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeRoundStarted events.Type = "RoundStarted"
	TypeRoundEnded   events.Type = "RoundEnded"
)

// Timed matches are played in rounds: every court starts and ends together,
// with one shared countdown. The desk starts a round, then whistles its end;
// nothing ends on its own at 0:00.

// RoundCall is a match of a round and its court.
type RoundCall struct {
	Match MatchID `json:"match"`
	Court int     `json:"court"`
}

// RoundStarted is the payload of TypeRoundStarted. The start time is the
// event time; Minutes is the duration shown by the countdown.
type RoundStarted struct {
	Number  int         `json:"number"`
	Minutes int         `json:"minutes"`
	Calls   []RoundCall `json:"calls"`
}

// RoundEnded is the payload of TypeRoundEnded. The end time is the event time.
type RoundEnded struct {
	Number int `json:"number"`
}

// Round is a round of timed matches.
type Round struct {
	Number    int
	Minutes   int
	Calls     []RoundCall
	StartedAt time.Time
	EndedAt   time.Time // zero while in progress
}

// InProgress reports whether the round has not been whistled yet.
func (r Round) InProgress() bool { return r.EndedAt.IsZero() }

// Timed reports whether a match is won by time.
func (s State) Timed(m Match) bool {
	return s.WinCondition(m).Kind == engine.WinByTime
}

// Rounds returns every round, oldest first.
func (s State) Rounds() []Round {
	return slices.Clone(s.rounds)
}

// CurrentRound returns the round in progress, if any.
func (s State) CurrentRound() (Round, bool) {
	if n := len(s.rounds); n > 0 && s.rounds[n-1].InProgress() {
		return s.rounds[n-1], true
	}
	return Round{}, false
}

// NextRound proposes the matches of the next round, one per free court:
// waiting timed matches in queue order, nobody twice, entrants who sat out
// the previous round first. The minimum rest does not apply between rounds.
func (s State) NextRound() []RoundCall {
	var free []int
	for _, c := range s.CourtStates() {
		if c.State != CourtPlaying {
			free = append(free, c.Number)
		}
	}
	if len(free) == 0 {
		return nil
	}

	playing := map[engine.EntrantID]bool{}
	for _, m := range s.matches {
		if s.Playing(m) {
			playing[m.A], playing[m.B] = true, true
		}
	}
	lastRound := map[engine.EntrantID]bool{}
	if n := len(s.rounds); n > 0 {
		for _, call := range s.rounds[n-1].Calls {
			if m, ok := s.Match(call.Match); ok {
				lastRound[m.A], lastRound[m.B] = true, true
			}
		}
	}

	var candidates []Match
	for _, m := range s.orderedWaiting(s.lastAt) {
		if !s.Timed(m) || playing[m.A] || playing[m.B] {
			continue
		}
		a, _ := s.Entrant(m.A)
		b, _ := s.Entrant(m.B)
		if !a.Complete() || !b.Complete() {
			continue
		}
		candidates = append(candidates, m)
	}
	// Tier 2: both entrants sat out the previous round, then 1, then 0.
	rested := func(m Match) int {
		n := 0
		for _, id := range []engine.EntrantID{m.A, m.B} {
			if !lastRound[id] {
				n++
			}
		}
		return n
	}
	slices.SortStableFunc(candidates, func(x, y Match) int { return rested(y) - rested(x) })

	taken := map[engine.EntrantID]bool{}
	var calls []RoundCall
	for _, m := range candidates {
		if len(calls) == len(free) {
			break
		}
		if taken[m.A] || taken[m.B] {
			continue
		}
		taken[m.A], taken[m.B] = true, true
		calls = append(calls, RoundCall{Match: m.ID, Court: free[len(calls)]})
	}
	return calls
}

// StartRound calls the next round on every free court at once.
func (s State) StartRound() (Change, error) {
	if _, ok := s.CurrentRound(); ok {
		return Change{}, engine.NewError(engine.CodeRoundInProgress)
	}
	calls := s.NextRound()
	if len(calls) == 0 {
		return Change{}, engine.NewError(engine.CodeNoMatchReady)
	}
	first, _ := s.Match(calls[0].Match)
	return Change{TypeRoundStarted, RoundStarted{
		Number:  len(s.rounds) + 1,
		Minutes: s.WinCondition(first).Minutes,
		Calls:   calls,
	}}, nil
}

// EndRound whistles the end of the round in progress: its matches still on
// court end and go to pending scores. It can be done before 0:00.
func (s State) EndRound() (Change, error) {
	r, ok := s.CurrentRound()
	if !ok {
		return Change{}, engine.NewError(engine.CodeNoRound)
	}
	return Change{TypeRoundEnded, RoundEnded{Number: r.Number}}, nil
}

func (s *State) applyRoundStarted(p RoundStarted, at time.Time, seq uint64) {
	for _, call := range p.Calls {
		s.applyMatchCalled(MatchCalled{Match: call.Match, Court: call.Court}, at, seq)
	}
	s.rounds = append(s.rounds, Round{Number: p.Number, Minutes: p.Minutes, Calls: slices.Clone(p.Calls), StartedAt: at})
}

func (s *State) applyRoundEnded(p RoundEnded, at time.Time) {
	for i := range s.rounds {
		if s.rounds[i].Number == p.Number && s.rounds[i].InProgress() {
			s.rounds[i].EndedAt = at
			for _, call := range s.rounds[i].Calls {
				s.applyMatchEnded(MatchEnded{Match: call.Match}, at)
			}
		}
	}
}
