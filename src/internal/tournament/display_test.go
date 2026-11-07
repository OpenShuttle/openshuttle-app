package tournament

import (
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// bracketEntry draws a full-placement bracket of n entrants as entry stage.
func (g *game) bracketEntry(n int) []Match {
	g.t.Helper()
	g.presentSingles(engine.MensSingles, []string{"a", "b", "c", "d", "e", "f", "g", "h"}, n)
	g.editStage(engine.MensSingles, "S1", func(st *StageConfig) {
		st.Kind, st.Win, st.FullPlacement = StageBracket, sets21, true
	})
	g.do(func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })
	g.do(func(s State) (Change, error) { return s.ConfirmDraw(engine.MensSingles) })
	return g.bracketMatches(engine.MensSingles, "S1")
}

func TestBracketNames(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	ms := g.bracketEntry(8)
	// Quarters 0-3, semis 4-5, places 5-8 6-7, final 8, 3rd 9, 5th 10, 7th 11.
	if len(ms) != 12 {
		t.Fatalf("%d matches", len(ms))
	}
	names := g.state().BracketNames()
	letters := make([]string, len(ms))
	for i, m := range ms {
		letters[i] = names.Letter(m.ID)
	}
	want := []string{"A", "B", "C", "D", "A", "B", "A", "B", "", "", "", ""}
	if !slices.Equal(letters, want) {
		t.Errorf("letters = %v, want %v", letters, want)
	}

	type src struct {
		match int
		loser bool
	}
	index := func(s *MatchSource) src {
		if s == nil {
			return src{-1, false}
		}
		return src{slices.IndexFunc(ms, func(m Match) bool { return m.ID == s.Match.ID }), s.Loser}
	}
	tests := []struct {
		match int
		a, b  src
	}{
		{0, src{-1, false}, src{-1, false}}, // a quarter: first round
		{4, src{0, false}, src{1, false}},   // semi A: winners of QF A and B
		{5, src{2, false}, src{3, false}},
		{6, src{0, true}, src{1, true}}, // places 5-8 A: losers of QF A and B
		{7, src{2, true}, src{3, true}},
		{8, src{4, false}, src{5, false}}, // final
		{9, src{4, true}, src{5, true}},   // 3rd place
		{10, src{6, false}, src{7, false}},
		{11, src{6, true}, src{7, true}},
	}
	for _, tt := range tests {
		a, b := names.Sources(ms[tt.match])
		if index(a) != tt.a || index(b) != tt.b {
			t.Errorf("match %d: sources %v %v, want %v %v", tt.match, index(a), index(b), tt.a, tt.b)
		}
	}
}

func TestBracketNamesPassByes(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	ms := g.bracketEntry(6) // a bracket of 8 with 2 byes: the 5th place follows the quarters
	names := g.state().BracketNames()
	passed := false
	for _, m := range ms {
		if m.ByeA || m.ByeB {
			if names.Letter(m.ID) != "" {
				t.Errorf("%s never plays but has a letter", m.ID)
			}
			continue
		}
		a, b := names.Sources(m)
		for _, s := range []*MatchSource{a, b} {
			if s == nil {
				continue
			}
			if s.Match.ByeA || s.Match.ByeB || s.Match.Size <= m.Size {
				t.Errorf("%s fed by %+v", m.ID, s.Match)
			}
			passed = passed || s.Match.Size > 2*m.Size
		}
	}
	if !passed {
		t.Error("no side passed through a bye")
	}
}

func TestStageDisplay(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	main := g.addStage(engine.MensSingles, StageBracket, "Principal", true)
	g.route(engine.MensSingles, "S1", main, main)
	g.fails(engine.CodeStageNotFound, func(s State) (Change, error) { return s.SetStageDisplay(engine.MensSingles, "S9", false) })

	refs := func() []string {
		var out []string
		for _, r := range g.state().DisplayStages() {
			out = append(out, r.Stage)
		}
		return out
	}
	// Drawn, not started: not shown until the desk says so.
	if got := refs(); len(got) != 0 || g.state().ShownOnDisplay(engine.MensSingles, "S1") {
		t.Errorf("stages = %v", got)
	}
	if ch, err := g.state().SetStageDisplay(engine.MensSingles, "S1", false); err != nil || ch.Type != "" {
		t.Errorf("hiding a hidden stage = %+v, %v", ch, err)
	}
	g.do(func(s State) (Change, error) { return s.SetStageDisplay(engine.MensSingles, "S1", true) })
	if got := refs(); !slices.Equal(got, []string{"S1"}) {
		t.Errorf("stages = %v", got)
	}
	g.undo()

	// The first match called shows the stage.
	first := g.state().Matches()[0]
	g.do(func(s State) (Change, error) { return s.CallMatch(first.ID, 1) })
	if got := refs(); !slices.Equal(got, []string{"S1"}) {
		t.Errorf("stages = %v", got)
	}
	// The desk hides it anyway.
	g.do(func(s State) (Change, error) { return s.SetStageDisplay(engine.MensSingles, "S1", false) })
	if got := refs(); len(got) != 0 {
		t.Errorf("hidden stage shown: %v", got)
	}
	g.undo()

	g.finishPools(engine.MensSingles)
	// Pools over, bracket filled but not started: the pools stay on screen.
	g.fill(engine.MensSingles, main)
	if got := refs(); !slices.Equal(got, []string{"S1"}) {
		t.Errorf("stages = %v", got)
	}
	// Shown by hand, the bracket takes the screen.
	g.do(func(s State) (Change, error) { return s.SetStageDisplay(engine.MensSingles, main, true) })
	if got := refs(); !slices.Equal(got, []string{main}) {
		t.Errorf("stages = %v", got)
	}
}
