package tournament

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func rng() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

// presentSingles registers n singles of the given clubs (cycled) and checks
// them all in.
func (g *game) presentSingles(c engine.Category, clubs []string, n int) []engine.EntrantID {
	g.t.Helper()
	var ids []engine.EntrantID
	for i := range n {
		ch := g.do(func(s State) (Change, error) {
			return s.RegisterEntrant(c, []PlayerInput{{Name: "P " + string(rune('A'+i)), Club: clubs[i%len(clubs)]}})
		})
		id := ch.Data.(EntrantRegistered).ID
		ids = append(ids, id)
		pid := ch.Data.(EntrantRegistered).Players[0].ID
		g.do(func(s State) (Change, error) { return s.CheckInPlayer(pid) })
	}
	return ids
}

func TestConfigurePoolsStage(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	if st := g.state().StageConfigs(engine.MensSingles)[0]; st.Kind != StagePools || st.Pools != 2 || st.Win.PointsPerSet != 21 {
		t.Fatalf("default stage = %+v", st)
	}
	set := func(edit func(*StageConfig)) func(State) (Change, error) {
		return func(s State) (Change, error) {
			stages := s.StageConfigs(engine.MensSingles)
			edit(&stages[0])
			return s.SetFormat(engine.MensSingles, stages)
		}
	}
	timed := engine.WinCondition{Kind: engine.WinByTime, Minutes: 12, SetsToWin: 2}
	g.do(set(func(st *StageConfig) { st.Pools, st.Win = 3, timed }))
	st := g.state().StageConfigs(engine.MensSingles)[0]
	if st.Pools != 3 || st.Win != (engine.WinCondition{Kind: engine.WinByTime, Minutes: 12}) {
		t.Errorf("stage = %+v", st)
	}
	// Unchanged: nothing appended.
	if ch := g.do(set(func(st *StageConfig) {})); !ch.None() {
		t.Errorf("unchanged stage produced %+v", ch)
	}
	// A timed stage forbids a second category.
	g.fails(engine.CodeTimedNeedsOneCategory, func(s State) (Change, error) { return s.AddCategory(engine.WomensSingles) })

	bad := []func(*StageConfig){
		func(st *StageConfig) { st.Pools = 0 },
		func(st *StageConfig) { st.Pools = MaxPools + 1 },
		func(st *StageConfig) { st.Tiebreaks = nil },
		func(st *StageConfig) { st.Tiebreaks = []engine.Tiebreak{"wins", "wins"} },
		func(st *StageConfig) { st.Tiebreaks = []engine.Tiebreak{"elo"} },
		func(st *StageConfig) { st.Kind = "swiss" },
	}
	for _, b := range bad {
		g.fails(engine.CodeInvalidFormat, set(b))
	}
	g.fails(engine.CodeInvalidWinCondition, set(func(st *StageConfig) { st.Win = engine.WinCondition{Kind: engine.WinByTime} }))
}

func TestTimedRefusedWithTwoCategories(t *testing.T) {
	g := newGame(t, engine.MensSingles, engine.WomensSingles)
	g.fails(engine.CodeTimedNeedsOneCategory, func(s State) (Change, error) {
		stages := s.StageConfigs(engine.MensSingles)
		stages[0].Win = engine.WinCondition{Kind: engine.WinByTime, Minutes: 10}
		return s.SetFormat(engine.MensSingles, stages)
	})
}

func TestSeeds(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	ids := g.presentSingles(engine.MensSingles, []string{"A"}, 3)
	g.do(func(s State) (Change, error) { return s.SetSeed(ids[0], 1) })
	g.do(func(s State) (Change, error) { return s.SetSeed(ids[1], 1) }) // takes seed 1 from ids[0]
	if a, b := g.entrant(ids[0]).Seed, g.entrant(ids[1]).Seed; a != 0 || b != 1 {
		t.Errorf("seeds = %d, %d; want 0, 1", a, b)
	}
	g.undo()
	if a, b := g.entrant(ids[0]).Seed, g.entrant(ids[1]).Seed; a != 1 || b != 0 {
		t.Errorf("after undo seeds = %d, %d; want 1, 0", a, b)
	}
	g.fails(engine.CodeInvalidSeed, func(s State) (Change, error) { return s.SetSeed(ids[2], MaxSeed+1) })
	g.fails(engine.CodeEntrantNotFound, func(s State) (Change, error) { return s.SetSeed("e999", 1) })
}

func TestDrawLifecycle(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.fails(engine.CodeNotEnoughEntrants, func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })

	ids := g.presentSingles(engine.MensSingles, []string{"Lyon", "Paris", "Nice"}, 7)
	// One registered but absent entrant is not drawn.
	g.do(func(s State) (Change, error) {
		return s.RegisterEntrant(engine.MensSingles, []PlayerInput{{Name: "Late Comer", Club: "Lyon"}})
	})
	late := g.state().Entrants[len(g.state().Entrants)-1]

	g.do(func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })
	d, ok := g.state().Draw(engine.MensSingles)
	if !ok || len(d.Pools) != 2 || len(d.Pools[0]) != 4 || len(d.Pools[1]) != 3 || d.Confirmed {
		t.Fatalf("draw = %+v", d)
	}

	// The late entrant checks in: reported as unplaced, then placed by hand.
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(late.Players[0].ID) })
	if check := g.state().CheckDraw(engine.MensSingles); !slices.Equal(check.Unplaced, []engine.EntrantID{late.ID}) {
		t.Errorf("unplaced = %v", check.Unplaced)
	}
	g.do(func(s State) (Change, error) { return s.MoveEntrant(late.ID, 1) })
	d, _ = g.state().Draw(engine.MensSingles)
	if len(d.Pools[1]) != 4 || d.Pools[1][3] != late.ID {
		t.Errorf("after placing late entrant: %v", d.Pools)
	}

	// Move then undo.
	first := d.Pools[0][0]
	g.do(func(s State) (Change, error) { return s.MoveEntrant(first, 1) })
	if check := g.state().CheckDraw(engine.MensSingles); !check.Pools[0].Unbalanced || !check.Pools[1].Unbalanced {
		t.Errorf("3 vs 5 should be unbalanced: %+v", check.Pools)
	}
	g.undo()
	d, _ = g.state().Draw(engine.MensSingles)
	if d.Pools[0][0] != first {
		t.Errorf("undo move: %v", d.Pools)
	}
	g.fails(engine.CodeInvalidPool, func(s State) (Change, error) { return s.MoveEntrant(first, 2) })

	// A cancelled check-in is reported; a removed entrant leaves the draw.
	g.do(func(s State) (Change, error) { return s.CancelCheckIn(g.entrant(ids[0]).Players[0].ID) })
	if check := g.state().CheckDraw(engine.MensSingles); !slices.Equal(check.Absent, []engine.EntrantID{ids[0]}) {
		t.Errorf("absent = %v", check.Absent)
	}
	g.do(func(s State) (Change, error) { return s.RemoveEntrant(ids[0]) })
	d, _ = g.state().Draw(engine.MensSingles)
	for _, pool := range d.Pools {
		if slices.Contains(pool, ids[0]) {
			t.Errorf("removed entrant still drawn: %v", d.Pools)
		}
	}

	// Confirm locks the draw and the number of pools.
	g.do(func(s State) (Change, error) { return s.ConfirmDraw(engine.MensSingles) })
	g.fails(engine.CodeDrawConfirmed, func(s State) (Change, error) { return s.MoveEntrant(first, 1) })
	g.fails(engine.CodeDrawConfirmed, func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })
	three := func(s State) (Change, error) {
		stages := s.StageConfigs(engine.MensSingles)
		stages[0].Pools = 3
		return s.SetFormat(engine.MensSingles, stages)
	}
	g.fails(engine.CodeDrawConfirmed, three)

	// Reopen, then a new number of pools discards the draft.
	g.do(func(s State) (Change, error) { return s.ReopenDraw(engine.MensSingles) })
	g.do(three)
	if _, ok := g.state().Draw(engine.MensSingles); ok {
		t.Error("draft draw kept after changing the number of pools")
	}
	g.fails(engine.CodeNoDraw, func(s State) (Change, error) { return s.ConfirmDraw(engine.MensSingles) })
}

func TestCheckDrawSameClubAndSeeds(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	ids := g.presentSingles(engine.MensSingles, []string{"Lyon"}, 4)
	g.do(func(s State) (Change, error) { return s.SetSeed(ids[0], 1) })
	g.do(func(s State) (Change, error) { return s.SetSeed(ids[1], 2) })
	g.do(func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })
	d, _ := g.state().Draw(engine.MensSingles)
	// Bring both seeds into pool A.
	if !slices.Contains(d.Pools[0], ids[1]) {
		g.do(func(s State) (Change, error) { return s.MoveEntrant(ids[1], 0) })
	}
	check := g.state().CheckDraw(engine.MensSingles)
	if check.Pools[0].Seeds != 2 {
		t.Errorf("seeds in A = %d", check.Pools[0].Seeds)
	}
	if len(check.Pools[0].SameClub[ids[0]]) == 0 {
		t.Errorf("same club not reported: %+v", check.Pools[0])
	}
}
