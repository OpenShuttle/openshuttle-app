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

func TestBracketViews(t *testing.T) {
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
			{FirstName: "P", LastName: fmt.Sprint("N", i), Club: fmt.Sprint("C", i)},
		}))
	}
	for _, row := range f.svc.GetCheckInView().Rows {
		must(f.svc.CheckInPlayer(ctx, row.ID))
	}
	stage := tournament.DefaultPoolStage()
	stage.Pools = 1
	must(f.svc.ConfigurePoolStage(ctx, engine.MensSingles, stage))
	win := engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 30}
	must(f.svc.AddBracket(ctx, engine.MensSingles, tournament.BracketConfig{Name: "Finale", Win: win}))
	must(f.svc.SetRouting(ctx, engine.MensSingles, []string{"T1", "T1"}))

	fc := f.svc.GetFormatView().Categories[0]
	if len(fc.Brackets) != 1 || fc.Brackets[0].Name != "Finale" || fc.RoutingRows != 4 || fc.Routing[1] != "T1" {
		t.Fatalf("format = %+v", fc)
	}
	if a := f.svc.GetTournamentView().LastAction; a.Type != "RoutingSet" || a.Params["category"] != "MS" {
		t.Errorf("routing action = %+v", a)
	}

	must(f.svc.DrawPools(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))
	bp := f.svc.GetPhasesView().Categories[0].Brackets[0]
	if bp.CanDraw || bp.Drawn {
		t.Fatalf("bracket before pools end = %+v", bp)
	}
	for _, m := range f.svc.GetPhasesView().Categories[0].Pools[0].Matches {
		score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
		if m.A.Entrant > m.B.Entrant {
			score = engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
		}
		must(f.svc.RecordScore(ctx, m.ID, score))
	}
	must(f.svc.DrawBracket(ctx, engine.MensSingles, "T1"))
	if a := f.svc.GetTournamentView().LastAction; a.Params["bracket"] != "Finale" {
		t.Errorf("draw action = %+v", a)
	}

	bp = f.svc.GetPhasesView().Categories[0].Brackets[0]
	if !bp.Drawn || bp.Routed != 2 || len(bp.Slots) != 2 || len(bp.Rounds) != 1 {
		t.Fatalf("bracket = %+v", bp)
	}
	final := bp.Rounds[0].Matches[0]
	if final.Bracket != "T1" || final.BracketName != "Finale" || final.RoundSize != 2 || final.PlaceFrom != 1 || final.PlaceTo != 2 || final.Pool != "" {
		t.Errorf("final view = %+v", final)
	}
	// The final shows up on the control desk once the finalists have rested.
	f.clock = f.clock.Add(15 * time.Minute)
	v := f.svc.GetControlDeskView()
	if v.Courts[0].Proposal == nil || v.Courts[0].Proposal.ID != final.ID {
		t.Errorf("proposal = %+v", v.Courts[0].Proposal)
	}
	must(f.svc.RecordScore(ctx, final.ID, engine.Score{Sets: []engine.SetScore{{A: 21, B: 19}}}))
	bp = f.svc.GetPhasesView().Categories[0].Brackets[0]
	if len(bp.Places) != 2 || bp.Places[0].From != 1 || bp.Places[0].Entrant.Entrant != final.A.Entrant {
		t.Errorf("places = %+v", bp.Places)
	}
}

func TestBracketPreviews(t *testing.T) {
	sets := engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 30}
	brackets := []tournament.Bracket{
		{ID: "T2", BracketConfig: tournament.BracketConfig{Name: "Consolation", Win: sets, FullPlacement: true}},
		{ID: "T1", BracketConfig: tournament.BracketConfig{Name: "Main", Win: sets}},
		{ID: "T3", BracketConfig: tournament.BracketConfig{Name: "Unused", Win: sets}},
	}
	// Three pools of 4, 4 and 3: ranks 1-2 to T1, ranks 3-4 to T2.
	got := bracketPreviews(brackets, []string{"T1", "T1", "T2", "T2"}, []int{4, 4, 3})
	want := []BracketView{
		{ID: "T2", BracketConfig: brackets[0].BracketConfig, Entrants: 5, Byes: 3, Matches: 5, Rounds: 3, PlaceFrom: 7, PlaceTo: 11},
		{ID: "T1", BracketConfig: brackets[1].BracketConfig, Entrants: 6, Byes: 2, Matches: 5, Rounds: 3, PlaceFrom: 1, PlaceTo: 6},
		{ID: "T3", BracketConfig: brackets[2].BracketConfig},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("previews =\n%+v\nwant\n%+v", got, want)
	}
}
