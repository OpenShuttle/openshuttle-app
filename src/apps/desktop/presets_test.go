package main

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestPresets(t *testing.T) {
	f := newFixture(t)
	ctx := f.ctx
	presets := NewPresetService(f.app, f.rec.emit, func() time.Time { return f.clock })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	p, err := presets.GetPresets(ctx)
	if err != nil || len(p.Win) == 0 || len(p.Tiebreaks) == 0 || len(p.Formats) == 0 {
		t.Fatalf("built-in presets = %+v, %v", p, err)
	}
	builtIns := len(p.Win)

	win := engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 1, PointsPerSet: 11, Cap: 15}
	must(presets.SaveWinPreset(ctx, "", "  Court  ", win))
	wantCode(t, presets.SaveWinPreset(ctx, "", " ", win), engine.CodeInvalidName)
	wantCode(t, presets.SaveWinPreset(ctx, "", "X", engine.WinCondition{}), engine.CodeInvalidWinCondition)
	wantCode(t, presets.SaveWinPreset(ctx, "b-sets-2x21", "Mine", win), engine.CodePresetReadOnly)
	wantCode(t, presets.SaveWinPreset(ctx, "u404", "Gone", win), engine.CodePresetNotFound)
	order := []engine.Tiebreak{engine.TiebreakWins, engine.TiebreakDraw}
	must(presets.SaveTiebreakPreset(ctx, "", "Rapide", order))
	wantCode(t, presets.SaveTiebreakPreset(ctx, "", "Bad", []engine.Tiebreak{"elo"}), engine.CodeInvalidFormat)
	if got := f.rec.last(); got.name != EventPresetsChanged {
		t.Errorf("emitted %+v", got)
	}

	p, _ = presets.GetPresets(ctx)
	saved := p.Win[len(p.Win)-1]
	if len(p.Win) != builtIns+1 || saved.Name != "Court" || saved.Win != win {
		t.Fatalf("win presets = %+v", p.Win)
	}
	if tb := p.Tiebreaks[len(p.Tiebreaks)-1]; tb.Name != "Rapide" || !slices.Equal(tb.Tiebreaks, order) {
		t.Errorf("tiebreak preset = %+v", tb)
	}

	// Edit a saved preset in place; duplicate a built-in one.
	f.clock = f.clock.Add(time.Second)
	must(presets.SaveWinPreset(ctx, saved.ID, "Court 11", win))
	must(presets.DuplicatePreset(ctx, "b-sets-1x21", "Un set"))
	p, _ = presets.GetPresets(ctx)
	if len(p.Win) != builtIns+2 || p.Win[builtIns].Name != "Court 11" || p.Win[builtIns+1].Name != "Un set" || p.Win[builtIns+1].Win.SetsToWin != 1 {
		t.Errorf("after edit and duplicate = %+v", p.Win[builtIns:])
	}

	// Format presets, made outside any tournament, and simulated.
	stages := []tournament.StageConfig{
		{ID: "A", Kind: tournament.StagePools, PoolSizes: []int{4, 5}, Win: win, Tiebreaks: engine.DefaultTiebreaks,
			Routing: []tournament.RouteRule{{Kind: tournament.RouteBest, To: "B", Count: 16}, {Kind: tournament.RouteRest, To: "C"}}},
		{ID: "B", Kind: tournament.StageBracket, Win: win, FullPlacement: true},
		{ID: "C", Kind: tournament.StageBracket, Win: win, FullPlacement: true},
	}
	must(presets.SaveFormatPreset(ctx, "", "Top 16", "", stages))
	wantCode(t, presets.SaveFormatPreset(ctx, "", "Bad", "", stages[1:2:2][:0]), engine.CodeInvalidFormat)
	sim, err := presets.PreviewFormat(stages, 22)
	if err != nil || sim.Stages[1].Entrants != 16 || sim.Stages[2].Entrants != 6 || sim.Ranked != 22 {
		t.Errorf("simulation = %+v, %v", sim, err)
	}

	// Formats: save one from a tournament, apply it to another category.
	must(f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "p.openshuttle"), "Plume"))
	must(f.svc.AddCategory(ctx, engine.MensDoubles))
	must(f.svc.AddCategory(ctx, engine.WomensDoubles))
	must(f.svc.AddStage(ctx, engine.MensDoubles, tournament.StageBracket))
	must(f.svc.SaveFormatPreset(ctx, engine.MensDoubles, "Mon format", "Poules puis tableau"))
	p, _ = presets.GetPresets(ctx)
	mine := p.Formats[len(p.Formats)-1]
	if mine.Name != "Mon format" || mine.Description != "Poules puis tableau" || len(mine.Stages) != 2 {
		t.Fatalf("format preset = %+v", mine)
	}
	must(f.svc.ApplyFormatPreset(ctx, engine.WomensDoubles, mine.ID))
	if fc := f.svc.GetFormatView().Categories[1]; len(fc.Stages) != 2 || fc.Stages[1].Kind != tournament.StageBracket {
		t.Errorf("applied = %+v", fc.Stages)
	}
	must(f.svc.ApplyFormatPreset(ctx, engine.WomensDoubles, "b-pools-main-consolation"))
	if fc := f.svc.GetFormatView().Categories[1]; len(fc.Stages) != 3 || len(fc.Stages[0].Routing) != 2 ||
		!ruleEq(fc.Stages[0].Routing[1], tournament.RouteRule{Kind: tournament.RouteRest, To: "S3"}) {
		t.Errorf("built-in applied = %+v", fc.Stages)
	}
	wantCode(t, f.svc.ApplyFormatPreset(ctx, engine.WomensDoubles, "nope"), engine.CodePresetNotFound)

	// Disabling: any preset, built-in ones included; deleting forgets it.
	must(presets.SetPresetEnabled(ctx, "b-pools", false))
	must(presets.SetPresetEnabled(ctx, "b-pools", false))
	must(presets.SetPresetEnabled(ctx, mine.ID, false))
	wantCode(t, presets.SetPresetEnabled(ctx, "nope", false), engine.CodePresetNotFound)
	if p, _ = presets.GetPresets(ctx); !slices.Equal(p.Disabled, []string{"b-pools", mine.ID}) {
		t.Errorf("disabled = %v", p.Disabled)
	}
	must(presets.SetPresetEnabled(ctx, "b-pools", true))

	// Deleting: saved presets only.
	must(presets.DeletePreset(ctx, mine.ID))
	wantCode(t, presets.DeletePreset(ctx, mine.ID), engine.CodePresetNotFound)
	wantCode(t, presets.DeletePreset(ctx, "b-pools"), engine.CodePresetReadOnly)
	if p, _ = presets.GetPresets(ctx); slices.ContainsFunc(p.Formats, func(x FormatPreset) bool { return x.ID == mine.ID }) {
		t.Error("deleted preset still listed")
	}
	if len(p.Disabled) != 0 {
		t.Errorf("disabled after delete = %v", p.Disabled)
	}

	// Every built-in format is a valid format.
	for _, fp := range builtInPresets().Formats {
		must(f.svc.ApplyFormatPreset(ctx, engine.MensDoubles, fp.ID))
	}
}

func ruleEq(a, b tournament.RouteRule) bool {
	return a.Kind == b.Kind && a.To == b.To && a.Rank == b.Rank && a.Count == b.Count && slices.Equal(a.Ranks, b.Ranks)
}

func TestClubs(t *testing.T) {
	f := newFixture(t)
	presets := NewPresetService(f.app, f.rec.emit, func() time.Time { return f.clock })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	clubs := func() []string {
		p, err := presets.GetPresets(f.ctx)
		must(err)
		return p.Clubs
	}
	must(presets.SaveClub(f.ctx, "", "  BC  Lyon "))
	must(presets.SaveClub(f.ctx, "", "Écully BC"))
	must(presets.SaveClub(f.ctx, "", "bc lyon"))
	if got := clubs(); !slices.Equal(got, []string{"BC Lyon", "Écully BC"}) {
		t.Errorf("clubs = %v", got)
	}
	must(presets.SaveClub(f.ctx, "BC Lyon", "ASPTT"))
	wantCode(t, presets.SaveClub(f.ctx, "Nope", "X"), engine.CodeClubNotFound)
	wantCode(t, presets.SaveClub(f.ctx, "", " "), engine.CodeInvalidClub)
	must(presets.DeleteClub(f.ctx, "Écully BC"))
	wantCode(t, presets.DeleteClub(f.ctx, "Écully BC"), engine.CodeClubNotFound)

	// Clubs typed for players join the list.
	f.dialogs.save = filepath.Join(f.dir, "clubs")
	if _, err := f.svc.CreateTournament(f.ctx, "Plume"); err != nil {
		t.Fatal(err)
	}
	must(f.svc.AddCategory(f.ctx, engine.MensSingles))
	must(f.svc.RegisterEntrant(f.ctx, engine.MensSingles, []tournament.PlayerInput{{Name: "Paul Zola", Club: "Badminton Villeurbanne"}}))
	if got := clubs(); !slices.Equal(got, []string{"ASPTT", "Badminton Villeurbanne"}) {
		t.Errorf("clubs = %v", got)
	}
}
