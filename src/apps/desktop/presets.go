package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// Presets are saved in app.db and shared by every tournament: win
// conditions, tiebreak orders and whole formats. Applying one copies it into
// the tournament, which stays self-contained (ADR 4).

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

// Presets lists every preset, built-in ones first.
type Presets struct {
	Win       []WinPreset      `json:"win"`
	Tiebreaks []TiebreakPreset `json:"tiebreaks"`
	Formats   []FormatPreset   `json:"formats"`
}

// builtInPresets are always there and cannot be deleted.
func builtInPresets() Presets {
	sets := func(n, points, cap int) engine.WinCondition {
		return engine.WinCondition{Kind: engine.WinBySets, SetsToWin: n, PointsPerSet: points, Cap: cap}
	}
	timed := func(m int) engine.WinCondition { return engine.WinCondition{Kind: engine.WinByTime, Minutes: m} }
	std := sets(2, 21, 30)
	pools := func(id string, n int, routing ...string) tournament.StageConfig {
		return tournament.StageConfig{ID: id, Kind: tournament.StagePools, Win: std, Pools: n,
			Tiebreaks: slices.Clone(engine.DefaultTiebreaks), Routing: routing}
	}
	bracket := func(id string, full bool) tournament.StageConfig {
		return tournament.StageConfig{ID: id, Kind: tournament.StageBracket, Win: std, FullPlacement: full}
	}
	return Presets{
		Win: []WinPreset{
			{ID: "b-sets-2x21", Win: std},
			{ID: "b-sets-1x21", Win: sets(1, 21, 30)},
			{ID: "b-sets-2x15", Win: sets(2, 15, 21)},
			{ID: "b-sets-1x15", Win: sets(1, 15, 21)},
			{ID: "b-sets-3x11", Win: sets(3, 11, 15)},
			{ID: "b-timed-10", Win: timed(10)},
			{ID: "b-timed-12", Win: timed(12)},
		},
		Tiebreaks: []TiebreakPreset{
			{ID: "b-standard", Key: "standard", Tiebreaks: slices.Clone(engine.DefaultTiebreaks)},
			{ID: "b-points-first", Key: "pointsFirst", Tiebreaks: []engine.Tiebreak{
				engine.TiebreakWins, engine.TiebreakPointDifference, engine.TiebreakSetDifference,
				engine.TiebreakHeadToHead, engine.TiebreakDraw,
			}},
		},
		Formats: []FormatPreset{
			{ID: "b-pools", Key: "pools", Stages: []tournament.StageConfig{pools("S1", 4)}},
			{ID: "b-pools-main", Key: "poolsMain", Stages: []tournament.StageConfig{
				pools("S1", 4, "S2", "S2"), bracket("S2", true),
			}},
			{ID: "b-pools-main-consolation", Key: "poolsMainConsolation", Stages: []tournament.StageConfig{
				pools("S1", 4, "S2", "S2", "S3", "S3"), bracket("S2", true), bracket("S3", true),
			}},
			{ID: "b-bracket", Key: "bracket", Stages: []tournament.StageConfig{bracket("S1", true)}},
			{ID: "b-two-pool-rounds", Key: "twoPoolRounds", Stages: []tournament.StageConfig{
				pools("S1", 4, "S2", "S2"), pools("S2", 2, "S3", "S3"), bracket("S3", true),
			}},
		},
	}
}

// readPresets returns the built-in presets, then the saved ones.
func readPresets(ctx context.Context, db *store.AppDB) (Presets, error) {
	all := builtInPresets()
	user, err := readUserPresets(ctx, db)
	if err != nil {
		return all, err
	}
	all.Win = append(all.Win, user.Win...)
	all.Tiebreaks = append(all.Tiebreaks, user.Tiebreaks...)
	all.Formats = append(all.Formats, user.Formats...)
	return all, nil
}

func readUserPresets(ctx context.Context, db *store.AppDB) (Presets, error) {
	p := Presets{Win: []WinPreset{}, Tiebreaks: []TiebreakPreset{}, Formats: []FormatPreset{}}
	raw, ok, err := db.Setting(ctx, settingPresets)
	if err != nil || !ok {
		return p, err
	}
	// A corrupt list is not worth blocking the user: start over.
	_ = json.Unmarshal([]byte(raw), &p)
	return p, nil
}

func writeUserPresets(ctx context.Context, db *store.AppDB, p Presets) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return db.SetSetting(ctx, settingPresets, string(raw))
}

// presetName trims a name and checks its length.
func presetName(name string, maxLen int, required bool) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if (required && name == "") || utf8.RuneCountInString(name) > maxLen {
		return "", engine.NewError(engine.CodeInvalidName)
	}
	return name, nil
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

// SaveWinPreset saves a win condition under a name.
func (s *PresetService) SaveWinPreset(ctx context.Context, name string, win engine.WinCondition) error {
	name, err := presetName(name, maxPresetName, true)
	if err != nil {
		return err
	}
	if err := win.Validate(); err != nil {
		return err
	}
	return s.update(ctx, func(p *Presets) {
		p.Win = append(p.Win, WinPreset{ID: newPresetID(s.now()), Name: name, Win: win})
	})
}

// SaveTiebreakPreset saves a tiebreak order under a name.
func (s *PresetService) SaveTiebreakPreset(ctx context.Context, name string, tiebreaks []engine.Tiebreak) error {
	name, err := presetName(name, maxPresetName, true)
	if err != nil {
		return err
	}
	if !tournament.ValidTiebreaks(tiebreaks) {
		return engine.NewError(engine.CodeInvalidFormat)
	}
	return s.update(ctx, func(p *Presets) {
		p.Tiebreaks = append(p.Tiebreaks, TiebreakPreset{ID: newPresetID(s.now()), Name: name, Tiebreaks: slices.Clone(tiebreaks)})
	})
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
	})
	if err == nil && !found {
		return engine.NewError(engine.CodePresetNotFound)
	}
	return err
}

func (s *PresetService) update(ctx context.Context, change func(*Presets)) error {
	return updatePresets(ctx, s.db, s.emit, change)
}

func updatePresets(ctx context.Context, db *store.AppDB, emit func(string, any), change func(*Presets)) error {
	p, err := readUserPresets(ctx, db)
	if err != nil {
		return err
	}
	change(&p)
	if err := writeUserPresets(ctx, db, p); err != nil {
		return err
	}
	all, err := readPresets(ctx, db)
	if err != nil {
		return err
	}
	emit(EventPresetsChanged, all)
	return nil
}

func newPresetID(now time.Time) string {
	return fmt.Sprintf("u%d", now.UnixNano())
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
