package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// ActionView describes a log entry for display. The frontend translates
// Type ("history.<Type>") with Params.
type ActionView struct {
	Seq      uint64            `json:"seq"`
	Type     string            `json:"type"`
	At       time.Time         `json:"at"`
	Operator string            `json:"operator"`
	Params   map[string]string `json:"params"`
}

// describe turns an event into an ActionView. An undo describes its target.
func (s *TournamentService) describe(e events.Event) *ActionView {
	v := &ActionView{Seq: e.Seq, Type: string(e.Type), At: e.At, Operator: e.Operator, Params: map[string]string{}}
	var p struct {
		Number   int                    `json:"number"`
		Calls    []tournament.RoundCall `json:"calls"`
		Stage    string                 `json:"stage"`
		A        tournament.MatchID     `json:"a"`
		B        tournament.MatchID     `json:"b"`
		Match    tournament.MatchID     `json:"match"`
		Court    int                    `json:"court"`
		Courts   int                    `json:"courts"`
		Minutes  int                    `json:"minutes"`
		Priority string                 `json:"priority"`
		Score    engine.Score           `json:"score"`
		Pool     int                    `json:"pool"`
		Seed     int                    `json:"seed"`
		Entrant  engine.EntrantID       `json:"entrant"`
		Name     string                 `json:"name"`
		Category engine.Category        `json:"category"`
		ID       engine.EntrantID       `json:"id"`
		Player   tournament.PlayerID    `json:"player"`
		Bib      int                    `json:"bib"`
		Players  []tournament.Player    `json:"players"`
	}
	if e.Type != events.TypeUndone && e.Decode(&p) != nil {
		return v
	}
	switch e.Type {
	case tournament.TypeCreated, tournament.TypeRenamed:
		v.Params["name"] = p.Name
	case tournament.TypeCategoryAdded, tournament.TypeCategoryRemoved, tournament.TypeFormatSet, tournament.TypeSeedsSet,
		tournament.TypeEntryDrawn, tournament.TypeDrawConfirmed, tournament.TypeDrawReopened:
		v.Params["category"] = string(p.Category)
	case tournament.TypeMatchCalled, tournament.TypeMatchEnded, tournament.TypeScoreRecorded, tournament.TypeScoreCleared:
		v.Params["a"], v.Params["b"] = matchLabel(s.state, p.Match)
		if m, ok := s.state.Match(p.Match); ok {
			v.Params["court"] = itoa(m.Court)
		}
		if e.Type == tournament.TypeMatchCalled {
			v.Params["court"] = itoa(p.Court)
		}
		if e.Type == tournament.TypeScoreRecorded {
			v.Params["score"] = scoreLabel(p.Score)
		}
	case tournament.TypeEntrantWithdrawn, tournament.TypeEntrantReinstated:
		v.Params["names"] = s.entrantNamesAt(p.Entrant, e.Seq)
	case tournament.TypeStageFilled, tournament.TypeSlotsSwapped:
		v.Params["category"] = string(p.Category)
		v.Params["stage"] = s.stageNameAt(p.Category, p.Stage, e.Seq)
	case tournament.TypeMatchesSwapped:
		v.Params["first"], v.Params["second"] = string(p.A), string(p.B)
	case tournament.TypeLotsDrawn:
		v.Params["category"] = string(p.Category)
		v.Params["pool"] = engine.PoolName(p.Pool)
	case tournament.TypeRoundStarted:
		v.Params["number"] = itoa(p.Number)
		v.Params["count"] = itoa(len(p.Calls))
	case tournament.TypeRoundEnded:
		v.Params["number"] = itoa(p.Number)
	case tournament.TypeCourtsSet:
		v.Params["courts"] = itoa(p.Courts)
	case tournament.TypeMinRestSet:
		v.Params["minutes"] = itoa(p.Minutes)
	case tournament.TypeCourtPrioritySet:
		v.Params["priority"] = p.Priority
	case tournament.TypeEntrantSeeded:
		v.Params["names"] = s.entrantNamesAt(p.ID, e.Seq)
		v.Params["seed"] = itoa(p.Seed)
	case tournament.TypeEntrantMoved:
		v.Params["names"] = s.entrantNamesAt(p.Entrant, e.Seq)
		v.Params["pool"] = engine.PoolName(p.Pool)
	case tournament.TypeEntrantRegistered, tournament.TypeEntrantUpdated:
		v.Params["names"] = playerNames(p.Players)
	case tournament.TypeEntrantRemoved:
		v.Params["names"] = s.entrantNamesAt(p.ID, e.Seq)
	case tournament.TypeEntrantsImported:
		var imp tournament.EntrantsImported
		if e.Decode(&imp) == nil {
			v.Params["count"] = itoa(len(imp.Entrants))
		}
	case tournament.TypeEntrantCheckedIn:
		v.Params["names"] = s.entrantNamesAt(p.Entrant, e.Seq)
	case tournament.TypePlayerCheckedIn, tournament.TypeCheckInCancelled:
		v.Params["name"] = s.playerNameAt(p.Player, e.Seq)
	case events.TypeUndone:
		var u events.Undone
		if e.Decode(&u) == nil && u.Target >= 1 && int(u.Target) <= len(s.log) {
			target := s.describe(s.log[u.Target-1])
			v.Params = target.Params
			v.Params["action"] = target.Type
		}
	}
	return v
}

// registrations walks the log (undone events included, so history can name
// removed entrants) before seq, newest first.
func (s *TournamentService) registrations(before uint64, visit func(engine.EntrantID, []tournament.Player) bool) {
	for i := min(int(before), len(s.log)) - 1; i >= 0; i-- {
		e := s.log[i]
		if e.Type == tournament.TypeEntrantsImported {
			var imp tournament.EntrantsImported
			if e.Decode(&imp) == nil {
				for _, r := range imp.Entrants {
					if !visit(r.ID, r.Players) {
						return
					}
				}
			}
			continue
		}
		if e.Type != tournament.TypeEntrantRegistered && e.Type != tournament.TypeEntrantUpdated {
			continue
		}
		var p struct {
			ID      engine.EntrantID    `json:"id"`
			Players []tournament.Player `json:"players"`
		}
		if e.Decode(&p) == nil && !visit(p.ID, p.Players) {
			return
		}
	}
}

func (s *TournamentService) entrantNamesAt(id engine.EntrantID, seq uint64) string {
	names := ""
	s.registrations(seq, func(eid engine.EntrantID, players []tournament.Player) bool {
		if eid == id {
			names = playerNames(players)
			return false
		}
		return true
	})
	return names
}

func (s *TournamentService) playerNameAt(id tournament.PlayerID, seq uint64) string {
	name := ""
	s.registrations(seq, func(_ engine.EntrantID, players []tournament.Player) bool {
		for _, p := range players {
			if p.ID == id {
				name = p.Name
				return false
			}
		}
		return true
	})
	return name
}

func playerNames(players []tournament.Player) string {
	names := make([]string, len(players))
	for i, p := range players {
		names[i] = p.Name
	}
	return strings.Join(names, " / ")
}

func itoa(n int) string { return strconv.Itoa(n) }

// stageNameAt returns the name of a stage as set before seq, or its ID when
// it has none, so that history keeps naming removed or renamed stages.
func (s *TournamentService) stageNameAt(c engine.Category, id string, seq uint64) string {
	name := id
	for _, e := range s.log[:min(int(seq), len(s.log))] {
		if e.Type != tournament.TypeFormatSet {
			continue
		}
		var p tournament.FormatSet
		if e.Decode(&p) != nil || p.Category != c {
			continue
		}
		for _, st := range p.Stages {
			if st.ID == id && st.Name != "" {
				name = st.Name
			}
		}
	}
	return name
}
