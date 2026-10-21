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

// builtInPresets are always there and cannot be deleted.
func builtInPresets() Presets {
	sets := func(n, points, cap int) engine.WinCondition {
		return engine.WinCondition{Kind: engine.WinBySets, SetsToWin: n, PointsPerSet: points, Cap: cap}
	}
	timed := func(m int) engine.WinCondition { return engine.WinCondition{Kind: engine.WinByTime, Minutes: m} }
	std := sets(2, 15, 21)
	// Pools of 4 when possible, else of 5: the number of pools follows the
	// number of entrants.
	pools := func(id string, routing ...tournament.RouteRule) tournament.StageConfig {
		return tournament.StageConfig{ID: id, Kind: tournament.StagePools, Win: std, PoolSizes: []int{4, 5},
			Tiebreaks: slices.Clone(engine.DefaultTiebreaks), CrossPool: engine.CrossPoolSameMatches, Routing: routing}
	}
	ranks := func(to string, ranks ...int) tournament.RouteRule {
		return tournament.RouteRule{Kind: tournament.RouteRanks, To: to, Ranks: ranks}
	}
	rest := func(to string) tournament.RouteRule { return tournament.RouteRule{Kind: tournament.RouteRest, To: to} }
	bracket := func(id string, full bool) tournament.StageConfig {
		return tournament.StageConfig{ID: id, Kind: tournament.StageBracket, Win: std, FullPlacement: full}
	}
	return Presets{
		Win: []WinPreset{
			{ID: "b-sets-2x15", Win: std},
			{ID: "b-sets-1x15", Win: sets(1, 15, 21)},
			{ID: "b-sets-2x21", Win: sets(2, 21, 30)},
			{ID: "b-sets-1x21", Win: sets(1, 21, 30)},
			{ID: "b-sets-3x11", Win: sets(3, 11, 15)},
			{ID: "b-timed-10", Win: timed(10)},
			{ID: "b-timed-12", Win: timed(12)},
		},
		Tiebreaks: []TiebreakPreset{
			{ID: "b-standard", Key: "standard", Tiebreaks: slices.Clone(engine.DefaultTiebreaks)},
			{ID: "b-differences-first", Key: "differencesFirst", Tiebreaks: []engine.Tiebreak{
				engine.TiebreakWins, engine.TiebreakSetDifference, engine.TiebreakPointDifference,
				engine.TiebreakHeadToHead, engine.TiebreakDraw,
			}},
		},
		Formats: []FormatPreset{
			{ID: "b-pools", Key: "pools", Stages: []tournament.StageConfig{pools("S1")}},
			{ID: "b-pools-main", Key: "poolsMain", Stages: []tournament.StageConfig{
				pools("S1", ranks("S2", 1, 2)), bracket("S2", true),
			}},
			{ID: "b-pools-main-consolation", Key: "poolsMainConsolation", Stages: []tournament.StageConfig{
				pools("S1", ranks("S2", 1, 2), rest("S3")), bracket("S2", true), bracket("S3", true),
			}},
			{ID: "b-pools-best16", Key: "poolsBest16", Stages: []tournament.StageConfig{
				pools("S1", tournament.RouteRule{Kind: tournament.RouteBest, To: "S2", Count: 16}, rest("S3")),
				bracket("S2", true), bracket("S3", true),
			}},
			{ID: "b-bracket", Key: "bracket", Stages: []tournament.StageConfig{bracket("S1", true)}},
			{ID: "b-two-pool-rounds", Key: "twoPoolRounds", Stages: []tournament.StageConfig{
				pools("S1", ranks("S2", 1, 2)), pools("S2", ranks("S3", 1, 2)), bracket("S3", true),
			}},
		},
	}
}

// has reports whether a preset of that kind exists.
func (p Presets) has(kind PresetKind, id string) bool {
	switch kind {
	case PresetKindFormat:
		return slices.ContainsFunc(p.Formats, func(x FormatPreset) bool { return x.ID == id })
	case PresetKindWin:
		return slices.ContainsFunc(p.Win, func(x WinPreset) bool { return x.ID == id })
	case PresetKindTiebreaks:
		return slices.ContainsFunc(p.Tiebreaks, func(x TiebreakPreset) bool { return x.ID == id })
	}
	return false
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
	all.Disabled = user.Disabled
	all.Clubs = user.Clubs
	return all, nil
}

func readUserPresets(ctx context.Context, db *store.AppDB) (Presets, error) {
	p := Presets{Win: []WinPreset{}, Tiebreaks: []TiebreakPreset{}, Formats: []FormatPreset{}, Disabled: []string{}, Clubs: []string{}}
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
