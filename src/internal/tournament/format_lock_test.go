package tournament

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

var sets15 = engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 2, PointsPerSet: 15, Cap: 21}

// poolsPlaying sets up 2 pools of 4 routing their first two to a bracket
// (S2), with a pool match called: the pools started, the bracket did not.
func poolsPlaying(t *testing.T) *game {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	main := g.addStage(engine.MensSingles, StageBracket, "Main", true)
	g.route(engine.MensSingles, "S1", main, main)
	first := g.state().Matches()[0]
	g.do(func(s State) (Change, error) { return s.CallMatch(first.ID, 1) })
	return g
}

// bracketFilled plays the pools of poolsPlaying and fills the bracket.
func bracketFilled(t *testing.T) *game {
	g := poolsPlaying(t)
	g.finishPools(engine.MensSingles)
	g.fill(engine.MensSingles, "S2")
	return g
}

// edited returns the format of the category with one stage changed.
func edited(s State, id string, edit func(*StageConfig)) []StageConfig {
	stages := s.StageConfigs(engine.MensSingles)
	for i := range stages {
		if stages[i].ID == id {
			edit(&stages[i])
		}
	}
	return stages
}

func setStage(id string, edit func(*StageConfig)) func(State) (Change, error) {
	return func(s State) (Change, error) { return s.SetFormat(engine.MensSingles, edited(s, id, edit)) }
}

func TestStartedStageLocked(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(State) (Change, error)
		want engine.Code // "" when allowed
	}{
		{"started: win condition", setStage("S1", func(st *StageConfig) { st.Win = sets15 }), engine.CodeStageStarted},
		{"started: pool count", setStage("S1", func(st *StageConfig) { st.Pools = 1 }), engine.CodeStageStarted},
		{"started: tiebreaks", setStage("S1", func(st *StageConfig) { slices.Reverse(st.Tiebreaks) }), engine.CodeStageStarted},
		{"started: kind", setStage("S1", func(st *StageConfig) {
			st.Kind, st.Pools, st.Tiebreaks, st.Routing = StageBracket, 0, nil, nil
		}), engine.CodeStageStarted},
		{"started: name", setStage("S1", func(st *StageConfig) { st.Name = "Poules" }), ""},
		{"started: estimate", setStage("S1", func(st *StageConfig) { st.Win.Estimate = 30 }), ""},
		{"started: routing to an unfilled stage", setStage("S1", func(st *StageConfig) { st.Routing = RanksRules("S2", "S2", "S2") }), ""},
		{"started: removed", func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, "S1") }, engine.CodeStageStarted},
		{"later: win condition", setStage("S2", func(st *StageConfig) { st.Win = sets15 }), ""},
		{"later: full placement", setStage("S2", func(st *StageConfig) { st.FullPlacement = false }), ""},
		{"later: kind", setStage("S2", func(st *StageConfig) {
			st.Kind, st.Pools, st.Tiebreaks, st.FullPlacement = StagePools, 1, slices.Clone(engine.DefaultTiebreaks), false
		}), ""},
		{"later: removed", func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, "S2") }, ""},
		{"later: added", func(s State) (Change, error) { return s.AddStage(engine.MensSingles, StageBracket) }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := poolsPlaying(t)
			if tt.want != "" {
				g.fails(tt.want, tt.cmd)
				return
			}
			if g.do(tt.cmd).None() {
				t.Fatal("no change")
			}
		})
	}
}

func TestFilledStageChanges(t *testing.T) {
	tests := []struct {
		name    string
		cmd     func(State) (Change, error)
		emptied bool
	}{
		{"win condition", setStage("S2", func(st *StageConfig) { st.Win = sets15 }), false},
		{"name", setStage("S2", func(st *StageConfig) { st.Name = "Finale" }), false},
		{"full placement", setStage("S2", func(st *StageConfig) { st.FullPlacement = false }), false},
		{"kind", setStage("S2", func(st *StageConfig) {
			st.Kind, st.Pools, st.Tiebreaks, st.FullPlacement = StagePools, 1, slices.Clone(engine.DefaultTiebreaks), false
		}), true},
		{"feeding rules", setStage("S1", func(st *StageConfig) { st.Routing = RanksRules("S2", "S2", "S2") }), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := bracketFilled(t)
			before := len(g.bracketMatches(engine.MensSingles, "S2"))
			ch := g.do(tt.cmd)
			if got := ch.Data.(FormatSet).Emptied; (len(got) > 0) != tt.emptied {
				t.Fatalf("emptied = %v, want %v", got, tt.emptied)
			}
			st, _ := g.state().Stage(engine.MensSingles, "S2")
			ms := g.bracketMatches(engine.MensSingles, "S2")
			if tt.emptied {
				if st.Filled() || st.Confirmed || len(ms) != 0 {
					t.Fatalf("stage still filled: %+v, %d matches", st, len(ms))
				}
				// The desk fills it again with the new format.
				g.fill(engine.MensSingles, "S2")
				if len(g.bracketMatches(engine.MensSingles, "S2")) == 0 {
					t.Fatal("refilled stage has no matches")
				}
				// Undoing the fill, then the change, brings the bracket back.
				g.undo()
				g.undo()
				if n := len(g.bracketMatches(engine.MensSingles, "S2")); n != before {
					t.Fatalf("after undo: %d matches, want %d", n, before)
				}
				return
			}
			if !st.Filled() || len(ms) == 0 {
				t.Fatalf("stage emptied: %+v", st)
			}
			if win := g.state().WinCondition(ms[0]); win != st.Win {
				t.Errorf("match plays %+v, want %+v", win, st.Win)
			}
		})
	}
}

func TestRemoveFilledStage(t *testing.T) {
	g := bracketFilled(t)
	g.do(func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, "S2") })
	if ms := g.bracketMatches(engine.MensSingles, "S2"); len(ms) != 0 {
		t.Fatalf("%d matches left", len(ms))
	}
	if r := g.state().StageConfigs(engine.MensSingles)[0].Routing; len(r) != 0 {
		t.Fatalf("routing left: %+v", r)
	}
}

func TestStartedStageKeepsFeed(t *testing.T) {
	g := bracketFilled(t)
	semi := g.bracketMatches(engine.MensSingles, "S2")[0]
	g.do(func(s State) (Change, error) { return s.CallMatch(semi.ID, 2) })
	g.fails(engine.CodeStageStarted, setStage("S2", func(st *StageConfig) { st.Win = sets15 }))
	g.fails(engine.CodeStageStarted, setStage("S1", func(st *StageConfig) { st.Routing = RanksRules("S2", "S2", "S2") }))
	// A later stage can still be added and set.
	g.do(func(s State) (Change, error) { return s.AddStage(engine.MensSingles, StagePools) })
}

func TestEmptiedStageSchedule(t *testing.T) {
	g := bracketFilled(t)
	g.do(func(s State) (Change, error) { return s.ValidateSchedule() })
	g.do(setStage("S2", func(st *StageConfig) {
		st.Kind, st.Pools, st.Tiebreaks, st.FullPlacement = StagePools, 1, slices.Clone(engine.DefaultTiebreaks), false
	}))
	if order := g.state().ScheduleOrder(); slices.ContainsFunc(order, func(id MatchID) bool { return strings.HasPrefix(string(id), "MS-S2-") }) {
		t.Fatalf("emptied stage still scheduled: %v", order)
	}
	g.fill(engine.MensSingles, "S2")
	order := g.state().ScheduleOrder()
	if len(order) != 6 || order[0] != "MS-S2-A1" {
		t.Fatalf("order = %v, want the 6 matches of the new pool", order)
	}
}

// Files written before per-stage locking have FormatSet events with no
// "emptied" key: they replay as before.
func TestFormatSetWithoutEmptied(t *testing.T) {
	g := bracketFilled(t)
	ch := g.do(setStage("S2", func(st *StageConfig) { st.Name = "Finale" }))
	data, _ := json.Marshal(ch.Data)
	if strings.Contains(string(data), "emptied") {
		t.Fatalf("payload = %s", data)
	}
	if st, _ := g.state().Stage(engine.MensSingles, "S2"); !st.Filled() || st.Name != "Finale" {
		t.Fatalf("stage = %+v", st)
	}
}

func TestReshapedStageBlocks(t *testing.T) {
	g := poolsPlaying(t)
	before := g.state().ScheduleBlocks()
	last := before[len(before)-1]
	g.do(func(s State) (Change, error) { return s.MoveScheduledBlock(last, before[0]) })
	g.do(setStage("S2", func(st *StageConfig) {
		st.Kind, st.Pools, st.Tiebreaks, st.FullPlacement = StagePools, 1, slices.Clone(engine.DefaultTiebreaks), false
	}))
	got := g.state().ScheduleBlocks()
	if slices.ContainsFunc(got, func(k string) bool { return strings.HasPrefix(k, "MS/S2/R") }) ||
		!slices.Contains(got, "MS/S2/A/1") {
		t.Fatalf("blocks = %v", got)
	}
}
