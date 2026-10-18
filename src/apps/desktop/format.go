package main

import (
	"context"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// StageView is a stage in the format editor, with what it gives for the
// registered entrants (the actual ones once filled).
type StageView struct {
	tournament.StageConfig
	tournament.StagePreview
	Entry bool `json:"entry"`
	// Filled: entrants are placed (a draft draw for the entry stage).
	// Confirmed: matches exist. Started: a match was called or scored.
	Filled    bool `json:"filled"`
	Confirmed bool `json:"confirmed"`
	Started   bool `json:"started"`
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

// formatPreview previews each stage for the registered entrants, with the
// actual groups of the stages already filled.
func formatPreview(st tournament.State, c engine.Category, registered int) (stages []StageView, ranked int, ties bool) {
	actual := map[string][]int{}
	for _, stage := range st.Stages(c) {
		if !stage.Filled() {
			continue
		}
		var sizes []int
		for _, g := range stage.Groups {
			n := 0
			for _, id := range g {
				if id != "" {
					n++
				}
			}
			sizes = append(sizes, n)
		}
		actual[stage.ID] = sizes
	}
	preview := tournament.PreviewFormat(st.StageConfigs(c), registered, actual)
	for i, stage := range st.Stages(c) {
		v := StageView{
			StageConfig: stage.StageConfig, StagePreview: preview.Stages[i], Entry: i == 0,
			Filled: stage.Filled(), Confirmed: stage.Confirmed, Started: stageStarted(st, c, stage.ID),
		}
		if v.Routing == nil {
			v.Routing = []tournament.RouteRule{}
		}
		if v.Tiebreaks == nil {
			v.Tiebreaks = []engine.Tiebreak{}
		}
		if v.PoolSizes == nil {
			v.PoolSizes = []int{}
		}
		stages = append(stages, v)
	}
	return stages, preview.Ranked, preview.Ties
}

func stageStarted(st tournament.State, c engine.Category, id string) bool {
	return slices.ContainsFunc(st.Matches(), func(m tournament.Match) bool {
		return m.Category == c && m.Stage == id && (m.Called() || m.Score != nil)
	})
}
