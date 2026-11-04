package tournament

import (
	"encoding/json"
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
	maxPlayerNameLength = 100
	maxClubLength       = 80
)

// PlayerID identifies a player within a tournament.
type PlayerID string

// Player is a person entered in the tournament. Categories are disjoint, so a
// player belongs to exactly one entrant.
type Player struct {
	ID PlayerID `json:"id"`
	// Name is the full name, as typed: one field, first and last name
	// together.
	Name      string `json:"name"`
	Club      string `json:"club"`
	CheckedIn bool   `json:"-"`
	// CheckedInAt is when the player was checked in, zero when absent.
	CheckedInAt time.Time `json:"-"`
}

// UnmarshalJSON reads a player, joining the first and last names of events
// written before players had a single name.
func (p *Player) UnmarshalJSON(data []byte) error {
	type player Player
	var v struct {
		player
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = Player(v.player)
	if p.Name == "" {
		p.Name = strings.TrimSpace(v.FirstName + " " + v.LastName)
	}
	return nil
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

// Waiting reports whether the entrant is a player registered alone in a
// doubles category, waiting for a partner. It cannot be drawn.
func (e Entrant) Waiting() bool {
	return e.Category.IsDoubles() && len(e.Players) == 1
}

// Complete reports whether every player of the entrant is checked in. A
// player waiting for a partner is never complete.
func (e Entrant) Complete() bool {
	if e.Waiting() {
		return false
	}
	for _, p := range e.Players {
		if !p.CheckedIn {
			return false
		}
	}
	return len(e.Players) > 0
}

// PlayerInput is a player as typed at the desk.
type PlayerInput struct {
	Name string `json:"name"`
	Club string `json:"club"`
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

// RegisterEntrant enters a player (singles) or a pair (doubles). In
// doubles, a single player is entered waiting for a partner.
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
	if len(inputs) != len(entrant.Players) {
		// Partners are added, paired or split by their own commands.
		return Change{}, engine.NewError(engine.CodeWrongPlayerCount)
	}
	players, err := normalizePlayers(entrant.Category, inputs)
	if err != nil {
		return Change{}, err
	}
	changed := false
	for i := range players {
		old := entrant.Players[i]
		players[i].ID = old.ID
		changed = changed || players[i].Name != old.Name || players[i].Club != old.Club
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
	if s.InMatches(id) {
		// Its matches exist: declare a forfeit instead.
		return Change{}, engine.NewError(engine.CodeEntrantInPools)
	}
	return Change{TypeEntrantRemoved, EntrantRemoved{ID: id}}, nil
}

// InMatches reports whether an entrant has matches (a confirmed draw put it
// in a pool or a bracket): it can no longer be removed or split, only
// declared forfeit.
func (s State) InMatches(id engine.EntrantID) bool {
	return slices.ContainsFunc(s.matches, func(m Match) bool { return m.Has(id) })
}

// normalizePlayers checks the players of an entrant: one in singles, two in
// doubles, or one waiting for a partner.
func normalizePlayers(c engine.Category, inputs []PlayerInput) ([]Player, error) {
	if len(inputs) != 1 && (!c.IsDoubles() || len(inputs) != 2) {
		return nil, engine.NewError(engine.CodeWrongPlayerCount)
	}
	players := make([]Player, len(inputs))
	for i, in := range inputs {
		p, err := normalizePlayer(in, i+1)
		if err != nil {
			return nil, err
		}
		players[i] = p
	}
	return players, nil
}

// normalizePlayer checks a player's name and club; n numbers the player in
// the error.
func normalizePlayer(in PlayerInput, n int) (Player, error) {
	p := Player{Name: collapseSpaces(in.Name), Club: collapseSpaces(in.Club)}
	if p.Name == "" || utf8.RuneCountInString(p.Name) > maxPlayerNameLength {
		return Player{}, &engine.Error{Code: engine.CodeInvalidPlayerName, Set: n}
	}
	if utf8.RuneCountInString(p.Club) > maxClubLength {
		return Player{}, &engine.Error{Code: engine.CodeInvalidClub, Set: n}
	}
	return p, nil
}

// DuplicatePlayers returns the IDs of players whose full name appears more
// than once in the tournament. It is a warning, never a refusal: homonyms exist.
func (s State) DuplicatePlayers() map[PlayerID]bool {
	byName := map[string][]PlayerID{}
	for _, e := range s.Entrants {
		for _, p := range e.Players {
			key := strings.ToLower(p.Name)
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
	s.removeFromEntry(p.ID)
}

// removeFromEntry takes an entrant out of the draw of its entry stage: out
// of its pool, or a bye in its bracket slot.
func (s *State) removeFromEntry(id engine.EntrantID) {
	for _, stages := range s.formats {
		if len(stages) == 0 {
			continue
		}
		e := stages[0]
		for i := range e.Groups {
			if e.Kind == StageBracket {
				for j, g := range e.Groups[i] {
					if g == id {
						e.Groups[i][j] = "" // a bye
					}
				}
			} else {
				e.Groups[i] = slices.DeleteFunc(e.Groups[i], func(g engine.EntrantID) bool { return g == id })
			}
		}
	}
}
