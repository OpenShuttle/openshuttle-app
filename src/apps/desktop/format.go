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

// PreviewBase is the entrant count a format preview starts from. The Format
// screen offers each one; it is a display choice and never changes the draw.
type PreviewBase string

const (
	// PreviewPaired counts the entrants not waiting for a partner (doubles only).
	PreviewPaired PreviewBase = "paired"
	// PreviewRegistered counts every entrant of the category.
	PreviewRegistered PreviewBase = "registered"
	// PreviewPresent counts the entrants with every player checked in: the
	// ones the draw takes.
	PreviewPresent PreviewBase = "present"
)

// FormatPreviewView is the format of a category previewed for one base.
type FormatPreviewView struct {
	Base     PreviewBase `json:"base"`
	Entrants int         `json:"entrants"`
	Stages   []StageView `json:"stages"`
	// Ranked is how many entrants the full standings place; Ties is set when
	// some of them share a rank.
	Ranked int  `json:"ranked"`
	Ties   bool `json:"ties"`
}

// FormatCategory is one category of the format screen.
type FormatCategory struct {
	CategoryRef
	// TimedAllowed is false with two categories: timed rounds need all courts.
	TimedAllowed bool `json:"timedAllowed"`
	// Previews holds the format previewed for each base, in the order
	// offered: paired (doubles only), registered, present. Stages already
	// filled show their actual groups whatever the base.
	Previews []FormatPreviewView `json:"previews"`
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
		}
		for _, b := range previewCounts(st, ref.Category) {
			p := FormatPreviewView{Base: b.base, Entrants: b.n}
			p.Stages, p.Ranked, p.Ties = formatPreview(st, ref.Category, b.n)
			fc.Previews = append(fc.Previews, p)
		}
		v.Categories = append(v.Categories, fc)
	}
	return v
}

type previewCount struct {
	base PreviewBase
	n    int
}

// previewCounts counts the entrants of a category for each preview base.
// Singles have no partner to wait for: the paired base is left out.
func previewCounts(st tournament.State, c engine.Category) []previewCount {
	registered, paired := 0, 0
	for _, e := range st.Entrants {
		if e.Category != c {
			continue
		}
		registered++
		if !e.Waiting() {
			paired++
		}
	}
	var out []previewCount
	if c.IsDoubles() {
		out = append(out, previewCount{PreviewPaired, paired})
	}
	return append(out,
		previewCount{PreviewRegistered, registered},
		previewCount{PreviewPresent, len(st.Present(c))})
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
			Filled: stage.Filled(), Confirmed: stage.Confirmed, Started: st.StageStarted(c, stage.ID),
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

// SaveFormatPreset saves the format of a category as a preset.
func (s *TournamentService) SaveFormatPreset(ctx context.Context, c engine.Category, name, description string) error {
	name, err := presetName(name, maxPresetName, true)
	if err != nil {
		return err
	}
	description, err = presetName(description, maxPresetDesc, false)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.file == nil {
		s.mu.Unlock()
		return engine.NewError(engine.CodeNoTournament)
	}
	if !s.state.HasCategory(c) {
		s.mu.Unlock()
		return engine.NewError(engine.CodeCategoryNotFound)
	}
	stages := s.state.StageConfigs(c)
	s.mu.Unlock()
	return updatePresets(ctx, s.app, s.emit, func(p *Presets) {
		p.Formats = append(p.Formats, FormatPreset{ID: newPresetID(s.now()), Name: name, Description: description, Stages: stages})
	})
}

// ApplyFormatPreset replaces the format of a category with a preset.
func (s *TournamentService) ApplyFormatPreset(ctx context.Context, c engine.Category, id string) error {
	presets, err := readPresets(ctx, s.app)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(presets.Formats, func(p FormatPreset) bool { return p.ID == id })
	if i < 0 {
		return engine.NewError(engine.CodePresetNotFound)
	}
	stages := presets.Formats[i].Stages
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ApplyFormat(c, stages) })
}
