package tournament

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeEntrantRegistered events.Type = "EntrantRegistered"
	TypeEntrantUpdated    events.Type = "EntrantUpdated"
	TypeEntrantRemoved    events.Type = "EntrantRemoved"
)

const (
	maxPlayerNameLength = 50
	maxClubLength       = 80
)

// PlayerID identifies a player within a tournament.
type PlayerID string

// Player is a person entered in the tournament. Categories are disjoint, so a
// player belongs to exactly one entrant.
type Player struct {
	ID        PlayerID `json:"id"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Club      string   `json:"club"`
	CheckedIn bool     `json:"-"`
	// CheckedInAt is when the player was checked in, zero when absent.
	CheckedInAt time.Time `json:"-"`
}

// Entrant is a player in singles, a pair in doubles.
type Entrant struct {
	ID       engine.EntrantID
	Category engine.Category
	Players  []Player
	// Bib is 0 until the first player of the entrant checks in.
	Bib int
	// Seed is 1 for the top seed, 0 when not seeded.
	Seed int
}

// Complete reports whether every player of the entrant is checked in.
func (e Entrant) Complete() bool {
	for _, p := range e.Players {
		if !p.CheckedIn {
			return false
		}
	}
	return len(e.Players) > 0
}

// PlayerInput is a player as typed at the desk.
type PlayerInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Club      string `json:"club"`
}

// EntrantRegistered is the payload of TypeEntrantRegistered.
type EntrantRegistered struct {
	ID       engine.EntrantID `json:"id"`
	Category engine.Category  `json:"category"`
	Players  []Player         `json:"players"`
}

// EntrantUpdated is the payload of TypeEntrantUpdated: the new names and
// clubs, players in the same order and with the same IDs.
type EntrantUpdated struct {
	ID      engine.EntrantID `json:"id"`
	Players []Player         `json:"players"`
}

// EntrantRemoved is the payload of TypeEntrantRemoved.
type EntrantRemoved struct {
	ID engine.EntrantID `json:"id"`
}

// Entrant returns the entrant with the given ID.
func (s State) Entrant(id engine.EntrantID) (Entrant, bool) {
	i := s.entrantIndex(id)
	if i < 0 {
		return Entrant{}, false
	}
	return s.Entrants[i], true
}

func (s State) entrantIndex(id engine.EntrantID) int {
	return slices.IndexFunc(s.Entrants, func(e Entrant) bool { return e.ID == id })
}

// playerIndex returns the entrant and player positions of a player.
func (s State) playerIndex(id PlayerID) (int, int) {
	for i, e := range s.Entrants {
		for j, p := range e.Players {
			if p.ID == id {
				return i, j
			}
		}
	}
	return -1, -1
}

// RegisterEntrant enters a player (singles) or a pair (doubles).
func (s State) RegisterEntrant(c engine.Category, inputs []PlayerInput) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	players, err := normalizePlayers(c, inputs)
	if err != nil {
		return Change{}, err
	}
	seq := s.nextSeq()
	for i := range players {
		players[i].ID = PlayerID(fmt.Sprintf("p%d.%d", seq, i+1))
	}
	return Change{TypeEntrantRegistered, EntrantRegistered{
		ID:       engine.EntrantID(fmt.Sprintf("e%d", seq)),
		Category: c,
		Players:  players,
	}}, nil
}

// UpdateEntrant changes the names and clubs of an entrant's players.
func (s State) UpdateEntrant(id engine.EntrantID, inputs []PlayerInput) (Change, error) {
	entrant, ok := s.Entrant(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	players, err := normalizePlayers(entrant.Category, inputs)
	if err != nil {
		return Change{}, err
	}
	changed := false
	for i := range players {
		old := entrant.Players[i]
		players[i].ID = old.ID
		changed = changed || players[i].FirstName != old.FirstName ||
			players[i].LastName != old.LastName || players[i].Club != old.Club
	}
	if !changed {
		return Change{}, nil
	}
	return Change{TypeEntrantUpdated, EntrantUpdated{ID: id, Players: players}}, nil
}

// RemoveEntrant withdraws an entrant before the draw (absent, cancelled).
func (s State) RemoveEntrant(id engine.EntrantID) (Change, error) {
	if _, ok := s.Entrant(id); !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	if slices.ContainsFunc(s.matches, func(m Match) bool { return m.Has(id) }) {
		// Its matches exist: declare a forfeit instead.
		return Change{}, engine.NewError(engine.CodeEntrantInPools)
	}
	return Change{TypeEntrantRemoved, EntrantRemoved{ID: id}}, nil
}

func normalizePlayers(c engine.Category, inputs []PlayerInput) ([]Player, error) {
	want := 1
	if c.IsDoubles() {
		want = 2
	}
	if len(inputs) != want {
		return nil, engine.NewError(engine.CodeWrongPlayerCount)
	}
	players := make([]Player, len(inputs))
	for i, in := range inputs {
		p := Player{
			FirstName: collapseSpaces(in.FirstName),
			LastName:  collapseSpaces(in.LastName),
			Club:      collapseSpaces(in.Club),
		}
		if p.FirstName == "" || p.LastName == "" ||
			utf8.RuneCountInString(p.FirstName) > maxPlayerNameLength ||
			utf8.RuneCountInString(p.LastName) > maxPlayerNameLength {
			return nil, &engine.Error{Code: engine.CodeInvalidPlayerName, Set: i + 1}
		}
		if utf8.RuneCountInString(p.Club) > maxClubLength {
			return nil, &engine.Error{Code: engine.CodeInvalidClub, Set: i + 1}
		}
		players[i] = p
	}
	return players, nil
}

// DuplicatePlayers returns the IDs of players whose full name appears more
// than once in the tournament. It is a warning, never a refusal: homonyms exist.
func (s State) DuplicatePlayers() map[PlayerID]bool {
	byName := map[string][]PlayerID{}
	for _, e := range s.Entrants {
		for _, p := range e.Players {
			key := strings.ToLower(p.FirstName + " " + p.LastName)
			byName[key] = append(byName[key], p.ID)
		}
	}
	dup := map[PlayerID]bool{}
	for _, ids := range byName {
		if len(ids) > 1 {
			for _, id := range ids {
				dup[id] = true
			}
		}
	}
	return dup
}

func (s *State) applyEntrantRegistered(p EntrantRegistered) {
	s.Entrants = append(s.Entrants, Entrant{ID: p.ID, Category: p.Category, Players: slices.Clone(p.Players)})
}

func (s *State) applyEntrantUpdated(p EntrantUpdated) {
	i := s.entrantIndex(p.ID)
	if i < 0 {
		return
	}
	for j, updated := range p.Players {
		if j < len(s.Entrants[i].Players) {
			old := s.Entrants[i].Players[j]
			s.Entrants[i].Players[j] = updated
			s.Entrants[i].Players[j].CheckedIn = old.CheckedIn
			s.Entrants[i].Players[j].CheckedInAt = old.CheckedInAt
		}
	}
}

func (s *State) applyEntrantRemoved(p EntrantRemoved) {
	s.Entrants = slices.DeleteFunc(s.Entrants, func(e Entrant) bool { return e.ID == p.ID })
	for _, d := range s.draws {
		d.remove(p.ID)
	}
}
