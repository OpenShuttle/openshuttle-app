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
	if len(v.Slots) != 12 || v.Slots[0].Match != nil || v.Slots[0].Movable || v.End == nil || len(v.Blocks) != 6 || !v.Blocks[0].Pending {
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

func TestScheduleDayBreaksAndStyles(t *testing.T) {
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
	for i := range 8 {
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{{Name: fmt.Sprint("P N", i)}}))
	}
	wantCode(t, f.svc.SetDay(ctx, "2026-10-17", "11:00", "10:00"), engine.CodeInvalidTime)
	must(f.svc.SetDay(ctx, "2026-10-17", "10:00", "11:00"))
	must(f.svc.AddBreak(ctx, "12:00", "13:00"))
	if d := f.svc.GetTournamentView().Day; d != (tournament.DaySet{Date: "2026-10-17", Start: "10:00", End: "11:00"}) {
		t.Errorf("day = %+v", d)
	}

	// The clock is 9:00: the forecast starts at 10:00, ends after 11:00.
	v := f.svc.GetScheduleView()
	if v.Start == nil || v.Start.Hour() != 10 || v.DayEnd == nil || !v.Late || len(v.Breaks) != 1 || v.Breaks[0].From.Hour() != 12 {
		t.Fatalf("start %v, end %v, late %v, breaks %+v", v.Start, v.DayEnd, v.Late, v.Breaks)
	}
	// Two pools of 4: their own colors, 3 rounds each; the default acronym.
	colors := map[string]int{}
	for _, b := range v.Blocks {
		colors[b.Pool] = b.Color
		if b.Rounds != 3 || b.Acronym != "" {
			t.Errorf("block %s: rounds %d, acronym %q", b.Key, b.Rounds, b.Acronym)
		}
	}
	if colors["A"] != 1 || colors["B"] != 2 {
		t.Errorf("colors = %v", colors)
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Acronym = "Q" }))
	if s := f.svc.GetScheduleView().Slots[0]; s.Acronym != "Q" || s.Color == 0 || s.Rounds != 3 {
		t.Errorf("slot = %+v", s)
	}

	must(f.svc.RemoveBreak(ctx, v.Breaks[0].ID))
	h := f.svc.GetHistoryView()
	var got []map[string]string
	for _, a := range h.Entries {
		switch a.Type {
		case string(tournament.TypeDaySet), string(tournament.TypeBreakAdded), string(tournament.TypeBreakRemoved):
			got = append(got, a.Params)
		}
	}
	if len(got) != 3 || got[0]["from"] != "12:00" || got[0]["to"] != "13:00" || got[2]["start"] != "10:00" {
		t.Errorf("history = %v", got)
	}
}

func TestScheduleGhostMatches(t *testing.T) {
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
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{{Name: fmt.Sprint("P N", i)}}))
	}
	for _, team := range f.svc.GetCheckInView().Teams {
		must(f.svc.CheckInEntrant(ctx, team.EntrantID))
	}
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))
	// What the desk shows: its queue and proposals.
	deskOf := func() string {
		d := f.svc.GetControlDeskView()
		out := ""
		for _, q := range d.Queue {
			out += string(q.Match.ID) + " "
		}
		for _, c := range d.Courts {
			if c.Proposal != nil {
				out += fmt.Sprint(c.Number, ":", c.Proposal.ID, " ")
			}
		}
		return out
	}
	desk := deskOf()

	first := f.svc.GetScheduleView().Slots[0]
	wantCode(t, f.svc.AddGhostMatch(ctx, 3, 20, ""), engine.CodeInvalidCourt)
	must(f.svc.AddGhostMatch(ctx, first.Court, 20, first.Match.ID))
	v := f.svc.GetScheduleView()
	if len(v.Ghosts) != 1 {
		t.Fatalf("ghosts = %+v", v.Ghosts)
	}
	g := v.Ghosts[0]
	if g.Court != first.Court || g.Minutes != 20 || g.After == nil || g.After.ID != first.Match.ID ||
		g.Start == nil || g.Start.Sub(first.End).Abs() > time.Minute || g.End.Sub(*g.Start) != 20*time.Minute {
		t.Errorf("ghost = %+v", g)
	}
	for _, sl := range v.Slots {
		if sl.Match == nil || sl.Court != g.Court || !sl.Start.Before(*g.End) || !sl.End.After(*g.Start) {
			continue
		}
		t.Errorf("slot %s overlaps the ghost", sl.Match.ID)
	}
	// The control desk does not see them, even one emptying a court now.
	must(f.svc.AddGhostMatch(ctx, 2, 30, ""))
	if v := f.svc.GetScheduleView(); len(v.Ghosts) != 2 || v.Ghosts[1].After != nil || v.Slots[1].Court != 2 || v.Slots[1].Start.Before(*v.Ghosts[1].End) {
		t.Errorf("ghost emptying court 2 now = %+v, slots %+v", v.Ghosts, v.Slots[:2])
	}
	if d := deskOf(); d != desk || desk == "" {
		t.Errorf("the desk shows %q with the ghost, %q without", d, desk)
	}
	must(f.svc.RemoveGhostMatch(ctx, g.ID))
	if v := f.svc.GetScheduleView(); len(v.Ghosts) != 1 {
		t.Errorf("ghosts after removal = %+v", v.Ghosts)
	}
	var got []map[string]string
	for _, a := range f.svc.GetHistoryView().Entries {
		if a.Type == string(tournament.TypeGhostMatchAdded) || a.Type == string(tournament.TypeGhostMatchRemoved) {
			got = append(got, a.Params)
		}
	}
	if len(got) != 3 || got[0]["court"] != fmt.Sprint(g.Court) || got[0]["minutes"] != "20" || got[0]["after"] != string(first.Match.ID) {
		t.Errorf("history = %v", got)
	}
}

func TestScheduleCourtRules(t *testing.T) {
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
	for _, team := range f.svc.GetCheckInView().Teams {
		must(f.svc.CheckInEntrant(ctx, team.EntrantID))
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools, stage.PoolSizes = 2, nil }))
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))

	firstOn := func(v ScheduleView, court int) ScheduleSlotView {
		for _, s := range v.Slots {
			if s.Court == court {
				return s
			}
		}
		return ScheduleSlotView{}
	}
	v := f.svc.GetScheduleView()
	if len(v.CourtRules) != 2 || len(v.CourtRules[0].Rules) != 0 || len(v.Scopes) != 4 || firstOn(v, 1).Pool != "A" {
		t.Fatalf("no rules = %+v, scopes %+v", v.CourtRules, v.Scopes)
	}
	if sc := v.Scopes[3]; sc.Scope.Pool != "B" || sc.Color == 0 || !sc.Left || !sc.EntryStage || sc.Slot != 1 {
		t.Errorf("pool B scope = %+v", sc)
	}

	// Court 2 first for pool B: the preview shows it, nothing saved.
	poolB := tournament.CourtRule{Kind: tournament.CourtPriority, Level: 1, Scope: tournament.CourtScope{Category: engine.MensSingles, Stage: "S1", Pool: "B"}}
	version := f.svc.GetTournamentView().Version
	if p := f.svc.PreviewCourtRule([]int{1}, poolB); firstOn(p, 1).Pool != "B" {
		t.Errorf("preview = %+v", firstOn(p, 1))
	}
	if f.svc.GetTournamentView().Version != version || firstOn(f.svc.GetScheduleView(), 1).Pool != "A" {
		t.Error("the preview was saved")
	}
	wantCode(t, f.svc.AddCourtRule(ctx, []int{3}, poolB), engine.CodeInvalidCourt)
	must(f.svc.AddCourtRule(ctx, []int{1}, poolB))
	v = f.svc.GetScheduleView()
	if r := v.CourtRules[0].Rules; len(r) != 1 || r[0].Scope.Scope != poolB.Scope || r[0].Level != 1 || firstOn(v, 1).Pool != "B" {
		t.Errorf("court 1 rules = %+v", v.CourtRules[0])
	}
	desk := f.svc.GetControlDeskView()
	if p := desk.Courts[0].Proposal; p == nil || p.Pool != "B" {
		t.Errorf("court 1 proposal = %+v", p)
	}

	// Court 2 never pool A: calling a pool A match there warns.
	poolA := tournament.CourtScope{Category: engine.MensSingles, Stage: "S1", Pool: "A"}
	must(f.svc.AddCourtRule(ctx, []int{2}, tournament.CourtRule{Kind: tournament.CourtNever, Scope: poolA}))
	for _, q := range f.svc.GetControlDeskView().Queue {
		want := []int{}
		if q.Match.Pool == "A" {
			want = []int{2}
		}
		if fmt.Sprint(q.OffCourts) != fmt.Sprint(want) {
			t.Errorf("%s off courts = %v", q.Match.ID, q.OffCourts)
		}
	}
	must(f.svc.RemoveCourtRule(ctx, 2, poolA))
	must(f.svc.ClearCourtRules(ctx, 1))
	if v := f.svc.GetScheduleView(); len(v.CourtRules[0].Rules)+len(v.CourtRules[1].Rules) != 0 {
		t.Errorf("rules left = %+v", v.CourtRules)
	}
	if h := f.svc.GetHistoryView(); len(h.Entries) == 0 || h.Entries[0].Type != string(tournament.TypeScheduleSet) {
		t.Errorf("history = %+v", h.Entries[:1])
	}
}
