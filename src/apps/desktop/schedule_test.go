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
	if len(v.Slots) != 12 || v.Slots[0].Match != nil || v.Slots[0].Movable || v.End == nil {
		t.Fatalf("before draw = %+v", v)
	}
	for _, team := range f.svc.GetCheckInView().Teams {
		must(f.svc.CheckInEntrant(ctx, team.EntrantID))
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools, stage.PoolSizes = 2, nil }))
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))

	v = f.svc.GetScheduleView()
	if len(v.Slots) != 12 || v.ValidatedAt != nil || v.Slots[0].Pool != "A" || v.Slots[0].LeftA != 3 || v.Slots[0].Movable || len(v.Blocks) != 6 || len(v.Groups) != 2 {
		t.Fatalf("schedule = %+v", v.Slots[0])
	}
	if g := v.Groups[0]; g.Pool != "A" || len(g.Teams) != 4 || g.Teams[0].Left != 3 {
		t.Errorf("group A = %+v", g)
	}
	// Before validation, blocks move: pool B first, the desk proposes it.
	wantCode(t, f.svc.MoveScheduledMatch(ctx, v.Slots[3].Match.ID, ""), engine.CodeScheduleNotValidated)
	// The preview shows the move without saving it; a refused move previews as is.
	version := f.svc.GetTournamentView().Version
	if p := f.svc.PreviewScheduledBlockMove("MS/S1/B/1", "MS/S1/A/1"); p.Slots[0].Pool != "B" {
		t.Errorf("preview pool B first = %+v", p.Slots[0])
	}
	if p := f.svc.PreviewScheduledMatchMove(v.Slots[3].Match.ID, ""); p.Slots[0].Pool != "A" {
		t.Errorf("refused preview = %+v", p.Slots[0])
	}
	if f.svc.GetTournamentView().Version != version || f.svc.GetScheduleView().Slots[0].Pool != "A" {
		t.Error("the preview was saved")
	}
	must(f.svc.MoveScheduledBlock(ctx, "MS/S1/B/1", "MS/S1/A/1"))
	if v = f.svc.GetScheduleView(); v.Slots[0].Pool != "B" || v.Blocks[0].Key != "MS/S1/B/1" || v.Blocks[0].Matches != 2 {
		t.Errorf("pool B first = %+v, blocks %+v", v.Slots[0], v.Blocks)
	}
	if d := f.svc.GetControlDeskView(); d.Courts[0].Proposal == nil || d.Courts[0].Proposal.Pool != "B" {
		t.Errorf("desk proposal = %+v", d.Courts[0].Proposal)
	}
	if last := f.svc.GetTournamentView().LastAction; last == nil || last.Type != string(tournament.TypeScheduleBlocksSet) {
		t.Errorf("last action = %+v", last)
	}
	// Validated: matches move one by one, blocks no longer.
	must(f.svc.ValidateSchedule(ctx))
	wantCode(t, f.svc.MoveScheduledBlock(ctx, "MS/S1/A/1", ""), engine.CodeScheduleValidated)
	v = f.svc.GetScheduleView()
	lastMatch := v.Slots[len(v.Slots)-1].Match.ID
	if p := f.svc.PreviewScheduledMatchMove(lastMatch, v.Slots[0].Match.ID); p.Slots[0].Match.ID != lastMatch {
		t.Errorf("preview match first = %+v", p.Slots[0])
	}
	must(f.svc.MoveScheduledMatch(ctx, lastMatch, v.Slots[0].Match.ID))
	if v = f.svc.GetScheduleView(); v.ValidatedAt == nil || v.Slots[0].Match.ID != lastMatch {
		t.Errorf("after the move = %+v", v.Slots[0])
	}
	// A match called shows as playing, at its court, and no longer moves.
	must(f.svc.CallMatch(ctx, lastMatch, 1))
	v = f.svc.GetScheduleView()
	if s0 := v.Slots[0]; s0.State != SlotPlaying || s0.Court != 1 || s0.Movable || s0.Match.ID != lastMatch {
		t.Errorf("playing slot = %+v", s0)
	}
	must(f.svc.UnvalidateSchedule(ctx))
	if v = f.svc.GetScheduleView(); v.ValidatedAt != nil {
		t.Error("still validated")
	}
	wantCode(t, f.svc.SetScheduleMode(ctx, "golf"), engine.CodeInvalidSetting)
}
