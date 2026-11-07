package main

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
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
			{Name: fmt.Sprint("P N", i), Club: fmt.Sprint("C", i)},
		}))
	}
	if v := f.svc.GetControlDeskView(); v.Started || len(v.Courts) != 2 {
		t.Fatalf("before draw = %+v", v)
	}
	for _, team := range f.svc.GetCheckInView().Teams {
		must(f.svc.CheckInEntrant(ctx, team.EntrantID))
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) {
		stage.Pools = 1
	}))
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))

	v := f.svc.GetControlDeskView()
	if !v.Started || v.Courts[0].Proposal == nil || v.Courts[0].Proposal.ID != "MS-A1" || v.Courts[1].Proposal.ID != "MS-A2" {
		t.Fatalf("proposals = %+v", v.Courts)
	}
	p := v.Courts[0].Proposal
	if p.Pool != "A" || p.Round != 1 || p.Slot != 1 || p.Color != 1 || p.A.Bib == 0 || len(p.A.Names) != 1 || p.Win.PointsPerSet != 21 {
		t.Errorf("match view = %+v", p)
	}

	must(f.svc.CallMatch(ctx, "MS-A1", 1))
	if a := f.svc.GetTournamentView().LastAction; a.Type != "MatchCalled" || a.Params["court"] != "1" || a.Params["a"] == "" {
		t.Errorf("call action = %+v", a)
	}
	// The history names the teams without bibs unless they are shown.
	if a := f.svc.GetTournamentView().LastAction; strings.Contains(a.Params["a"], "#") {
		t.Errorf("bibs hidden, call action = %+v", a)
	}
	must(f.settings.SetShowBibs(ctx, true))
	if a := f.svc.GetTournamentView().LastAction; !strings.HasPrefix(a.Params["a"], "#") {
		t.Errorf("bibs shown, call action = %+v", a)
	}
	must(f.settings.SetShowBibs(ctx, false))
	d := f.svc.GetDisplayView()
	if len(d.Stages) != 1 || d.Stages[0].ID != "S1" || d.Stages[0].Slot != 1 || len(d.Stages[0].Pools) != 1 ||
		len(d.Stages[0].Pools[0].Standings) != 4 || d.Name == "" {
		t.Errorf("display = %+v", d)
	}
	// A stage the desk hides leaves the display screen.
	must(f.svc.SetStageDisplay(ctx, engine.MensSingles, "S1", false))
	if d := f.svc.GetDisplayView(); len(d.Stages) != 0 || f.svc.GetPhasesView().Categories[0].Stages[0].Displayed {
		t.Errorf("hidden stage shown: %+v", d.Stages)
	}
	if a := f.svc.GetTournamentView().LastAction; a.Type != "StageDisplaySet" || a.Params["shown"] != "false" || a.Params["stage"] != "S1" {
		t.Errorf("display action = %+v", a)
	}
	must(f.svc.SetStageDisplay(ctx, engine.MensSingles, "S1", true))

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
	if tv := f.svc.GetTournamentView(); tv.Courts != 2 || tv.MinRest != 10 {
		t.Errorf("settings = %+v", tv)
	}
}

// A swap in the history names the teams of both matches, never their codes.
func TestHistorySwapNamesTeams(t *testing.T) {
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
	for i := range 4 {
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{{Name: fmt.Sprint("P N", i)}}))
	}
	for _, team := range f.svc.GetCheckInView().Teams {
		must(f.svc.CheckInEntrant(ctx, team.EntrantID))
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools = 1 }))
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))
	must(f.svc.SwapMatches(ctx, "MS-A2", "MS-A3"))
	a := f.svc.GetTournamentView().LastAction
	for _, k := range []string{"first", "second"} {
		if p := a.Params[k]; !strings.Contains(p, " – ") || strings.Contains(p, "MS-") || strings.Contains(p, "#") {
			t.Errorf("%s = %q", k, p)
		}
	}
}
