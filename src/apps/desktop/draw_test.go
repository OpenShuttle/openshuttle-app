package main

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestFormatAndDraw(t *testing.T) {
	f := newFixture(t)
	f.svc.rng = func() *rand.Rand { return rand.New(rand.NewPCG(3, 4)) }
	ctx := f.ctx
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "t.openshuttle"), "Plume"))
	must(f.svc.AddCategory(ctx, engine.MensSingles))

	for i := range 7 {
		club := []string{"Lyon", "Paris"}[i%2]
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{
			{Name: fmt.Sprint("P N", i), Club: club},
		}))
	}

	fv := f.svc.GetFormatView()
	fc := fv.Categories[0]
	if !fc.TimedAllowed || fc.Registered != 7 || fc.Present != 0 || fc.Stages[0].Matches != 6+3 || len(fc.Stages[0].Sizes) != 2 {
		t.Fatalf("format = %+v", fc)
	}

	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools = 3 }))
	if fc = f.svc.GetFormatView().Categories[0]; fc.Stages[0].Pools != 3 || fc.Stages[0].Matches != 3+1+1 {
		t.Errorf("after 3 pools = %+v", fc)
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools = 2 }))

	// Nobody present: cannot draw.
	dv := f.svc.GetDrawView().Categories[0]
	if dv.Status != DrawNone || dv.CanDraw {
		t.Fatalf("draw before check-in = %+v", dv)
	}
	wantCode(t, f.svc.DrawEntry(ctx, engine.MensSingles), engine.CodeNotEnoughEntrants)

	for _, row := range f.svc.GetCheckInView().Rows {
		must(f.svc.CheckInPlayer(ctx, row.ID))
	}
	if dv = f.svc.GetDrawView().Categories[0]; !dv.CanDraw || dv.Planned != 2 || len(dv.Unseeded) != 7 || len(dv.Ranking) != 0 {
		t.Fatalf("draw view after check-in = %+v", dv)
	}
	first, second := dv.Unseeded[3].ID, dv.Unseeded[5].ID
	must(f.svc.SetSeeds(ctx, engine.MensSingles, []engine.EntrantID{second, first}))
	must(f.svc.SetSeeds(ctx, engine.MensSingles, []engine.EntrantID{first, second}))
	if dv = f.svc.GetDrawView().Categories[0]; len(dv.Ranking) != 2 || dv.Ranking[0].ID != first || len(dv.Unseeded) != 5 {
		t.Fatalf("seeds = %+v", dv.Ranking)
	}
	must(f.svc.DrawEntry(ctx, engine.MensSingles))

	dv = f.svc.GetDrawView().Categories[0]
	if dv.Status != DrawDraft || len(dv.Pools) != 2 || dv.Pools[0].Name != "A" || len(dv.Pools[0].Entrants)+len(dv.Pools[1].Entrants) != 7 {
		t.Fatalf("draw = %+v", dv)
	}
	if dv.Pools[0].Entrants[0].ID != first || dv.Pools[0].Entrants[0].Seed != 1 {
		t.Errorf("top seed should open pool A: %+v", dv.Pools[0].Entrants[0])
	}
	// 4 Lyon and 3 Paris in 2 pools: same-club pairs are unavoidable and reported.
	conflicts := 0
	for _, p := range dv.Pools {
		for _, e := range p.Entrants {
			conflicts += len(e.SameClub)
		}
	}
	if conflicts == 0 {
		t.Error("same-club pairs not reported")
	}

	moved := dv.Pools[1].Entrants[0].ID
	must(f.svc.MoveEntrant(ctx, moved, 0))
	if a := f.svc.GetTournamentView().LastAction; a.Type != "EntrantMoved" || a.Params["pool"] != "A" || a.Params["names"] == "" {
		t.Errorf("move action = %+v", a)
	}
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))
	if dv = f.svc.GetDrawView().Categories[0]; dv.Status != DrawConfirmed {
		t.Errorf("status = %s", dv.Status)
	}
	if fc = f.svc.GetFormatView().Categories[0]; !fc.Stages[0].Confirmed {
		t.Error("pools should be locked")
	}
	must(f.svc.Undo(ctx))
	if dv = f.svc.GetDrawView().Categories[0]; dv.Status != DrawDraft {
		t.Errorf("status after undo = %s", dv.Status)
	}
}
