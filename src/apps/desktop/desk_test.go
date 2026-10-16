package main

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestControlDesk(t *testing.T) {
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
	for i := range 4 {
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{
			{FirstName: "P", LastName: fmt.Sprint("N", i), Club: fmt.Sprint("C", i)},
		}))
	}
	if v := f.svc.GetControlDeskView(); v.Started || len(v.Courts) != 2 {
		t.Fatalf("before draw = %+v", v)
	}
	for _, row := range f.svc.GetCheckInView().Rows {
		must(f.svc.CheckInPlayer(ctx, row.ID))
	}
	stage := tournament.DefaultPoolStage()
	stage.Pools = 1
	must(f.svc.ConfigurePoolStage(ctx, engine.MensSingles, stage))
	must(f.svc.DrawPools(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))

	v := f.svc.GetControlDeskView()
	if !v.Started || v.Courts[0].Proposal == nil || v.Courts[0].Proposal.ID != "MS-A1" || v.Courts[1].Proposal.ID != "MS-A2" {
		t.Fatalf("proposals = %+v", v.Courts)
	}
	p := v.Courts[0].Proposal
	if p.Pool != "A" || p.Round != 1 || p.Slot != 1 || p.A.Bib == 0 || len(p.A.Names) != 1 || p.Win.PointsPerSet != 21 {
		t.Errorf("match view = %+v", p)
	}

	must(f.svc.CallMatch(ctx, "MS-A1", 1))
	if a := f.svc.GetTournamentView().LastAction; a.Type != "MatchCalled" || a.Params["court"] != "1" || a.Params["a"] == "" {
		t.Errorf("call action = %+v", a)
	}
	d := f.svc.GetDisplayView()
	if len(d.Boards) != 1 || d.Boards[0].Pool != "A" || len(d.Boards[0].Standings) != 4 {
		t.Errorf("boards = %+v", d.Boards)
	}
	if len(d.Calls) != 1 || d.Calls[0].ID != "MS-A1" || d.Courts[0].Match == nil || d.Courts[1].Match != nil {
		t.Errorf("display = %+v", d)
	}

	f.clock = f.clock.Add(12 * time.Minute)
	must(f.svc.EndMatch(ctx, "MS-A1"))
	v = f.svc.GetControlDeskView()
	if v.Courts[0].State != tournament.CourtFinished || len(v.Pending) != 1 || v.Pending[0].EndedAt == nil {
		t.Fatalf("after end = %+v", v)
	}
	// The queue explains why round 2 waits.
	var resting int
	for _, q := range v.Queue {
		if q.Reason == tournament.ReasonResting && q.RestingSince != nil {
			resting++
		}
	}
	if resting == 0 {
		t.Errorf("no resting reason in %+v", v.Queue)
	}

	wantCode(t, f.svc.RecordScore(ctx, "MS-A1", engine.Score{Sets: []engine.SetScore{{A: 21, B: 20}, {A: 3, B: 21}}}), engine.CodeInvalidSetScore)
	must(f.svc.RecordScore(ctx, "MS-A1", engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 19, B: 21}, {A: 30, B: 29}}}))
	if a := f.svc.GetTournamentView().LastAction; a.Params["score"] != "21-10 19-21 30-29" {
		t.Errorf("score action = %+v", a)
	}
	if v = f.svc.GetControlDeskView(); v.Courts[0].State != tournament.CourtFree || len(v.Pending) != 0 {
		t.Errorf("after score = %+v", v.Courts[0])
	}
	if tv := f.svc.GetTournamentView(); tv.Courts != 2 || tv.MinRest != 10 || tv.Priority != "balanced" {
		t.Errorf("settings = %+v", tv)
	}
}
