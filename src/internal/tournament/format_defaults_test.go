package tournament

import (
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestCreationDefaults(t *testing.T) {
	g := &game{t: t, now: t0}
	rest := 5
	g.append(Change{TypeCreated, Created{Name: "Plume", Courts: 7, MinRest: &rest, TimedPools: true}})
	s := g.state()
	if s.Courts() != 7 || s.MinRest() != 5 {
		t.Errorf("courts %d, rest %d", s.Courts(), s.MinRest())
	}

	g.do(func(s State) (Change, error) { return s.AddCategory(engine.MensSingles) })
	if w := g.state().PoolStage(engine.MensSingles).Win; w.Kind != engine.WinByTime || w.Minutes != DefaultTimedMinutes {
		t.Errorf("single category default = %+v", w)
	}
	// A timed default does not block a second category: it gives way to sets.
	g.do(func(s State) (Change, error) { return s.AddCategory(engine.WomensSingles) })
	if w := g.state().PoolStage(engine.MensSingles).Win; w.Kind != engine.WinBySets {
		t.Errorf("two categories default = %+v", w)
	}
}

func TestZeroRestDefault(t *testing.T) {
	g := &game{t: t, now: t0}
	zero := 0
	g.append(Change{TypeCreated, Created{Name: "Plume", MinRest: &zero}})
	if s := g.state(); s.MinRest() != 0 || s.Courts() != DefaultCourts {
		t.Errorf("rest %d courts %d", s.MinRest(), s.Courts())
	}
}

func TestTimedBracketBlocksSecondCategory(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "T", Win: engine.WinCondition{Kind: engine.WinByTime, Minutes: 10}})
	})
	g.fails(engine.CodeTimedNeedsOneCategory, func(s State) (Change, error) { return s.AddCategory(engine.WomensSingles) })
}
