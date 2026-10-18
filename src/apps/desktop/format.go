package main

import (
	"context"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// RankPreview is a rank of a stage and how many entrants it holds: one per
// pool for a pool position, one per bracket place (several when tied).
type RankPreview struct {
	Rank  int `json:"rank"`
	Count int `json:"count"`
}

// StageView is a stage in the format editor, with a preview computed from
// the registered entrants (or the actual ones once filled).
type StageView struct {
	tournament.StageConfig
	Phase int  `json:"phase"`
	Entry bool `json:"entry"`
	// Filled: entrants are placed (a draft draw for the entry stage).
	// Confirmed: matches exist. Started: a match was called or scored.
	Filled    bool `json:"filled"`
	Confirmed bool `json:"confirmed"`
	Started   bool `json:"started"`
	// Preview.
	Entrants int           `json:"entrants"`
	Sizes    []int         `json:"sizes"` // pools
	Byes     int           `json:"byes"`  // brackets
	Matches  int           `json:"matches"`
	Rounds   int           `json:"rounds"`
	Ranks    []RankPreview `json:"ranks"`
	// PlaceFrom..PlaceTo are the final places of the entrants kept in this
	// stage; 0 when every rank goes on.
	PlaceFrom int `json:"placeFrom"`
	PlaceTo   int `json:"placeTo"`
}

// FormatCategory is one category of the format screen.
type FormatCategory struct {
	CategoryRef
	// TimedAllowed is false with two categories: timed rounds need all courts.
	TimedAllowed bool        `json:"timedAllowed"`
	Registered   int         `json:"registered"`
	Present      int         `json:"present"`
	Stages       []StageView `json:"stages"`
	// Ranked is how many entrants the full standings place; Ties is set when
	// some of them share a rank.
	Ranked int  `json:"ranked"`
	Ties   bool `json:"ties"`
}

// FormatView is the format screen.
type FormatView struct {
	Categories []FormatCategory `json:"categories"`
}

// SetFormat changes the stages of a category (same stages, same order).
func (s *TournamentService) SetFormat(ctx context.Context, c engine.Category, stages []tournament.StageConfig) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetFormat(c, stages) })
}

// AddStage adds a stage at the end of a category's format.
func (s *TournamentService) AddStage(ctx context.Context, c engine.Category, kind tournament.StageKind) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.AddStage(c, kind) })
}

// RemoveStage removes a stage with no entrants yet.
func (s *TournamentService) RemoveStage(ctx context.Context, c engine.Category, id string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.RemoveStage(c, id) })
}

// FillStage fills a later stage from the ranks routed to it.
func (s *TournamentService) FillStage(ctx context.Context, c engine.Category, id string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.FillStage(c, id) })
}

// SwapSlots swaps two first-round slots of a bracket stage before it starts.
func (s *TournamentService) SwapSlots(ctx context.Context, c engine.Category, id string, a, b int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SwapSlots(c, id, a, b) })
}

// GetFormatView returns the format screen.
func (s *TournamentService) GetFormatView() FormatView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	v := FormatView{Categories: []FormatCategory{}}
	for _, ref := range categoryRefs(st) {
		fc := FormatCategory{
			CategoryRef:  ref,
			TimedAllowed: len(st.Categories) == 1,
			Present:      len(st.Present(ref.Category)),
		}
		for _, e := range st.Entrants {
			if e.Category == ref.Category {
				fc.Registered++
			}
		}
		fc.Stages, fc.Ranked, fc.Ties = formatPreview(st, ref.Category, fc.Registered)
		v.Categories = append(v.Categories, fc)
	}
	return v
}

// formatPreview sums up each stage: how many entrants reach it, its pools or
// byes, matches, rounds and ranks, then the final places of the entrants it
// keeps, following the routing like the full standings do.
func formatPreview(st tournament.State, c engine.Category, registered int) (stages []StageView, ranked int, ties bool) {
	expected := map[string]int{}
	byID := map[string]*StageView{}
	for i, stage := range st.Stages(c) {
		v := StageView{
			StageConfig: stage.StageConfig, Phase: st.Phase(c, stage.ID), Entry: i == 0,
			Filled: stage.Filled(), Confirmed: stage.Confirmed, Started: stageStarted(st, c, stage.ID),
			Sizes: []int{}, Ranks: []RankPreview{},
		}
		if v.Routing == nil {
			v.Routing = []string{}
		}
		if v.Tiebreaks == nil {
			v.Tiebreaks = []engine.Tiebreak{}
		}
		switch {
		case stage.Filled():
			for _, g := range stage.Groups {
				n := 0
				for _, id := range g {
					if id != "" {
						n++
					}
				}
				v.Entrants += n
				if stage.Kind == tournament.StagePools {
					v.Sizes = append(v.Sizes, n)
				}
			}
		case i == 0:
			v.Entrants = registered
		default:
			v.Entrants = expected[stage.ID]
		}
		if stage.Kind == tournament.StagePools {
			if !stage.Filled() {
				v.Sizes = engine.PoolSizes(v.Entrants, stage.Pools)
			}
			for _, size := range v.Sizes {
				v.Matches += engine.RoundRobinMatches(size)
				v.Rounds = max(v.Rounds, engine.RoundRobinRounds(size))
			}
			for r := 1; ; r++ {
				count := 0
				for _, size := range v.Sizes {
					if size >= r {
						count++
					}
				}
				if count == 0 {
					break
				}
				v.Ranks = append(v.Ranks, RankPreview{Rank: r, Count: count})
			}
		} else if v.Entrants > 0 {
			v.Byes = engine.NextPowerOfTwo(v.Entrants) - v.Entrants
			v.Matches = engine.BracketMatches(v.Entrants, stage.FullPlacement)
			v.Rounds = engine.BracketRounds(v.Entrants)
			v.Ranks = bracketRanks(v.Entrants, stage.FullPlacement)
		}
		for _, rp := range v.Ranks {
			if to := stage.RouteOf(rp.Rank); to != "" {
				expected[to] += rp.Count
			}
		}
		stages = append(stages, v)
	}
	for i := range stages {
		byID[stages[i].ID] = &stages[i]
	}

	// Final places, following the routing from the entry stage.
	expanded := map[string]bool{}
	var walk func(v *StageView)
	walk = func(v *StageView) {
		expanded[v.ID] = true
		for _, rp := range v.Ranks {
			if to := v.RouteOf(rp.Rank); to != "" {
				if next := byID[to]; next != nil && !expanded[to] {
					walk(next)
				}
				continue
			}
			if v.PlaceFrom == 0 {
				v.PlaceFrom = ranked + 1
			}
			ranked += rp.Count
			v.PlaceTo = ranked
			ties = ties || rp.Count > 1
		}
	}
	if len(stages) > 0 {
		walk(&stages[0])
	}
	return stages, ranked, ties
}

// bracketRanks lists the places of a bracket of n entrants: every place with
// full placement; otherwise 1, 2, then the losers of each round tied (3–4,
// 5–8…).
func bracketRanks(n int, full bool) []RankPreview {
	var out []RankPreview
	if full {
		for r := 1; r <= n; r++ {
			out = append(out, RankPreview{Rank: r, Count: 1})
		}
		return out
	}
	out = append(out, RankPreview{Rank: 1, Count: 1})
	if n >= 2 {
		out = append(out, RankPreview{Rank: 2, Count: 1})
	}
	for p := 2; p < n; p *= 2 {
		out = append(out, RankPreview{Rank: p + 1, Count: min(p, n-p)})
	}
	return out
}

func stageStarted(st tournament.State, c engine.Category, id string) bool {
	return slices.ContainsFunc(st.Matches(), func(m tournament.Match) bool {
		return m.Category == c && m.Stage == id && (m.Called() || m.Score != nil)
	})
}
