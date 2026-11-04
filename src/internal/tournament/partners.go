package tournament

import (
	"fmt"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// Players registered alone in a doubles category wait for a partner (see
// Entrant.Waiting). On the tournament day the desk pairs two of them, adds a
// partner who was not registered, or splits a pair back into two waiting
// players. A waiting player gets no bib: the pair gets one once formed, when
// one of its players is present.
const (
	TypeEntrantsPaired events.Type = "EntrantsPaired"
	TypePartnerAdded   events.Type = "PartnerAdded"
	TypeEntrantSplit   events.Type = "EntrantSplit"
)

// EntrantsPaired is the payload of TypeEntrantsPaired: the player of Partner
// joins the entrant ID, and Partner is gone. Players is the resulting pair,
// for history. Bib is set when the pair gets a new number.
type EntrantsPaired struct {
	ID      engine.EntrantID `json:"id"`
	Partner engine.EntrantID `json:"partner"`
	Players []Player         `json:"players"`
	Bib     int              `json:"bib,omitempty"`
}

// PartnerAdded is the payload of TypePartnerAdded: a new player, not
// registered before, joins a waiting entrant. Players is the resulting pair,
// the new player last (ID "p<seq>.1"). Bib is set when the pair gets a new
// number.
type PartnerAdded struct {
	ID      engine.EntrantID `json:"id"`
	Players []Player         `json:"players"`
	Bib     int              `json:"bib,omitempty"`
}

// EntrantSplit is the payload of TypeEntrantSplit: Player leaves the pair ID
// and waits for a partner as the new entrant New ("e<seq>"). Players is
// what stays in ID. ID keeps its bib, New has none.
type EntrantSplit struct {
	ID      engine.EntrantID `json:"id"`
	Players []Player         `json:"players"`
	New     engine.EntrantID `json:"new"`
	Player  PlayerID         `json:"player"`
}

// PairEntrants pairs two players waiting for a partner in the same
// category: the partner's player joins the entrant id.
func (s State) PairEntrants(id, partner engine.EntrantID) (Change, error) {
	a, ok := s.Entrant(id)
	b, okb := s.Entrant(partner)
	if !ok || !okb {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	if !a.Waiting() || !b.Waiting() || id == partner {
		return Change{}, engine.NewError(engine.CodeNotWaitingPartner)
	}
	if a.Category != b.Category {
		return Change{}, engine.NewError(engine.CodeNotSameCategory)
	}
	players := slices.Concat(a.Players, b.Players)
	p := EntrantsPaired{ID: id, Partner: partner, Players: players}
	if a.Bib == 0 && b.Bib == 0 && anyCheckedIn(players) {
		p.Bib = s.lastBib + 1
	}
	return Change{TypeEntrantsPaired, p}, nil
}

// AddPartner adds a partner, not registered before, to a player waiting
// for one.
func (s State) AddPartner(id engine.EntrantID, in PlayerInput) (Change, error) {
	e, ok := s.Entrant(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	if !e.Waiting() {
		return Change{}, engine.NewError(engine.CodeNotWaitingPartner)
	}
	partner, err := normalizePlayer(in, 2)
	if err != nil {
		return Change{}, err
	}
	partner.ID = PlayerID(fmt.Sprintf("p%d.1", s.nextSeq()))
	p := PartnerAdded{ID: id, Players: slices.Concat(e.Players, []Player{partner})}
	if e.Bib == 0 && anyCheckedIn(e.Players) {
		p.Bib = s.lastBib + 1
	}
	return Change{TypePartnerAdded, p}, nil
}

// SplitEntrant takes a player out of a pair: both wait for a partner again.
// Refused once the pair has matches (declare a forfeit); a pair placed in a
// draw not confirmed yet leaves it.
func (s State) SplitEntrant(id engine.EntrantID, player PlayerID) (Change, error) {
	e, ok := s.Entrant(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	if !e.Category.IsDoubles() || len(e.Players) != 2 {
		return Change{}, engine.NewError(engine.CodeNotAPair)
	}
	j := slices.IndexFunc(e.Players, func(p Player) bool { return p.ID == player })
	if j < 0 {
		return Change{}, engine.NewError(engine.CodePlayerNotFound)
	}
	if s.InMatches(id) {
		return Change{}, engine.NewError(engine.CodeEntrantInPools)
	}
	return Change{TypeEntrantSplit, EntrantSplit{
		ID:      id,
		Players: []Player{e.Players[1-j]},
		New:     engine.EntrantID(fmt.Sprintf("e%d", s.nextSeq())),
		Player:  player,
	}}, nil
}

// Waiting returns the players of a category waiting for a partner.
func (s State) Waiting(c engine.Category) []Entrant {
	var out []Entrant
	for _, e := range s.Entrants {
		if e.Category == c && e.Waiting() {
			out = append(out, e)
		}
	}
	return out
}

func anyCheckedIn(players []Player) bool {
	return slices.ContainsFunc(players, func(p Player) bool { return p.CheckedIn })
}

// assignBib gives an entrant without a bib the number recorded in its event,
// or the next free one when that number was taken since (undo of an undo).
func (s *State) assignBib(i, bib int) {
	if bib == 0 || s.Entrants[i].Bib != 0 {
		return
	}
	bib = max(bib, s.lastBib+1)
	s.Entrants[i].Bib = bib
	s.lastBib = bib
}

func (s *State) applyEntrantsPaired(p EntrantsPaired) {
	i, j := s.entrantIndex(p.ID), s.entrantIndex(p.Partner)
	if i < 0 || j < 0 {
		return
	}
	a, b := &s.Entrants[i], s.Entrants[j]
	a.Players = slices.Concat(a.Players, b.Players)
	if a.Bib == 0 {
		a.Bib = b.Bib // the partner's number, when it had one from a pair split since
	}
	s.assignBib(i, p.Bib)
	s.Entrants = slices.Delete(s.Entrants, j, j+1)
	s.removeFromEntry(p.Partner)
}

func (s *State) applyPartnerAdded(p PartnerAdded) {
	i := s.entrantIndex(p.ID)
	if i < 0 || len(p.Players) == 0 {
		return
	}
	s.Entrants[i].Players = slices.Concat(s.Entrants[i].Players, p.Players[len(p.Players)-1:])
	s.assignBib(i, p.Bib)
}

func (s *State) applyEntrantSplit(p EntrantSplit) {
	i := s.entrantIndex(p.ID)
	if i < 0 {
		return
	}
	e := &s.Entrants[i]
	k := slices.IndexFunc(e.Players, func(pl Player) bool { return pl.ID == p.Player })
	if k < 0 {
		return
	}
	leaving := e.Players[k]
	e.Players = slices.Delete(slices.Clone(e.Players), k, k+1)
	e.Seed = 0 // a waiting player is not drawn, so not seeded
	s.removeFromEntry(p.ID)
	s.Entrants = slices.Insert(s.Entrants, i+1, Entrant{ID: p.New, Category: e.Category, Players: []Player{leaving}})
}
