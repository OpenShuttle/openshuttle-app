package tournament

import (
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

type rankWant struct {
	rank int
	tied bool
}

func ranksOf(rows []Ranked) []rankWant {
	out := make([]rankWant, len(rows))
	for i, r := range rows {
		out[i] = rankWant{r.Rank, r.Tied}
	}
	return out
}

func TestFinalStandingsPoolsOnly(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 6)
	if _, complete := g.state().FinalStandings(engine.MensSingles); complete {
		t.Fatal("complete before any match")
	}
	g.finishPools(engine.MensSingles)
	rows, complete := g.state().FinalStandings(engine.MensSingles)
	// No bracket: entrants at the same pool position are tied.
	want := []rankWant{{1, true}, {1, true}, {3, true}, {3, true}, {5, true}, {5, true}}
	if !complete || len(rows) != 6 {
		t.Fatalf("rows = %+v, complete %v", rows, complete)
	}
	for i, w := range want {
		if got := ranksOf(rows)[i]; got != w {
			t.Errorf("row %d = %+v, want %+v", i, got, w)
		}
	}
}

func TestFinalStandingsWithBrackets(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 3, 12) // 3 pools of 4
	// Consolation added first: order follows the routing, not creation.
	g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "Consolation", Win: sets21})
	})
	g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "Main", Win: sets21, FullPlacement: true})
	})
	g.do(func(s State) (Change, error) { return s.SetRouting(engine.MensSingles, []string{"T2", "T2", "T1"}) })
	g.finishPools(engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T2") }) // 6 entrants, size 8
	g.do(func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") }) // 3 entrants, size 4

	rows, complete := g.state().FinalStandings(engine.MensSingles)
	if complete || len(rows) != 12 {
		t.Fatalf("before brackets: %d rows, complete %v", len(rows), complete)
	}
	// The 4th of each pool are already ranked: 10th, tied.
	for _, r := range rows[:3] {
		if r.Rank != 10 || !r.Tied || r.Bracket != "" || r.PoolPosition != 4 {
			t.Errorf("eliminated = %+v", r)
		}
	}

	// Play every bracket match, side A wins.
	for range 10 {
		for _, m := range g.state().Matches() {
			if m.Bracket != "" && g.state().Playable(m) && !g.state().Decided(m) {
				g.win(m.ID, engine.SideA)
			}
		}
	}
	rows, complete = g.state().FinalStandings(engine.MensSingles)
	if !complete {
		t.Fatalf("not complete: %+v", rows)
	}
	got := ranksOf(rows)
	main := got[:6]
	for i, r := range main {
		if r.rank != i+1 || r.tied {
			t.Errorf("main place %d = %+v", i, r)
		}
	}
	// Consolation without full placement, 3 entrants: the semi-final has a
	// bye, so a single semi loser takes 9th alone.
	if got[6] != (rankWant{7, false}) || got[7] != (rankWant{8, false}) || got[8] != (rankWant{9, false}) {
		t.Errorf("consolation = %+v", got[6:9])
	}
	for _, r := range got[9:] {
		if r != (rankWant{10, true}) {
			t.Errorf("fourths = %+v", got[9:])
		}
	}
	for _, r := range rows[:6] {
		if r.Bracket != "T2" {
			t.Errorf("top six should come from T2: %+v", r)
		}
	}
}

func TestArchive(t *testing.T) {
	g := newGame(t)
	g.do(func(s State) (Change, error) { return s.Archive() })
	if !g.state().Archived {
		t.Fatal("not archived")
	}
	if ch := g.do(func(s State) (Change, error) { return s.Archive() }); !ch.None() {
		t.Error("archived twice")
	}
	g.undo()
	if g.state().Archived {
		t.Error("undo should reopen")
	}
}
