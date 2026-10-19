package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestStageViews(t *testing.T) {
	f := newFixture(t)
	ctx := f.ctx
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "t.openshuttle"), "Plume"))
	must(f.svc.AddCategory(ctx, engine.MensSingles))
	for i := range 4 {
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{
			{Name: fmt.Sprint("P N", i), Club: fmt.Sprint("C", i)},
		}))
	}
	for _, row := range f.svc.GetCheckInView().Rows {
		must(f.svc.CheckInPlayer(ctx, row.ID))
	}
	must(f.svc.AddStage(ctx, engine.MensSingles, tournament.StageBracket))
	win := engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 30}
	stages := func() []tournament.StageConfig {
		var out []tournament.StageConfig
		for _, st := range f.svc.GetFormatView().Categories[0].Stages {
			out = append(out, st.StageConfig)
		}
		return out
	}
	format := stages()
	format[0].Pools, format[0].Routing = 1, tournament.RanksRules("S2", "S2")
	format[1].Name, format[1].Win, format[1].FullPlacement = "Finale", win, false
	must(f.svc.SetFormat(ctx, engine.MensSingles, format))

	fc := f.svc.GetFormatView().Categories[0]
	if len(fc.Stages) != 2 || fc.Stages[1].Name != "Finale" || fc.Stages[1].Phase != 2 || fc.Stages[0].Routing[0].To != "S2" {
		t.Fatalf("format = %+v", fc)
	}
	if s := fc.Stages[1]; s.Entrants != 2 || s.PlaceFrom != 1 || s.PlaceTo != 2 || fc.Ranked != 4 || fc.Ties {
		t.Errorf("preview = %+v, ranked %d", s, fc.Ranked)
	}
	if a := f.svc.GetTournamentView().LastAction; a.Type != "FormatSet" || a.Params["category"] != "MS" {
		t.Errorf("format action = %+v", a)
	}

	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))
	bp := f.svc.GetPhasesView().Categories[0].Stages[1]
	if bp.CanFill || bp.Confirmed {
		t.Fatalf("bracket before pools end = %+v", bp)
	}
	for _, m := range f.svc.GetPhasesView().Categories[0].Stages[0].Pools[0].Matches {
		score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
		if m.A.Entrant > m.B.Entrant {
			score = engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
		}
		must(f.svc.RecordScore(ctx, m.ID, score))
	}
	if v := f.svc.GetControlDeskView(); len(v.StagesToFill) != 1 || v.StagesToFill[0].Stage != "S2" || len(v.Complete) != 0 {
		t.Errorf("alerts = %+v / %+v", v.StagesToFill, v.Complete)
	}
	must(f.svc.FillStage(ctx, engine.MensSingles, "S2"))
	if a := f.svc.GetTournamentView().LastAction; a.Type != "StageFilled" || a.Params["stage"] != "Finale" {
		t.Errorf("fill action = %+v", a)
	}

	bp = f.svc.GetPhasesView().Categories[0].Stages[1]
	if !bp.Confirmed || bp.Routed != 2 || len(bp.Slots) != 2 || len(bp.Rounds) != 1 {
		t.Fatalf("bracket = %+v", bp)
	}
	final := bp.Rounds[0].Matches[0]
	if final.Stage != "S2" || final.StageName != "Finale" || final.StageKind != tournament.StageBracket || final.EntryStage ||
		final.RoundSize != 2 || final.PlaceFrom != 1 || final.PlaceTo != 2 || final.Pool != "" {
		t.Errorf("final view = %+v", final)
	}
	// The final shows up on the control desk once the finalists have rested.
	f.clock = f.clock.Add(15 * time.Minute)
	v := f.svc.GetControlDeskView()
	if v.Courts[0].Proposal == nil || v.Courts[0].Proposal.ID != final.ID {
		t.Errorf("proposal = %+v", v.Courts[0].Proposal)
	}
	must(f.svc.RecordScore(ctx, final.ID, engine.Score{Sets: []engine.SetScore{{A: 21, B: 19}}}))
	bp = f.svc.GetPhasesView().Categories[0].Stages[1]
	if len(bp.Places) != 2 || bp.Places[0].From != 1 || bp.Places[0].Entrant.Entrant != final.A.Entrant {
		t.Errorf("places = %+v", bp.Places)
	}
}

func TestFormatPreview(t *testing.T) {
	// 11 entrants in 3 pools (4, 4, 3): ranks 1-2 to a main bracket with
	// every place played, rank 3 to a consolation bracket, rank 4 kept.
	f := newFixture(t)
	ctx := f.ctx
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "p.openshuttle"), "Plume"))
	must(f.svc.AddCategory(ctx, engine.MensSingles))
	for i := range 11 {
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{{Name: fmt.Sprint("P N", i)}}))
	}
	sets := engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 30}
	must(f.svc.AddStage(ctx, engine.MensSingles, tournament.StageBracket))
	must(f.svc.AddStage(ctx, engine.MensSingles, tournament.StageBracket))
	var format []tournament.StageConfig
	for _, st := range f.svc.GetFormatView().Categories[0].Stages {
		format = append(format, st.StageConfig)
	}
	format[0].Pools, format[0].Routing = 3, tournament.RanksRules("S2", "S2", "S3")
	format[1].Win, format[1].FullPlacement = sets, true
	format[2].Win, format[2].FullPlacement = sets, false
	must(f.svc.SetFormat(ctx, engine.MensSingles, format))

	fc := f.svc.GetFormatView().Categories[0]
	pools, main, cons := fc.Stages[0], fc.Stages[1], fc.Stages[2]
	if !reflect.DeepEqual(pools.Sizes, []int{4, 4, 3}) || pools.Matches != 6+6+3 || pools.Rounds != 3 {
		t.Errorf("pools = %+v", pools)
	}
	if main.Entrants != 6 || main.Byes != 2 || main.Matches != 7 || main.Rounds != 3 || main.PlaceFrom != 1 || main.PlaceTo != 6 {
		t.Errorf("main = %+v", main)
	}
	// Without full placement: places 1, 2, then 3 tied (one semi lost).
	if cons.Entrants != 3 || cons.PlaceFrom != 7 || cons.PlaceTo != 9 ||
		!reflect.DeepEqual(cons.Ranks, []tournament.RankCount{{Rank: 1, Count: 1}, {Rank: 2, Count: 1}, {Rank: 3, Count: 1}}) {
		t.Errorf("consolation = %+v", cons)
	}
	if pools.PlaceFrom != 10 || pools.PlaceTo != 11 || fc.Ranked != 11 || !fc.Ties {
		t.Errorf("kept in pools = %d-%d, ranked %d, ties %v", pools.PlaceFrom, pools.PlaceTo, fc.Ranked, fc.Ties)
	}
}
