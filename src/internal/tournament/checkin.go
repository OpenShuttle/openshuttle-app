package tournament

import (
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypePlayerCheckedIn  events.Type = "PlayerCheckedIn"
	TypeCheckInCancelled events.Type = "PlayerCheckInCancelled"
	TypeEntrantCheckedIn events.Type = "EntrantCheckedIn"
)

// PlayerCheckedIn is the payload of TypePlayerCheckedIn. Bib is set when this
// check-in assigns the entrant's bib (its first player to arrive), 0 otherwise.
type PlayerCheckedIn struct {
	Player PlayerID `json:"player"`
	Bib    int      `json:"bib,omitempty"`
}

// EntrantCheckedIn is the payload of TypeEntrantCheckedIn: the players of
// an entrant still absent, checked in at once. Bib is set when the entrant
// had none yet.
type EntrantCheckedIn struct {
	Entrant engine.EntrantID `json:"entrant"`
	Players []PlayerID       `json:"playerIds"`
	Bib     int              `json:"bib,omitempty"`
}

// CheckInCancelled is the payload of TypeCheckInCancelled. The bib stays
// assigned: numbers already written on paper must not move.
type CheckInCancelled struct {
	Player PlayerID `json:"player"`
}

// CheckInPlayer marks a player present. The first check-in of an entrant
// assigns its bib: the next number, unique across the tournament.
func (s State) CheckInPlayer(id PlayerID) (Change, error) {
	i, j := s.playerIndex(id)
	if i < 0 {
		return Change{}, engine.NewError(engine.CodePlayerNotFound)
	}
	if s.Entrants[i].Players[j].CheckedIn {
		return Change{}, nil
	}
	p := PlayerCheckedIn{Player: id}
	if s.Entrants[i].Bib == 0 {
		p.Bib = s.lastBib + 1
	}
	return Change{TypePlayerCheckedIn, p}, nil
}

// CheckInEntrant marks every absent player of an entrant present, in one
// action: a pair arriving together.
func (s State) CheckInEntrant(id engine.EntrantID) (Change, error) {
	e, ok := s.Entrant(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	p := EntrantCheckedIn{Entrant: id}
	for _, pl := range e.Players {
		if !pl.CheckedIn {
			p.Players = append(p.Players, pl.ID)
		}
	}
	if len(p.Players) == 0 {
		return Change{}, nil
	}
	if e.Bib == 0 {
		p.Bib = s.lastBib + 1
	}
	return Change{TypeEntrantCheckedIn, p}, nil
}

// CancelCheckIn marks a player absent again.
func (s State) CancelCheckIn(id PlayerID) (Change, error) {
	i, j := s.playerIndex(id)
	if i < 0 {
		return Change{}, engine.NewError(engine.CodePlayerNotFound)
	}
	if !s.Entrants[i].Players[j].CheckedIn {
		return Change{}, nil
	}
	return Change{TypeCheckInCancelled, CheckInCancelled{Player: id}}, nil
}

func (s *State) applyPlayerCheckedIn(p PlayerCheckedIn, at time.Time) {
	i, j := s.playerIndex(p.Player)
	if i < 0 {
		return
	}
	s.Entrants[i].Players[j].CheckedIn = true
	s.Entrants[i].Players[j].CheckedInAt = at
	if p.Bib != 0 && s.Entrants[i].Bib == 0 {
		// Normally p.Bib == lastBib+1. After an undo of an undo, the recorded
		// number may have been reused since: take the next free one.
		bib := max(p.Bib, s.lastBib+1)
		s.Entrants[i].Bib = bib
		s.lastBib = bib
	}
}

func (s *State) applyEntrantCheckedIn(p EntrantCheckedIn, at time.Time) {
	for i, id := range p.Players {
		bib := 0
		if i == 0 {
			bib = p.Bib
		}
		s.applyPlayerCheckedIn(PlayerCheckedIn{Player: id, Bib: bib}, at)
	}
}

func (s *State) applyCheckInCancelled(p CheckInCancelled) {
	if i, j := s.playerIndex(p.Player); i >= 0 {
		s.Entrants[i].Players[j].CheckedIn = false
		s.Entrants[i].Players[j].CheckedInAt = time.Time{}
	}
}
