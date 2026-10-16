package main

import (
	"context"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// BracketView is a bracket in the format editor.
type BracketView struct {
	ID string `json:"id"`
	tournament.BracketConfig
	Drawn bool `json:"drawn"`
}

// RoundView is the first round of a sub-bracket: the entrants playing for
// places From..To.
type RoundView struct {
	From    int         `json:"from"`
	To      int         `json:"to"`
	Size    int         `json:"size"`
	Matches []MatchView `json:"matches"`
}

// PlaceView is a final position of a bracket.
type PlaceView struct {
	From    int      `json:"from"`
	To      int      `json:"to"`
	Entrant SideView `json:"entrant"`
}

// BracketPhase is a bracket on the stages screen.
type BracketPhase struct {
	ID string `json:"id"`
	tournament.BracketConfig
	Drawn   bool `json:"drawn"`
	Started bool `json:"started"`
	// Routed is the number of entrants the pools send to this bracket.
	Routed int `json:"routed"`
	// CanDraw is set when every pool is complete with no tie left.
	CanDraw bool        `json:"canDraw"`
	Slots   []SideView  `json:"slots"` // first round, byes have an empty entrant
	Rounds  []RoundView `json:"rounds"`
	Places  []PlaceView `json:"places"`
}

// AddBracket adds a bracket stage.
func (s *TournamentService) AddBracket(ctx context.Context, c engine.Category, cfg tournament.BracketConfig) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.AddBracket(c, cfg) })
}

// ConfigureBracket changes a bracket.
func (s *TournamentService) ConfigureBracket(ctx context.Context, c engine.Category, id string, cfg tournament.BracketConfig) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ConfigureBracket(c, id, cfg) })
}

// RemoveBracket removes a bracket.
func (s *TournamentService) RemoveBracket(ctx context.Context, c engine.Category, id string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.RemoveBracket(c, id) })
}

// SetRouting sets the bracket of each pool position ("" for eliminated).
func (s *TournamentService) SetRouting(ctx context.Context, c engine.Category, routing []string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetRouting(c, routing) })
}

// DrawBracket fills a bracket from the final pool standings.
func (s *TournamentService) DrawBracket(ctx context.Context, c engine.Category, id string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.DrawBracket(c, id) })
}

// SwapBracketSlots swaps two first-round slots before the bracket starts.
func (s *TournamentService) SwapBracketSlots(ctx context.Context, c engine.Category, id string, a, b int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SwapBracketSlots(c, id, a, b) })
}

// bracketPhases builds the brackets of a category for the stages screen.
func (s *TournamentService) bracketPhases(st tournament.State, c engine.Category) []BracketPhase {
	mv := s.matchViewer(st)
	side := sideViewer(st)
	canDraw := poolsSettled(st, c)

	out := []BracketPhase{}
	for _, b := range st.Brackets(c) {
		bp := BracketPhase{
			ID: b.ID, BracketConfig: b.BracketConfig, Drawn: b.Drawn(),
			Routed: len(st.Routed(c, b.ID)), CanDraw: canDraw,
			Slots: []SideView{}, Rounds: []RoundView{}, Places: []PlaceView{},
		}
		for _, id := range b.Slots {
			if id == "" {
				bp.Slots = append(bp.Slots, SideView{Names: []string{}, Clubs: []string{}})
			} else {
				bp.Slots = append(bp.Slots, side(id))
			}
		}
		var current *RoundView
		for _, m := range st.Matches() {
			if m.Category != c || m.Bracket != b.ID {
				continue
			}
			bp.Started = bp.Started || m.Called() || m.Score != nil
			if current == nil || current.From != m.Start || current.Size != m.Size {
				bp.Rounds = append(bp.Rounds, RoundView{From: m.Start, To: m.Start + m.Size - 1, Size: m.Size, Matches: []MatchView{}})
				current = &bp.Rounds[len(bp.Rounds)-1]
			}
			current.Matches = append(current.Matches, mv(m))
		}
		for _, p := range st.Places(c, b.ID) {
			bp.Places = append(bp.Places, PlaceView{From: p.From, To: p.To, Entrant: side(p.Entrant)})
		}
		out = append(out, bp)
	}
	return out
}

// poolsSettled reports whether every pool of a category is complete with no
// tie left to draw.
func poolsSettled(st tournament.State, c engine.Category) bool {
	d, ok := st.Draw(c)
	if !ok || !d.Confirmed {
		return false
	}
	complete := 0
	for _, ref := range st.CompletePools() {
		if ref.Category == c {
			complete++
		}
	}
	if complete < len(d.Pools) {
		return false
	}
	for p := range d.Pools {
		if slices.ContainsFunc(st.PoolStandings(c, p).Rows, func(r engine.Standing) bool { return r.DrawNeeded }) {
			return false
		}
	}
	return true
}
