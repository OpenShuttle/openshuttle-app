package main

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestRoundsView(t *testing.T) {
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
	must(f.svc.SetCourts(ctx, 2))
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
	stage.Win = engine.WinCondition{Kind: engine.WinByTime, Minutes: 10}
	must(f.svc.ConfigurePoolStage(ctx, engine.MensSingles, stage))
	must(f.svc.DrawPools(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))

	v := f.svc.GetControlDeskView()
	if !v.Rounds.Active || v.Rounds.Current != nil || len(v.Rounds.Next) != 2 || v.Rounds.Next[1].Court != 2 {
		t.Fatalf("rounds before start = %+v", v.Rounds)
	}
	if v.Courts[0].Proposal != nil {
		t.Error("timed match proposed per court")
	}

	must(f.svc.StartRound(ctx))
	v = f.svc.GetControlDeskView()
	r := v.Rounds.Current
	if r == nil || r.Number != 1 || len(r.Matches) != 2 || !r.EndsAt.Equal(r.StartedAt.Add(10*time.Minute)) {
		t.Fatalf("current = %+v", r)
	}
	if d := f.svc.GetDisplayView(); d.Round == nil || d.Round.Number != 1 || len(d.Calls) != 2 {
		t.Errorf("display = %+v", d)
	}
	if a := f.svc.GetTournamentView().LastAction; a.Type != "RoundStarted" || a.Params["count"] != "2" {
		t.Errorf("start action = %+v", a)
	}
	must(f.svc.EndRound(ctx))
	if v = f.svc.GetControlDeskView(); v.Rounds.Current != nil || len(v.Pending) != 2 || v.Rounds.Played != 1 {
		t.Errorf("after end = %+v / %d pending", v.Rounds, len(v.Pending))
	}
}
