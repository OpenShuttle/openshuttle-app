package main

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestScheduleView(t *testing.T) {
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
	must(f.svc.SetCourts(ctx, 2))
	for i := range 8 {
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{{Name: fmt.Sprint("P N", i)}}))
	}
	// Before the draw: stand-ins for two pools of 4, then nothing called.
	v := f.svc.GetScheduleView()
	if len(v.Slots) != 12 || v.Slots[0].Match != nil || len(v.Blocks) != 2 || v.End == nil {
		t.Fatalf("before draw = %+v", v)
	}
	for _, row := range f.svc.GetCheckInView().Rows {
		must(f.svc.CheckInPlayer(ctx, row.ID))
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools, stage.PoolSizes = 2, nil }))
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))

	v = f.svc.GetScheduleView()
	if len(v.Slots) != 12 || v.Slots[0].Match == nil || v.Slots[0].Pool != "A" || v.Blocks[0].Key != "MS/S1/A" || v.Blocks[0].Matches != 6 {
		t.Fatalf("schedule = %+v", v.Blocks)
	}
	// Moving pool B up: it starts first, and the desk proposes it.
	must(f.svc.SetSchedulePins(ctx, []string{"MS/S1/B"}))
	if v = f.svc.GetScheduleView(); v.Blocks[0].Key != "MS/S1/B" || v.Blocks[0].Pinned != 1 {
		t.Errorf("pinned = %+v", v.Blocks[0])
	}
	if d := f.svc.GetControlDeskView(); d.Courts[0].Proposal == nil || d.Courts[0].Proposal.Pool != "B" {
		t.Errorf("desk proposal = %+v", d.Courts[0].Proposal)
	}
	if last := f.svc.GetTournamentView().LastAction; last == nil || last.Type != string(tournament.TypeScheduleSet) {
		t.Errorf("last action = %+v", last)
	}
	wantCode(t, f.svc.SetScheduleMode(ctx, "golf"), engine.CodeInvalidSetting)
}
