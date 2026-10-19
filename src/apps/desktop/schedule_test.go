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
	if len(v.Slots) != 12 || v.Slots[0].Match != nil || len(v.Order) != 0 || v.End == nil {
		t.Fatalf("before draw = %+v", v)
	}
	for _, team := range f.svc.GetCheckInView().Teams {
		must(f.svc.CheckInEntrant(ctx, team.EntrantID))
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools, stage.PoolSizes = 2, nil }))
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))

	v = f.svc.GetScheduleView()
	if len(v.Order) != 12 || v.ValidatedAt != nil || v.Order[0].Pool != "A" || v.Order[0].LeftA != 3 || len(v.Groups) != 2 {
		t.Fatalf("schedule = %+v", v)
	}
	if g := v.Groups[0]; g.Pool != "A" || len(g.Teams) != 4 || g.Teams[0].Left != 3 {
		t.Errorf("group A = %+v", g)
	}
	// Moves need a validated order.
	wantCode(t, f.svc.MoveScheduledMatch(ctx, v.Order[3].Match.ID, ""), engine.CodeScheduleNotValidated)
	must(f.svc.ValidateSchedule(ctx))
	if v = f.svc.GetScheduleView(); v.ValidatedAt == nil {
		t.Fatal("not validated")
	}
	// Pool B first: the desk proposes it.
	firstB := ""
	for _, o := range v.Order {
		if o.Pool == "B" && firstB == "" {
			firstB = string(o.Match.ID)
		}
	}
	must(f.svc.MoveScheduledPool(ctx, tournament.MatchID(firstB), v.Order[0].Match.ID))
	if v = f.svc.GetScheduleView(); v.Order[0].Pool != "B" || v.Order[5].Pool != "B" || v.Order[6].Pool != "A" {
		t.Errorf("pool B first = %+v", v.Order[:7])
	}
	if d := f.svc.GetControlDeskView(); d.Courts[0].Proposal == nil || d.Courts[0].Proposal.Pool != "B" {
		t.Errorf("desk proposal = %+v", d.Courts[0].Proposal)
	}
	if last := f.svc.GetTournamentView().LastAction; last == nil || last.Type != string(tournament.TypeScheduleOrderSet) {
		t.Errorf("last action = %+v", last)
	}
	wantCode(t, f.svc.SetScheduleMode(ctx, "golf"), engine.CodeInvalidSetting)
}
