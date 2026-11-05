package tournament

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func ghostIDs(gs []GhostMatch) []string {
	out := []string{}
	for _, g := range gs {
		out = append(out, g.ID)
	}
	return out
}

func ghostSlot(sc Schedule, id string) (ScheduleSlot, bool) {
	i := slices.IndexFunc(sc.Slots, func(sl ScheduleSlot) bool { return sl.Ghost == id })
	if i < 0 {
		return ScheduleSlot{}, false
	}
	return sc.Slots[i], true
}

func TestAddRemoveGhostMatch(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4)
	g.do(func(s State) (Change, error) { return s.SetCourts(2) })
	tests := []struct {
		name    string
		court   int
		minutes int
		after   MatchID
		code    engine.Code
	}{
		{"no court", 0, 20, "", engine.CodeInvalidCourt},
		{"court not there", 3, 20, "", engine.CodeInvalidCourt},
		{"no minutes", 1, 0, "", engine.CodeInvalidTime},
		{"too long", 1, MaxGhostMinutes + 1, "", engine.CodeInvalidTime},
		{"unknown match", 1, 20, "MS-Z9", engine.CodeMatchNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g.fails(tt.code, func(s State) (Change, error) { return s.AddGhostMatch(tt.court, tt.minutes, tt.after) })
		})
	}
	n := len(g.log)
	g.do(func(s State) (Change, error) { return s.AddGhostMatch(2, 30, "MS-A1") })
	g.do(func(s State) (Change, error) { return s.AddGhostMatch(1, 15, "") })
	want := []GhostMatch{
		{ID: fmt.Sprintf("g%d", n+1), Court: 2, Minutes: 30, After: "MS-A1", AddedAt: t0},
		{ID: fmt.Sprintf("g%d", n+2), Court: 1, Minutes: 15, AddedAt: t0},
	}
	if got := g.state().GhostMatches(); !slices.Equal(got, want) {
		t.Fatalf("ghosts = %v, want %v", got, want)
	}
	g.fails(engine.CodeInvalidSetting, func(s State) (Change, error) { return s.RemoveGhostMatch("g999") })
	g.do(func(s State) (Change, error) { return s.RemoveGhostMatch(want[0].ID) })
	if got := ghostIDs(g.state().GhostMatches()); !slices.Equal(got, []string{want[1].ID}) {
		t.Errorf("after removal = %v", got)
	}
	g.undo() // the removal
	g.undo() // the second ghost
	if got := ghostIDs(g.state().GhostMatches()); !slices.Equal(got, []string{want[0].ID}) {
		t.Errorf("after undos = %v", got)
	}
	// Fewer courts drop the ghost of a court no longer there.
	g.do(func(s State) (Change, error) { return s.SetCourts(1) })
	if got := g.state().GhostMatches(); len(got) != 0 {
		t.Errorf("ghosts on 1 court = %v", got)
	}
}

func TestGhostMatchInTheSchedule(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4) // 3 rounds of 2 matches, 29 min each
	g.do(func(s State) (Change, error) { return s.SetCourts(2) })
	g.do(func(s State) (Change, error) { return s.AddGhostMatch(1, 30, "MS-A1") })
	id := g.state().GhostMatches()[0].ID
	min := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }

	st := g.state()
	sc := st.ScheduleWithGhosts(t0)
	a1 := sc.Slots[slices.IndexFunc(sc.Slots, func(sl ScheduleSlot) bool { return sl.Match == "MS-A1" })].End
	gs, ok := ghostSlot(sc, id)
	if !ok || gs.Court != 1 || !gs.Start.Equal(a1) || !gs.End.Equal(a1.Add(30*time.Minute)) {
		t.Fatalf("ghost slot = %+v, want court 1 for 30 min from %v", gs, a1)
	}
	for _, sl := range sc.Slots {
		if sl.Ghost == "" && sl.Court == 1 && sl.Start.Before(gs.End) && sl.End.After(gs.Start) {
			t.Errorf("match %s on court 1 %v-%v during the ghost", sl.Match, sl.Start, sl.End)
		}
	}
	// The control desk never sees it: its schedule and proposals are as
	// without the ghost.
	if _, ok := ghostSlot(st.Schedule(t0), id); ok {
		t.Error("the desk's schedule has the ghost")
	}
	if got := len(st.Proposals(t0)); got != 2 {
		t.Errorf("proposals = %d, want 2", got)
	}

	// While its match plays, the ghost waits for its estimated end; once it
	// ended, from its real end; a match called on the court then ends it.
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 2) })
	if gs, _ := ghostSlot(g.state().ScheduleWithGhosts(t0), id); gs.Court != 1 || !gs.Start.Equal(a1) {
		t.Errorf("ghost after its match playing = %+v, want court 1 at %v", gs, a1)
	}
	g.now = min(20)
	g.do(func(s State) (Change, error) { return s.EndMatch("MS-A1") })
	if gs, _ := ghostSlot(g.state().ScheduleWithGhosts(min(20)), id); !gs.Start.Equal(min(20)) {
		t.Errorf("ghost after its match ended = %+v, want at 20 min", gs)
	}
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A2", 2) })
	if got := g.state().GhostMatches(); len(got) != 1 {
		t.Errorf("a call on another court ended the ghost: %v", got)
	}
	g.now = min(25)
	g.do(func(s State) (Change, error) { return s.EndMatch("MS-A2") })
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A3", 1) })
	if got := g.state().GhostMatches(); len(got) != 0 {
		t.Errorf("ghost still there after a call on its court: %v", got)
	}
}

func TestGhostMatchFollowingNoMatch(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4)
	g.do(func(s State) (Change, error) { return s.SetCourts(2) })
	g.do(func(s State) (Change, error) { return s.AddGhostMatch(2, 45, "") })
	id := g.state().GhostMatches()[0].ID
	gs, ok := ghostSlot(g.state().ScheduleWithGhosts(t0), id)
	if !ok || gs.Court != 2 || !gs.Start.Equal(t0) {
		t.Fatalf("ghost = %+v, want court 2 now", gs)
	}
	g.now = t0.Add(time.Minute)
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 2) })
	if got := g.state().GhostMatches(); len(got) != 0 {
		t.Errorf("ghost still there after a call on its court: %v", got)
	}
}
