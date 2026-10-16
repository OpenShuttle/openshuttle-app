package tournament

import (
	"math/rand/v2"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeScoreCleared      events.Type = "ScoreCleared"
	TypeEntrantReinstated events.Type = "EntrantReinstated"
	TypeMatchesSwapped    events.Type = "MatchesSwapped"
	TypeLotsDrawn         events.Type = "LotsDrawn"
)

// ScoreCleared is the payload of TypeScoreCleared: the match goes back to
// pending scores (or to the queue if it was never played).
type ScoreCleared struct {
	Match MatchID `json:"match"`
}

// EntrantReinstated is the payload of TypeEntrantReinstated: a forfeit is
// cancelled, its matches are playable again.
type EntrantReinstated struct {
	Entrant engine.EntrantID `json:"entrant"`
}

// MatchesSwapped is the payload of TypeMatchesSwapped: two matches trade
// places in the order courts take them.
type MatchesSwapped struct {
	A MatchID `json:"a"`
	B MatchID `json:"b"`
}

// LotsDrawn is the payload of TypeLotsDrawn: lots break the remaining ties of
// a pool. Lower lots rank first.
type LotsDrawn struct {
	Category engine.Category          `json:"category"`
	Pool     int                      `json:"pool"`
	Lots     map[engine.EntrantID]int `json:"lots"`
}

// ClearScore removes a recorded score.
func (s State) ClearScore(id MatchID) (Change, error) {
	m, ok := s.Match(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	if m.Score == nil {
		return Change{}, engine.NewError(engine.CodeNoScore)
	}
	return Change{TypeScoreCleared, ScoreCleared{Match: id}}, nil
}

// ReinstateEntrant cancels a forfeit.
func (s State) ReinstateEntrant(id engine.EntrantID) (Change, error) {
	if _, ok := s.Entrant(id); !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	if !s.withdrawn[id] {
		return Change{}, engine.NewError(engine.CodeEntrantNotWithdrawn)
	}
	return Change{TypeEntrantReinstated, EntrantReinstated{Entrant: id}}, nil
}

// SwapMatches swaps two waiting matches in the call order.
func (s State) SwapMatches(a, b MatchID) (Change, error) {
	ma, okA := s.Match(a)
	mb, okB := s.Match(b)
	if !okA || !okB {
		return Change{}, engine.NewError(engine.CodeMatchNotFound)
	}
	if ma.Called() || mb.Called() || s.Decided(ma) || s.Decided(mb) {
		return Change{}, engine.NewError(engine.CodeMatchStarted)
	}
	if a == b {
		return Change{}, nil
	}
	return Change{TypeMatchesSwapped, MatchesSwapped{A: a, B: b}}, nil
}

// DrawLots draws lots among the entrants of a complete pool still tied after
// every other criterion. rng is seeded by the caller; the event stores the lots.
func (s State) DrawLots(c engine.Category, pool int, rng *rand.Rand) (Change, error) {
	var tied []engine.EntrantID
	for _, row := range s.PoolStandings(c, pool).Rows {
		if row.DrawNeeded {
			tied = append(tied, row.Entrant)
		}
	}
	if len(tied) == 0 {
		return Change{}, engine.NewError(engine.CodeNoTie)
	}
	order := rng.Perm(len(tied))
	lots := map[engine.EntrantID]int{}
	for i, id := range tied {
		lots[id] = order[i] + 1
	}
	return Change{TypeLotsDrawn, LotsDrawn{Category: c, Pool: pool, Lots: lots}}, nil
}

func (s *State) applyScoreCleared(p ScoreCleared) {
	if i := s.matchIndex(p.Match); i >= 0 {
		s.matches[i].Score = nil
		if !s.matches[i].Called() {
			// Scored without being played on a court: back to the queue.
			s.matches[i].EndedAt = s.matches[i].CalledAt
		}
	}
}

func (s *State) applyEntrantReinstated(p EntrantReinstated) {
	delete(s.withdrawn, p.Entrant)
}

func (s *State) applyMatchesSwapped(p MatchesSwapped) {
	i, j := s.matchIndex(p.A), s.matchIndex(p.B)
	if i >= 0 && j >= 0 {
		s.matches[i], s.matches[j] = s.matches[j], s.matches[i]
	}
}

func (s *State) applyLotsDrawn(p LotsDrawn) {
	if s.lots == nil {
		s.lots = map[PoolRef]map[engine.EntrantID]int{}
	}
	ref := PoolRef{p.Category, p.Pool}
	lots := map[engine.EntrantID]int{}
	for id, lot := range s.lots[ref] {
		lots[id] = lot
	}
	// Later lots for the same entrants replace earlier ones, offset above
	// them so they never collide with lots of other tied groups.
	offset := 0
	for _, lot := range lots {
		offset = max(offset, lot)
	}
	ids := make([]engine.EntrantID, 0, len(p.Lots))
	for id := range p.Lots {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		lots[id] = offset + p.Lots[id]
	}
	s.lots[ref] = lots
}
