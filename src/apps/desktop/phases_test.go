package main

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestPhasesAndHistory(t *testing.T) {
	f := newFixture(t)
	f.svc.rng = func() *rand.Rand { return rand.New(rand.NewPCG(5, 6)) }
	ctx := f.ctx
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "t.openshuttle"), "Plume"))
	must(f.svc.AddCategory(ctx, engine.MensSingles))
	for i := range 3 {
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{
			{FirstName: "P", LastName: fmt.Sprint("N", i), Club: fmt.Sprint("C", i)},
		}))
	}
	if v := f.svc.GetPhasesView(); v.Categories[0].Started {
		t.Fatal("started before the draw")
	}
	for _, row := range f.svc.GetCheckInView().Rows {
		must(f.svc.CheckInPlayer(ctx, row.ID))
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) {
		stage.Pools = 1
	}))
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))

	// A perfect three-way tie, then lots.
	win := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
	loss := engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
	must(f.svc.RecordScore(ctx, "MS-A1", win))
	must(f.svc.RecordScore(ctx, "MS-A2", loss))
	must(f.svc.RecordScore(ctx, "MS-A3", win))

	pool := f.svc.GetPhasesView().Categories[0].Stages[0].Pools[0]
	if !pool.Complete || !pool.DrawNeeded || len(pool.Standings) != 3 || len(pool.Matches) != 3 {
		t.Fatalf("pool = %+v", pool)
	}
	must(f.svc.DrawLots(ctx, engine.MensSingles, "S1", 0))
	pool = f.svc.GetPhasesView().Categories[0].Stages[0].Pools[0]
	if pool.DrawNeeded || pool.Standings[2].Rank != 3 || pool.Standings[0].Entrant.Bib == 0 {
		t.Errorf("after lots = %+v", pool.Standings)
	}

	// Corrections and history.
	must(f.svc.ClearScore(ctx, "MS-A1"))
	must(f.svc.Undo(ctx))
	h := f.svc.GetHistoryView().Entries
	if h[0].Type != "ActionUndone" || h[1].Type != "ScoreCleared" || !h[1].Undone || h[2].Undone {
		t.Errorf("history head = %+v / %+v / %+v", h[0], h[1], h[2])
	}
	if h[len(h)-1].Type != "TournamentCreated" {
		t.Errorf("oldest entry = %+v", h[len(h)-1])
	}
}
