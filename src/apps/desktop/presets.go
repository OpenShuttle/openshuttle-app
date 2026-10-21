package main

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

const (
	settingPresets      = "presets"
	EventPresetsChanged = "presets.changed"
	maxPresetName       = 60
	maxPresetDesc       = 200
)

// WinPreset is a saved win condition. Built-in presets have an ID starting
// with "b-" and no name: the frontend describes the condition.
type WinPreset struct {
	ID   string              `json:"id"`
	Name string              `json:"name"`
	Win  engine.WinCondition `json:"win"`
}

// TiebreakPreset is a saved tiebreak order. Key names a built-in preset,
// translated by the frontend.
type TiebreakPreset struct {
	ID        string            `json:"id"`
	Key       string            `json:"key"`
	Name      string            `json:"name"`
	Tiebreaks []engine.Tiebreak `json:"tiebreaks"`
}

// FormatPreset is a saved format: its stages, routing included. Key names a
// built-in preset, translated by the frontend.
type FormatPreset struct {
	ID          string                   `json:"id"`
	Key         string                   `json:"key"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Stages      []tournament.StageConfig `json:"stages"`
}

// Presets lists every preset, built-in ones first. Disabled presets (any
// kind, built-in ones included) stay listed but are not offered.
type Presets struct {
	Win       []WinPreset      `json:"win"`
	Tiebreaks []TiebreakPreset `json:"tiebreaks"`
	Formats   []FormatPreset   `json:"formats"`
	Disabled  []string         `json:"disabled"`
	// Clubs offered when entering a player, sorted by name.
	Clubs []string `json:"clubs"`
}

// PresetService manages the presets of this PC.
type PresetService struct {
	db   *store.AppDB
	emit func(name string, data any)
	now  func() time.Time
}

func NewPresetService(db *store.AppDB, emit func(string, any), now func() time.Time) *PresetService {
	return &PresetService{db: db, emit: emit, now: now}
}

// GetPresets lists every preset.
func (s *PresetService) GetPresets(ctx context.Context) (Presets, error) {
	return readPresets(ctx, s.db)
}

// SaveWinPreset creates a win condition preset (id "") or changes a saved
// one. Built-in presets are read-only.
func (s *PresetService) SaveWinPreset(ctx context.Context, id, name string, win engine.WinCondition) error {
	name, err := presetName(name, maxPresetName, true)
	if err != nil {
		return err
	}
	if err := win.Validate(); err != nil {
		return err
	}
	return s.save(ctx, id, func(p *Presets, id string, isNew bool) bool {
		return upsert(&p.Win, WinPreset{ID: id, Name: name, Win: win}, func(x WinPreset) string { return x.ID }, isNew)
	})
}

// SaveTiebreakPreset creates a tiebreak order preset (id "") or changes a
// saved one.
func (s *PresetService) SaveTiebreakPreset(ctx context.Context, id, name string, tiebreaks []engine.Tiebreak) error {
	name, err := presetName(name, maxPresetName, true)
	if err != nil {
		return err
	}
	if !tournament.ValidTiebreaks(tiebreaks) {
		return engine.NewError(engine.CodeInvalidFormat)
	}
	tiebreaks = slices.Clone(tiebreaks)
	return s.save(ctx, id, func(p *Presets, id string, isNew bool) bool {
		return upsert(&p.Tiebreaks, TiebreakPreset{ID: id, Name: name, Tiebreaks: tiebreaks}, func(x TiebreakPreset) string { return x.ID }, isNew)
	})
}

// SaveFormatPreset creates a format preset (id "") or changes a saved one.
// The format is checked on its own, as for a tournament of one category.
func (s *PresetService) SaveFormatPreset(ctx context.Context, id, name, description string, stages []tournament.StageConfig) error {
	name, err := presetName(name, maxPresetName, true)
	if err != nil {
		return err
	}
	if description, err = presetName(description, maxPresetDesc, false); err != nil {
		return err
	}
	if stages, err = tournament.ValidateFormat(stages); err != nil {
		return err
	}
	return s.save(ctx, id, func(p *Presets, id string, isNew bool) bool {
		return upsert(&p.Formats, FormatPreset{ID: id, Name: name, Description: description, Stages: stages}, func(x FormatPreset) string { return x.ID }, isNew)
	})
}

// DuplicatePreset saves a copy of any preset, built-in or saved, under a
// new name.
func (s *PresetService) DuplicatePreset(ctx context.Context, id, name string) error {
	all, err := readPresets(ctx, s.db)
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(all.Win, func(x WinPreset) bool { return x.ID == id }); i >= 0 {
		return s.SaveWinPreset(ctx, "", name, all.Win[i].Win)
	}
	if i := slices.IndexFunc(all.Tiebreaks, func(x TiebreakPreset) bool { return x.ID == id }); i >= 0 {
		return s.SaveTiebreakPreset(ctx, "", name, all.Tiebreaks[i].Tiebreaks)
	}
	if i := slices.IndexFunc(all.Formats, func(x FormatPreset) bool { return x.ID == id }); i >= 0 {
		return s.SaveFormatPreset(ctx, "", name, all.Formats[i].Description, all.Formats[i].Stages)
	}
	return engine.NewError(engine.CodePresetNotFound)
}

// EstimateMinutes is the expected time of a match on court with this win
// condition: its estimate, or the default one.
func (s *PresetService) EstimateMinutes(win engine.WinCondition) int {
	return win.EstimatedMinutes()
}

// PreviewFormat shows what a format gives for a number of entrants, before
// saving it: pools, routing, brackets, matches and final places.
func (s *PresetService) PreviewFormat(stages []tournament.StageConfig, entrants int) (tournament.FormatPreview, error) {
	stages, err := tournament.ValidateFormat(stages)
	if err != nil {
		return tournament.FormatPreview{}, err
	}
	if entrants < 0 || entrants > 512 {
		return tournament.FormatPreview{}, engine.NewError(engine.CodeInvalidFormat)
	}
	return tournament.PreviewFormat(stages, entrants, nil), nil
}

// save creates (id "") or changes a saved preset: change returns false when
// the id is not a saved preset.
func (s *PresetService) save(ctx context.Context, id string, change func(p *Presets, id string, isNew bool) bool) error {
	if strings.HasPrefix(id, "b-") {
		return engine.NewError(engine.CodePresetReadOnly)
	}
	isNew := id == ""
	if isNew {
		id = newPresetID(s.now())
	}
	found := true
	err := s.update(ctx, func(p *Presets) { found = change(p, id, isNew) })
	if err == nil && !found {
		return engine.NewError(engine.CodePresetNotFound)
	}
	return err
}

// upsert replaces the item with the same ID, or appends a new one. It
// returns false for an unknown ID that is not new.
func upsert[T any](list *[]T, item T, idOf func(T) string, isNew bool) bool {
	id := idOf(item)
	if i := slices.IndexFunc(*list, func(x T) bool { return idOf(x) == id }); i >= 0 {
		(*list)[i] = item
		return true
	}
	if !isNew {
		return false
	}
	*list = append(*list, item)
	return true
}

// DeletePreset deletes a saved preset. Built-in presets cannot be deleted.
func (s *PresetService) DeletePreset(ctx context.Context, id string) error {
	if strings.HasPrefix(id, "b-") {
		return engine.NewError(engine.CodePresetReadOnly)
	}
	found := false
	err := s.update(ctx, func(p *Presets) {
		n := len(p.Win) + len(p.Tiebreaks) + len(p.Formats)
		p.Win = slices.DeleteFunc(p.Win, func(x WinPreset) bool { return x.ID == id })
		p.Tiebreaks = slices.DeleteFunc(p.Tiebreaks, func(x TiebreakPreset) bool { return x.ID == id })
		p.Formats = slices.DeleteFunc(p.Formats, func(x FormatPreset) bool { return x.ID == id })
		found = n != len(p.Win)+len(p.Tiebreaks)+len(p.Formats)
		p.Disabled = slices.DeleteFunc(p.Disabled, func(x string) bool { return x == id })
	})
	if err == nil && !found {
		return engine.NewError(engine.CodePresetNotFound)
	}
	return err
}

// SetPresetEnabled offers a preset again, or stops offering it in the
// tournament screens. Built-in presets can be disabled too.
func (s *PresetService) SetPresetEnabled(ctx context.Context, id string, enabled bool) error {
	all, err := readPresets(ctx, s.db)
	if err != nil {
		return err
	}
	if !all.has(PresetKindWin, id) && !all.has(PresetKindTiebreaks, id) && !all.has(PresetKindFormat, id) {
		return engine.NewError(engine.CodePresetNotFound)
	}
	return s.update(ctx, func(p *Presets) {
		p.Disabled = slices.DeleteFunc(p.Disabled, func(x string) bool { return x == id })
		if !enabled {
			p.Disabled = append(p.Disabled, id)
		}
	})
}

func (s *PresetService) update(ctx context.Context, change func(*Presets)) error {
	return updatePresets(ctx, s.db, s.emit, change)
}
