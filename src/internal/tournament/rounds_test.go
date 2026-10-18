package tournament

import (
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// timedPool sets up one timed pool of n entrants on the given courts.
func timedPool(t *testing.T, n, courts int) *game {
	g := newGame(t, engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.SetCourts(courts) })
	clubs := make([]string, n)
	for i := range clubs {
		clubs[i] = string(rune('a' + i))
	}
	g.presentSingles(engine.MensSingles, clubs, n)
	g.editStage(engine.MensSingles, "S1", func(st *StageConfig) {
		st.Pools, st.Win = 1, engine.WinCondition{Kind: engine.WinByTime, Minutes: 12}
	})
	g.do(func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })
	g.do(func(s State) (Change, error) { return s.ConfirmDraw(engine.MensSingles) })
	return g
}

func entrantsOf(s State, calls []RoundCall) map[engine.EntrantID]bool {
	out := map[engine.EntrantID]bool{}
	for _, c := range calls {
		m, _ := s.Match(c.Match)
		out[m.A], out[m.B] = true, true
	}
	return out
}

func TestRounds(t *testing.T) {
	g := timedPool(t, 5, 2) // 10 matches, 4 entrants play per round, 1 sits out
	if p := g.state().Proposals(t0); len(p) != 0 {
		t.Errorf("timed matches proposed per court: %v", p)
	}
	g.fails(engine.CodeNoRound, func(s State) (Change, error) { return s.EndRound() })

	g.do(func(s State) (Change, error) { return s.StartRound() })
	r, ok := g.state().CurrentRound()
	if !ok || r.Number != 1 || r.Minutes != 12 || len(r.Calls) != 2 || !r.StartedAt.Equal(t0) {
		t.Fatalf("round = %+v", r)
	}
	if got := entrantsOf(g.state(), r.Calls); len(got) != 4 {
		t.Errorf("an entrant plays twice in round 1: %v", r.Calls)
	}
	for _, c := range g.state().CourtStates() {
		if c.State != CourtPlaying {
			t.Errorf("court %d = %s", c.Number, c.State)
		}
	}
	g.fails(engine.CodeRoundInProgress, func(s State) (Change, error) { return s.StartRound() })

	// Whistle at 10:13: both matches go to pending scores.
	g.now = t0.Add(13 * time.Minute)
	g.do(func(s State) (Change, error) { return s.EndRound() })
	if len(g.state().PendingScores()) != 2 {
		t.Errorf("pending = %v", ids(g.state().PendingScores()))
	}
	if _, ok := g.state().CurrentRound(); ok {
		t.Error("round still in progress")
	}

	// Round 2 starts right away (no minimum rest) and the entrant who sat
	// out round 1 plays.
	all := map[engine.EntrantID]bool{}
	for _, e := range g.state().Entrants {
		all[e.ID] = true
	}
	played := entrantsOf(g.state(), r.Calls)
	var sat engine.EntrantID
	for id := range all {
		if !played[id] {
			sat = id
		}
	}
	g.do(func(s State) (Change, error) { return s.StartRound() })
	r2, _ := g.state().CurrentRound()
	if r2.Number != 2 || !entrantsOf(g.state(), r2.Calls)[sat] {
		t.Errorf("round 2 = %+v, should include %s", r2, sat)
	}

	// Undoing the start uncalls every match of the round at once.
	g.undo()
	if _, ok := g.state().CurrentRound(); ok {
		t.Error("round 2 still in progress after undo")
	}
	for _, c := range r2.Calls {
		if m, _ := g.state().Match(c.Match); m.Called() {
			t.Errorf("%s still called", c.Match)
		}
	}
}

func TestTimedScores(t *testing.T) {
	g := timedPool(t, 4, 2)
	g.do(func(s State) (Change, error) { return s.StartRound() })
	r, _ := g.state().CurrentRound()
	id := r.Calls[0].Match
	g.fails(engine.CodeInvalidTimedScore, func(s State) (Change, error) {
		return s.RecordScore(id, engine.Score{Sets: []engine.SetScore{{A: 15, B: 15}}})
	})
	g.do(func(s State) (Change, error) {
		return s.RecordScore(id, engine.Score{Sets: []engine.SetScore{{A: 15, B: 16}}, GoldenPoint: true})
	})
	// A score entered during the round ends that match early.
	if m, _ := g.state().Match(id); !m.Ended() {
		t.Error("scored match should be ended")
	}
}

func TestStartRoundWithNothingReady(t *testing.T) {
	g := timedPool(t, 2, 2) // a single match
	g.do(func(s State) (Change, error) { return s.StartRound() })
	g.do(func(s State) (Change, error) { return s.EndRound() })
	g.fails(engine.CodeNoMatchReady, func(s State) (Change, error) { return s.StartRound() })
}
