package tournament

import (
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestClearScore(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4)

	// Played on a court: clearing sends it back to pending scores.
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 1) })
	g.do(func(s State) (Change, error) { return s.RecordScore("MS-A1", straight) })
	g.do(func(s State) (Change, error) { return s.ClearScore("MS-A1") })
	if got := ids(g.state().PendingScores()); !slices.Equal(got, []MatchID{"MS-A1"}) {
		t.Errorf("pending = %v", got)
	}
	g.fails(engine.CodeNoScore, func(s State) (Change, error) { return s.ClearScore("MS-A1") })

	// Scored without a court: clearing sends it back to the queue.
	g.do(func(s State) (Change, error) { return s.RecordScore("MS-A2", straight) })
	g.do(func(s State) (Change, error) { return s.ClearScore("MS-A2") })
	if !slices.ContainsFunc(g.state().Queue(t0), func(q QueueItem) bool { return q.Match.ID == "MS-A2" }) {
		t.Error("MS-A2 should be queued again")
	}
}

func TestReinstateEntrant(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4)
	m, _ := g.state().Match("MS-A1")
	g.fails(engine.CodeEntrantNotWithdrawn, func(s State) (Change, error) { return s.ReinstateEntrant(m.A) })
	g.do(func(s State) (Change, error) { return s.WithdrawEntrant(m.A) })
	g.do(func(s State) (Change, error) { return s.ReinstateEntrant(m.A) })
	if g.state().Withdrawn(m.A) || g.state().Walkover(m) {
		t.Error("still withdrawn")
	}
}

func TestSwapMatches(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4)
	g.do(func(s State) (Change, error) { return s.SwapMatches("MS-A1", "MS-A5") })
	if got := ids(g.state().Matches())[:2]; got[0] != "MS-A5" {
		t.Errorf("order = %v", got)
	}
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A5", 1) })
	g.fails(engine.CodeMatchStarted, func(s State) (Change, error) { return s.SwapMatches("MS-A5", "MS-A2") })
	g.fails(engine.CodeMatchNotFound, func(s State) (Change, error) { return s.SwapMatches("MS-A2", "MS-Z1") })
}

func TestDrawLots(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 3)
	// A circle of wins with identical scores: a perfect three-way tie.
	for _, m := range g.state().Matches() {
		score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
		if m.ID == "MS-A2" {
			score = engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
		}
		g.do(func(s State) (Change, error) { return s.RecordScore(m.ID, score) })
	}
	rows := g.state().PoolStandings(engine.MensSingles, "S1", 0).Rows
	if !rows[0].DrawNeeded || rows[0].Rank != rows[2].Rank {
		t.Fatalf("expected a full tie: %+v", rows)
	}
	g.do(func(s State) (Change, error) { return s.DrawLots(engine.MensSingles, "S1", 0, rng()) })
	rows = g.state().PoolStandings(engine.MensSingles, "S1", 0).Rows
	for i, r := range rows {
		if r.DrawNeeded || r.Rank != i+1 {
			t.Errorf("after lots row %d = %+v", i, r)
		}
	}
	g.fails(engine.CodeNoTie, func(s State) (Change, error) { return s.DrawLots(engine.MensSingles, "S1", 0, rng()) })
}
