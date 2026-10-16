package tournament

import (
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeBracketAdded        events.Type = "BracketAdded"
	TypeBracketConfigured   events.Type = "BracketConfigured"
	TypeBracketRemoved      events.Type = "BracketRemoved"
	TypeRoutingSet          events.Type = "RoutingSet"
	TypeBracketDrawn        events.Type = "BracketDrawn"
	TypeBracketSlotsSwapped events.Type = "BracketSlotsSwapped"
)

// Limits of the bracket stages of a category.
const (
	MaxBrackets          = 8
	MaxBracketNameLength = 60
	MaxRouting           = 32 // routing entries: one per pool rank
)

// BracketConfig is what the format editor sets on a bracket.
type BracketConfig struct {
	Name string              `json:"name"`
	Win  engine.WinCondition `json:"win"`
	// FullPlacement plays every place (losers play for the lower places).
	// Otherwise entrants losing in the same round are tied.
	FullPlacement bool `json:"fullPlacement"`
}

// Bracket is a knockout stage fed by the pools.
type Bracket struct {
	ID string // "T1", "T2"… never reused within a category
	BracketConfig
	// Slots are the first-round slots once drawn; "" is a bye. Nil until drawn.
	Slots []engine.EntrantID
}

// Drawn reports whether the bracket has been filled from the pools.
func (b Bracket) Drawn() bool { return b.Slots != nil }

// BracketAdded is the payload of TypeBracketAdded.
type BracketAdded struct {
	Category engine.Category `json:"category"`
	ID       string          `json:"id"`
	BracketConfig
}

// BracketConfigured is the payload of TypeBracketConfigured.
type BracketConfigured struct {
	Category engine.Category `json:"category"`
	ID       string          `json:"id"`
	BracketConfig
}

// BracketRemoved is the payload of TypeBracketRemoved.
type BracketRemoved struct {
	Category engine.Category `json:"category"`
	ID       string          `json:"id"`
}

// RoutingSet is the payload of TypeRoutingSet: Routing[i] is the bracket of
// the entrants finishing at position i+1 of their pool, "" for eliminated.
type RoutingSet struct {
	Category engine.Category `json:"category"`
	Routing  []string        `json:"routing"`
}

// BracketDrawn is the payload of TypeBracketDrawn: the first-round slots.
type BracketDrawn struct {
	Category engine.Category    `json:"category"`
	ID       string             `json:"id"`
	Slots    []engine.EntrantID `json:"slots"`
}

// BracketSlotsSwapped is the payload of TypeBracketSlotsSwapped.
type BracketSlotsSwapped struct {
	Category engine.Category `json:"category"`
	ID       string          `json:"id"`
	A        int             `json:"a"`
	B        int             `json:"b"`
}

// Brackets returns the brackets of a category, in the order they were added.
func (s State) Brackets(c engine.Category) []Bracket {
	out := make([]Bracket, 0, len(s.brackets[c]))
	for _, b := range s.brackets[c] {
		out = append(out, *b)
	}
	return out
}

// Bracket returns a bracket by ID.
func (s State) Bracket(c engine.Category, id string) (Bracket, bool) {
	if b := s.bracket(c, id); b != nil {
		return *b, true
	}
	return Bracket{}, false
}

func (s State) bracket(c engine.Category, id string) *Bracket {
	for _, b := range s.brackets[c] {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// Routing returns where each pool position goes, "" for eliminated.
func (s State) Routing(c engine.Category) []string {
	return slices.Clone(s.routing[c])
}

// AddBracket adds a bracket stage to a category.
func (s State) AddBracket(c engine.Category, cfg BracketConfig) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	if len(s.brackets[c]) >= MaxBrackets {
		return Change{}, engine.NewError(engine.CodeTooManyBrackets)
	}
	cfg, err := s.validBracketConfig(cfg)
	if err != nil {
		return Change{}, err
	}
	id := fmt.Sprintf("T%d", s.bracketNumbers[c]+1)
	return Change{TypeBracketAdded, BracketAdded{Category: c, ID: id, BracketConfig: cfg}}, nil
}

// ConfigureBracket changes a bracket. Switching full placement regenerates
// its matches, so it is refused once a match of the bracket has started.
func (s State) ConfigureBracket(c engine.Category, id string, cfg BracketConfig) (Change, error) {
	b := s.bracket(c, id)
	if b == nil {
		return Change{}, engine.NewError(engine.CodeBracketNotFound)
	}
	cfg, err := s.validBracketConfig(cfg)
	if err != nil {
		return Change{}, err
	}
	if cfg == b.BracketConfig {
		return Change{}, nil
	}
	if cfg.FullPlacement != b.FullPlacement && s.bracketStarted(c, id) {
		return Change{}, engine.NewError(engine.CodeBracketStarted)
	}
	return Change{TypeBracketConfigured, BracketConfigured{Category: c, ID: id, BracketConfig: cfg}}, nil
}

// RemoveBracket removes a bracket that has no match started.
func (s State) RemoveBracket(c engine.Category, id string) (Change, error) {
	if s.bracket(c, id) == nil {
		return Change{}, engine.NewError(engine.CodeBracketNotFound)
	}
	if s.bracketStarted(c, id) {
		return Change{}, engine.NewError(engine.CodeBracketStarted)
	}
	return Change{TypeBracketRemoved, BracketRemoved{Category: c, ID: id}}, nil
}

// SetRouting sets where each pool position goes. Trailing eliminations are
// trimmed.
func (s State) SetRouting(c engine.Category, routing []string) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	if len(routing) > MaxRouting {
		return Change{}, engine.NewError(engine.CodeInvalidRouting)
	}
	for _, id := range routing {
		if id != "" && s.bracket(c, id) == nil {
			return Change{}, engine.NewError(engine.CodeInvalidRouting)
		}
	}
	for len(routing) > 0 && routing[len(routing)-1] == "" {
		routing = routing[:len(routing)-1]
	}
	if slices.Equal(routing, s.routing[c]) {
		return Change{}, nil
	}
	return Change{TypeRoutingSet, RoutingSet{Category: c, Routing: slices.Clone(routing)}}, nil
}

// Routed returns the entrants the pools send to a bracket, with their pool
// position. Forfeited entrants are left out.
func (s State) Routed(c engine.Category, id string) []engine.BracketEntrant {
	d, ok := s.Draw(c)
	if !ok {
		return nil
	}
	routing := s.routing[c]
	var out []engine.BracketEntrant
	for p := range d.Pools {
		for i, row := range s.PoolStandings(c, p).Rows {
			if i < len(routing) && routing[i] == id && !row.Withdrawn {
				out = append(out, engine.BracketEntrant{ID: row.Entrant, Rank: i + 1, Pool: p})
			}
		}
	}
	return out
}

// DrawBracket fills a bracket from the final pool standings. Every pool must
// be complete and ties broken: the seeding depends on it.
func (s State) DrawBracket(c engine.Category, id string) (Change, error) {
	if s.bracket(c, id) == nil {
		return Change{}, engine.NewError(engine.CodeBracketNotFound)
	}
	d, ok := s.Draw(c)
	if !ok || !d.Confirmed {
		return Change{}, engine.NewError(engine.CodePoolsNotComplete)
	}
	complete := 0
	for _, ref := range s.CompletePools() {
		if ref.Category == c {
			complete++
		}
	}
	if complete < len(d.Pools) {
		return Change{}, engine.NewError(engine.CodePoolsNotComplete)
	}
	for p := range d.Pools {
		for _, row := range s.PoolStandings(c, p).Rows {
			if row.DrawNeeded {
				return Change{}, engine.NewError(engine.CodeTieUnresolved)
			}
		}
	}
	if s.bracketStarted(c, id) {
		return Change{}, engine.NewError(engine.CodeBracketStarted)
	}
	routed := s.Routed(c, id)
	if len(routed) < 2 {
		return Change{}, engine.NewError(engine.CodeNotEnoughEntrants)
	}
	return Change{TypeBracketDrawn, BracketDrawn{Category: c, ID: id, Slots: engine.SeedBracket(routed)}}, nil
}

// SwapBracketSlots swaps two first-round slots, to correct the seeding
// before the bracket starts.
func (s State) SwapBracketSlots(c engine.Category, id string, a, b int) (Change, error) {
	br := s.bracket(c, id)
	switch {
	case br == nil:
		return Change{}, engine.NewError(engine.CodeBracketNotFound)
	case !br.Drawn():
		return Change{}, engine.NewError(engine.CodeBracketNotDrawn)
	case a < 0 || b < 0 || a >= len(br.Slots) || b >= len(br.Slots):
		return Change{}, engine.NewError(engine.CodeInvalidSlot)
	case s.bracketStarted(c, id):
		return Change{}, engine.NewError(engine.CodeBracketStarted)
	case a == b:
		return Change{}, nil
	}
	return Change{TypeBracketSlotsSwapped, BracketSlotsSwapped{Category: c, ID: id, A: a, B: b}}, nil
}

func (s State) validBracketConfig(cfg BracketConfig) (BracketConfig, error) {
	cfg.Name = collapseSpaces(cfg.Name)
	if cfg.Name == "" || utf8.RuneCountInString(cfg.Name) > MaxBracketNameLength {
		return cfg, engine.NewError(engine.CodeInvalidName)
	}
	if err := cfg.Win.Validate(); err != nil {
		return cfg, err
	}
	if cfg.Win.Kind == engine.WinByTime {
		if len(s.Categories) > 1 {
			return cfg, engine.NewError(engine.CodeTimedNeedsOneCategory)
		}
		cfg.Win = engine.WinCondition{Kind: engine.WinByTime, Minutes: cfg.Win.Minutes}
	} else {
		cfg.Win.Minutes = 0
	}
	return cfg, nil
}

// bracketStarted reports whether a match of the bracket was called or
// decided on court (byes do not count).
func (s State) bracketStarted(c engine.Category, id string) bool {
	return slices.ContainsFunc(s.matches, func(m Match) bool {
		return m.Category == c && m.Bracket == id && (m.Called() || m.Score != nil)
	})
}

func (s *State) applyBracketAdded(p BracketAdded) {
	if s.brackets == nil {
		s.brackets = map[engine.Category][]*Bracket{}
		s.bracketNumbers = map[engine.Category]int{}
	}
	s.brackets[p.Category] = append(s.brackets[p.Category], &Bracket{ID: p.ID, BracketConfig: p.BracketConfig})
	var n int
	if _, err := fmt.Sscanf(p.ID, "T%d", &n); err == nil {
		s.bracketNumbers[p.Category] = max(s.bracketNumbers[p.Category], n)
	}
}

func (s *State) applyBracketConfigured(p BracketConfigured) {
	b := s.bracket(p.Category, p.ID)
	if b == nil {
		return
	}
	regenerate := b.Drawn() && b.FullPlacement != p.FullPlacement
	b.BracketConfig = p.BracketConfig
	if regenerate {
		s.generateBracketMatches(p.Category, b)
	}
}

func (s *State) applyBracketRemoved(p BracketRemoved) {
	s.brackets[p.Category] = slices.DeleteFunc(s.brackets[p.Category], func(b *Bracket) bool { return b.ID == p.ID })
	s.dropBracketMatches(p.Category, p.ID)
	for i, id := range s.routing[p.Category] {
		if id == p.ID {
			s.routing[p.Category][i] = ""
		}
	}
}

func (s *State) applyRoutingSet(p RoutingSet) {
	if s.routing == nil {
		s.routing = map[engine.Category][]string{}
	}
	s.routing[p.Category] = slices.Clone(p.Routing)
}

func (s *State) applyBracketDrawn(p BracketDrawn) {
	b := s.bracket(p.Category, p.ID)
	if b == nil {
		return
	}
	b.Slots = slices.Clone(p.Slots)
	if b.Slots == nil {
		b.Slots = []engine.EntrantID{}
	}
	s.generateBracketMatches(p.Category, b)
}

func (s *State) applyBracketSlotsSwapped(p BracketSlotsSwapped) {
	if b := s.bracket(p.Category, p.ID); b != nil && p.A < len(b.Slots) && p.B < len(b.Slots) {
		b.Slots[p.A], b.Slots[p.B] = b.Slots[p.B], b.Slots[p.A]
	}
}
