package tournament

import (
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

var sets21 = engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 30}

func idNumber(id engine.EntrantID) int {
	n, _ := strconv.Atoi(string(id[1:]))
	return n
}

// finishPools scores every pool match: the entrant registered first wins, so
// standings follow registration order without ties.
func (g *game) finishPools(c engine.Category) {
	g.t.Helper()
	for _, m := range g.state().Matches() {
		if m.Category != c || m.IsBracket() || g.state().Decided(m) {
			continue
		}
		score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
		if idNumber(m.B) < idNumber(m.A) {
			score = engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
		}
		g.do(func(s State) (Change, error) { return s.RecordScore(m.ID, score) })
	}
}

func (g *game) bracketMatches(c engine.Category, id string) []Match {
	var out []Match
	for _, m := range g.state().Matches() {
		if m.Category == c && m.Stage == id {
			out = append(out, m)
		}
	}
	return out
}

// win scores a bracket match for the given side (1 set of 21).
func (g *game) win(id MatchID, side engine.Side) {
	g.t.Helper()
	score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 15}}}
	if side == engine.SideB {
		score = engine.Score{Sets: []engine.SetScore{{A: 15, B: 21}}}
	}
	g.do(func(s State) (Change, error) { return s.RecordScore(id, score) })
}

// addStage adds a stage and returns its ID. Brackets play one set of 21.
func (g *game) addStage(c engine.Category, kind StageKind, name string, full bool) string {
	g.t.Helper()
	ch := g.do(func(s State) (Change, error) { return s.AddStage(c, kind) })
	stages := ch.Data.(FormatSet).Stages
	id := stages[len(stages)-1].ID
	g.editStage(c, id, func(st *StageConfig) {
		st.Name, st.FullPlacement = name, full
		if kind == StageBracket {
			st.Win = sets21
		}
	})
	return id
}

// editStage changes one stage of the format.
func (g *game) editStage(c engine.Category, id string, edit func(*StageConfig)) {
	g.t.Helper()
	g.do(func(s State) (Change, error) {
		stages := s.StageConfigs(c)
		for i := range stages {
			if stages[i].ID == id {
				edit(&stages[i])
			}
		}
		return s.SetFormat(c, stages)
	})
}

// route sets where the ranks of a stage go.
func (g *game) route(c engine.Category, from string, routing ...string) {
	g.t.Helper()
	g.editStage(c, from, func(st *StageConfig) { st.Routing = RanksRules(routing...) })
}

func (g *game) fill(c engine.Category, id string) {
	g.t.Helper()
	g.do(func(s State) (Change, error) { return s.FillStage(c, id) })
}

func TestStageFormat(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	main := g.addStage(engine.MensSingles, StageBracket, " Tableau  principal ", true)
	if main != "S2" {
		t.Fatalf("new stage = %s, want S2", main)
	}
	if st, _ := g.state().Stage(engine.MensSingles, main); st.Name != "Tableau principal" || !st.FullPlacement || st.Pools != 0 {
		t.Errorf("stage = %+v", st)
	}
	cons := g.addStage(engine.MensSingles, StageBracket, "Consolante", false)
	g.route(engine.MensSingles, "S1", main, main, cons, "", "")
	if st, _ := g.state().Stage(engine.MensSingles, "S1"); !slices.EqualFunc(st.Routing, RanksRules(main, main, cons), ruleEqual) {
		t.Errorf("routing = %v (trailing kept ranks trimmed)", st.Routing)
	}
	if g.state().Phase(engine.MensSingles, cons) != 2 {
		t.Errorf("phase = %d", g.state().Phase(engine.MensSingles, cons))
	}

	// Routing only points forward, to known stages.
	g.fails(engine.CodeInvalidRouting, func(s State) (Change, error) {
		stages := s.StageConfigs(engine.MensSingles)
		stages[0].Routing = RanksRules("S1")
		return s.SetFormat(engine.MensSingles, stages)
	})
	g.fails(engine.CodeInvalidRouting, func(s State) (Change, error) {
		stages := s.StageConfigs(engine.MensSingles)
		stages[0].Routing = RanksRules("S9")
		return s.SetFormat(engine.MensSingles, stages)
	})
	g.fails(engine.CodeInvalidWinCondition, func(s State) (Change, error) {
		stages := s.StageConfigs(engine.MensSingles)
		stages[1].Win = engine.WinCondition{}
		return s.SetFormat(engine.MensSingles, stages)
	})
	g.fails(engine.CodeInvalidFormat, func(s State) (Change, error) {
		return s.SetFormat(engine.MensSingles, s.StageConfigs(engine.MensSingles)[:2])
	})

	// Removing a stage clears the routing to it; numbers are never reused.
	g.do(func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, cons) })
	if st, _ := g.state().Stage(engine.MensSingles, "S1"); !slices.EqualFunc(st.Routing, RanksRules(main, main), ruleEqual) {
		t.Errorf("routing after removal = %v", st.Routing)
	}
	if id := g.addStage(engine.MensSingles, StagePools, "Poules 2", false); id != "S4" {
		t.Errorf("new id = %s, want S4", id)
	}
	if st, _ := g.state().Stage(engine.MensSingles, "S4"); st.Pools != 2 || len(st.Tiebreaks) == 0 {
		t.Errorf("new pools stage = %+v", st)
	}
	g.do(func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, "S4") })
	g.do(func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, main) })
	g.fails(engine.CodeInvalidFormat, func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, "S1") })
	g.fails(engine.CodeStageNotFound, func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, "S9") })
}

func TestApplyFormatRenumbers(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	preset := []StageConfig{
		{ID: "P1", Kind: StagePools, Pools: 4, Win: sets21, Tiebreaks: engine.DefaultTiebreaks, Routing: RanksRules("P2", "P2", "P3")},
		{ID: "P2", Kind: StageBracket, Name: "Main", Win: sets21, FullPlacement: true},
		{ID: "P3", Kind: StageBracket, Name: "Consolation", Win: sets21},
	}
	g.do(func(s State) (Change, error) { return s.ApplyFormat(engine.MensSingles, preset) })
	stages := g.state().StageConfigs(engine.MensSingles)
	if len(stages) != 3 || stages[0].ID != "S1" || !slices.EqualFunc(stages[0].Routing, RanksRules("S2", "S2", "S3"), ruleEqual) {
		t.Fatalf("applied = %+v", stages)
	}
	g.fails(engine.CodeInvalidRouting, func(s State) (Change, error) {
		return s.ApplyFormat(engine.MensSingles, []StageConfig{{ID: "X", Kind: StagePools, Pools: 1, Win: sets21, Tiebreaks: engine.DefaultTiebreaks, Routing: RanksRules("Y")}})
	})
}

func TestBracketFullPlacement(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	main := g.addStage(engine.MensSingles, StageBracket, "Principal", true)
	g.route(engine.MensSingles, "S1", main, main)
	g.fails(engine.CodePoolsNotComplete, func(s State) (Change, error) { return s.FillStage(engine.MensSingles, main) })

	g.finishPools(engine.MensSingles)
	g.fill(engine.MensSingles, main)

	ms := g.bracketMatches(engine.MensSingles, main)
	if len(ms) != 4 {
		t.Fatalf("%d matches, want 2 semis + final + 3rd place", len(ms))
	}
	semi1, semi2, final, third := ms[0], ms[1], ms[2], ms[3]
	if !g.state().Playable(semi1) || g.state().Playable(final) || final.Start != 1 || third.Start != 3 {
		t.Fatalf("matches = %+v", ms)
	}
	// Pool mates never meet in the semi-finals.
	for _, m := range []Match{semi1, semi2} {
		if poolOfEntrant(g.state(), m.A) == poolOfEntrant(g.state(), m.B) {
			t.Errorf("pool mates in %s", m.ID)
		}
	}

	g.win(semi1.ID, engine.SideA)
	g.win(semi2.ID, engine.SideB)
	ms = g.bracketMatches(engine.MensSingles, main)
	final, third = ms[2], ms[3]
	if final.A != semi1.A || final.B != semi2.B || third.A != semi1.B || third.B != semi2.A {
		t.Fatalf("final %s-%s, third %s-%s", final.A, final.B, third.A, third.B)
	}

	// Once the final started, the semi result can no longer flip.
	g.do(func(s State) (Change, error) { return s.CallMatch(final.ID, 1) })
	g.fails(engine.CodeBracketNextStarted, func(s State) (Change, error) {
		return s.RecordScore(semi1.ID, engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}}})
	})
	g.fails(engine.CodeBracketNextStarted, func(s State) (Change, error) { return s.ClearScore(semi1.ID) })
	// The same winner with another score is fine.
	g.do(func(s State) (Change, error) {
		return s.RecordScore(semi1.ID, engine.Score{Sets: []engine.SetScore{{A: 22, B: 20}}})
	})
	g.fails(engine.CodeBracketStarted, func(s State) (Change, error) { return s.FillStage(engine.MensSingles, main) })
	g.fails(engine.CodeBracketStarted, func(s State) (Change, error) { return s.SwapSlots(engine.MensSingles, main, 0, 1) })
	// A filled stage keeps its kind and its feeding ranks.
	g.fails(engine.CodeStageFilled, func(s State) (Change, error) {
		stages := s.StageConfigs(engine.MensSingles)
		stages[0].Routing = RanksRules(main)
		return s.SetFormat(engine.MensSingles, stages)
	})
	g.fails(engine.CodeStageFilled, func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, main) })

	g.win(final.ID, engine.SideB)
	g.win(third.ID, engine.SideA)
	places := g.state().Places(engine.MensSingles, main)
	want := []Place{{1, 1, semi2.B}, {2, 2, semi1.A}, {3, 3, semi1.B}, {4, 4, semi2.A}}
	if !slices.Equal(places, want) {
		t.Errorf("places = %+v, want %+v", places, want)
	}
}

func poolOfEntrant(s State, id engine.EntrantID) int {
	d, _ := s.Draw(engine.MensSingles)
	for p, pool := range d.Pools {
		if slices.Contains(pool, id) {
			return p
		}
	}
	return -1
}

func TestBracketByesAndTiedPlaces(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 3, 9) // 3 pools of 3
	top := g.addStage(engine.MensSingles, StageBracket, "Top", false)
	g.route(engine.MensSingles, "S1", top)
	g.finishPools(engine.MensSingles)
	g.fill(engine.MensSingles, top)

	st, _ := g.state().Stage(engine.MensSingles, top)
	b := struct{ Slots []engine.EntrantID }{st.Slots()}
	if len(b.Slots) != 4 || b.Slots[1] != "" {
		t.Fatalf("slots = %v (the top seed gets the bye)", b.Slots)
	}
	ms := g.bracketMatches(engine.MensSingles, top)
	// Semi 1 is a bye: never queued, and the top seed is already in the final.
	if !ms[0].ByeB || g.state().Playable(ms[0]) || ms[2].A != b.Slots[0] || ms[2].B != "" {
		t.Fatalf("matches = %+v", ms)
	}
	for _, q := range g.state().Queue(t0) {
		if q.Match.ID == ms[0].ID || q.Match.ID == ms[2].ID {
			t.Errorf("%s should not be queued", q.Match.ID)
		}
	}
	g.win(ms[1].ID, engine.SideA)
	g.win(ms[2].ID, engine.SideA)
	places := g.state().Places(engine.MensSingles, top)
	want := []Place{{1, 1, b.Slots[0]}, {2, 2, b.Slots[2]}, {3, 4, b.Slots[3]}}
	if !slices.Equal(places, want) {
		t.Errorf("places = %+v, want %+v", places, want)
	}
}

func TestBracketForfeitKeepsPoolResults(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	p := g.addStage(engine.MensSingles, StageBracket, "P", false)
	g.route(engine.MensSingles, "S1", p, p)
	g.finishPools(engine.MensSingles)
	g.fill(engine.MensSingles, p)

	before := g.state().PoolStandings(engine.MensSingles, "S1", 0)
	semi := g.bracketMatches(engine.MensSingles, p)[0]
	g.do(func(s State) (Change, error) { return s.WithdrawEntrant(semi.A) })
	after := g.state().PoolStandings(engine.MensSingles, "S1", 0)
	if !slices.Equal(before.Rows, after.Rows) {
		t.Errorf("pool standings changed by a bracket forfeit")
	}
	ms := g.bracketMatches(engine.MensSingles, p)
	if !g.state().Walkover(ms[0]) || ms[2].A != semi.B {
		t.Errorf("opponent should advance: %+v", ms[2])
	}
}

func TestDrawBracketPreconditions(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 3)
	p := g.addStage(engine.MensSingles, StageBracket, "P", false)
	// A perfect three-way tie in the pool.
	for _, m := range g.state().Matches() {
		score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
		if m.ID == "MS-A2" {
			score = engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
		}
		g.do(func(s State) (Change, error) { return s.RecordScore(m.ID, score) })
	}
	g.route(engine.MensSingles, "S1", p)
	g.fails(engine.CodeTieUnresolved, func(s State) (Change, error) { return s.FillStage(engine.MensSingles, p) })
	g.do(func(s State) (Change, error) { return s.DrawLots(engine.MensSingles, "S1", 0, rng()) })
	// Only the pool winner is routed: not enough for a bracket.
	g.fails(engine.CodeNotEnoughEntrants, func(s State) (Change, error) { return s.FillStage(engine.MensSingles, p) })
	g.route(engine.MensSingles, "S1", p, p)
	g.fill(engine.MensSingles, p)
	g.do(func(s State) (Change, error) { return s.SwapSlots(engine.MensSingles, p, 0, 1) })
	g.fails(engine.CodeInvalidSlot, func(s State) (Change, error) { return s.SwapSlots(engine.MensSingles, p, 0, 5) })
}

func TestBracketMatchesFollowFormatOrder(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	main := g.addStage(engine.MensSingles, StageBracket, "Main", false)
	cons := g.addStage(engine.MensSingles, StageBracket, "Consolation", false)
	g.route(engine.MensSingles, "S1", main, main, cons, cons)
	g.finishPools(engine.MensSingles)
	// Filled in reverse order: the main bracket still comes first.
	g.fill(engine.MensSingles, cons)
	g.fill(engine.MensSingles, main)
	var order []string
	for _, m := range g.state().Matches() {
		if m.IsBracket() && (len(order) == 0 || order[len(order)-1] != m.Stage) {
			order = append(order, m.Stage)
		}
	}
	if !slices.Equal(order, []string{main, cons}) {
		t.Errorf("bracket order = %v", order)
	}
}

// playAll decides every playable match of a stage: the entrant registered
// first wins.
func (g *game) playAll(c engine.Category, stage string) {
	g.t.Helper()
	for range 10 {
		for _, m := range g.state().Matches() {
			if m.Category != c || m.Stage != stage || !g.state().Playable(m) || g.state().Decided(m) {
				continue
			}
			side := engine.SideA
			if idNumber(m.B) < idNumber(m.A) {
				side = engine.SideB
			}
			win := g.state().WinCondition(m)
			score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}}}
			if win.SetsToWin == 2 {
				score.Sets = append(score.Sets, engine.SetScore{A: 21, B: 10})
			}
			if side == engine.SideB {
				for i := range score.Sets {
					score.Sets[i].A, score.Sets[i].B = score.Sets[i].B, score.Sets[i].A
				}
			}
			g.do(func(s State) (Change, error) { return s.RecordScore(m.ID, score) })
		}
	}
}

func TestBracketAsEntryStage(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	ids := g.presentSingles(engine.MensSingles, []string{"a", "b", "c", "d", "e"}, 5)
	g.do(func(s State) (Change, error) { return s.SetSeed(ids[4], 1) })
	g.editStage(engine.MensSingles, "S1", func(st *StageConfig) {
		st.Kind, st.Win, st.FullPlacement = StageBracket, sets21, true
	})
	g.do(func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })
	d, _ := g.state().Draw(engine.MensSingles)
	if d.Kind != StageBracket || len(d.Pools) != 1 || len(d.Pools[0]) != 8 || d.Pools[0][0] != ids[4] {
		t.Fatalf("draw = %+v (the top seed takes slot 1)", d)
	}
	g.fails(engine.CodeInvalidPool, func(s State) (Change, error) { return s.MoveEntrant(ids[0], 0) })
	g.do(func(s State) (Change, error) { return s.SwapSlots(engine.MensSingles, "S1", 2, 3) })
	g.do(func(s State) (Change, error) { return s.ConfirmDraw(engine.MensSingles) })
	if n := len(g.bracketMatches(engine.MensSingles, "S1")); n != 12 {
		t.Errorf("%d matches, want a full-placement bracket of 8 (12)", n)
	}
	g.playAll(engine.MensSingles, "S1")
	rows, complete := g.state().FinalStandings(engine.MensSingles)
	if !complete || len(rows) != 5 {
		t.Fatalf("standings = %+v, complete %v", rows, complete)
	}
	for i, r := range rows {
		if r.Rank != i+1 || r.Tied {
			t.Errorf("row %d = %+v", i, r)
		}
	}
}

func TestThreePhases(t *testing.T) {
	// Pools, then pools again for the top two of each pool, then a bracket
	// for the top two of the second pools; the others keep their ranks.
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 4, 16) // 4 pools of 4
	second := g.addStage(engine.MensSingles, StagePools, "Poules 2", false)
	final := g.addStage(engine.MensSingles, StageBracket, "Finale", true)
	g.route(engine.MensSingles, "S1", second, second)
	g.route(engine.MensSingles, second, final, final)
	if g.state().Phase(engine.MensSingles, final) != 3 {
		t.Fatalf("phase = %d", g.state().Phase(engine.MensSingles, final))
	}
	g.fails(engine.CodePoolsNotComplete, func(s State) (Change, error) { return s.FillStage(engine.MensSingles, second) })
	g.finishPools(engine.MensSingles)
	g.fill(engine.MensSingles, second)

	st, _ := g.state().Stage(engine.MensSingles, second)
	if len(st.Groups) != 2 || len(st.Groups[0]) != 4 || len(st.Groups[1]) != 4 {
		t.Fatalf("second pools = %v", st.Groups)
	}
	// Entrants of the same first pool are kept apart.
	for _, pool := range st.Groups {
		seen := map[int]bool{}
		for _, id := range pool {
			p := poolOfEntrant(g.state(), id)
			if seen[p] {
				t.Errorf("two entrants of pool %d together: %v", p, pool)
			}
			seen[p] = true
		}
	}
	if ms := g.bracketMatches(engine.MensSingles, second); len(ms) != 12 || ms[0].ID != "MS-S2-A1" {
		t.Fatalf("second stage matches = %v", ids(ms))
	}

	// While phase 2 plays, the ranks kept in phase 1 are already final.
	rows, _ := g.state().FinalStandings(engine.MensSingles)
	if len(rows) != 16 || rows[0].Rank != 9 || rows[15].Rank != 0 || rows[15].Stage != second {
		t.Fatalf("standings while playing: %+v / %+v", rows[0], rows[15])
	}
	g.playAll(engine.MensSingles, second)
	g.fill(engine.MensSingles, final)
	g.playAll(engine.MensSingles, final)

	rows, complete := g.state().FinalStandings(engine.MensSingles)
	if !complete || len(rows) != 16 {
		t.Fatalf("standings = %+v, complete %v", rows, complete)
	}
	want := []rankWant{{1, false}, {2, false}, {3, false}, {4, false}, {5, true}, {5, true}, {7, true}, {7, true},
		{9, true}, {9, true}, {9, true}, {9, true}, {13, true}, {13, true}, {13, true}, {13, true}}
	if got := ranksOf(rows); !slices.Equal(got, want) {
		t.Errorf("ranks = %v, want %v", got, want)
	}
	if rows[0].Stage != final || rows[4].Stage != second || rows[8].Stage != "S1" || rows[8].StageRank != 3 {
		t.Errorf("stages = %+v / %+v / %+v", rows[0], rows[4], rows[8])
	}
}

func TestPhases(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	main := g.addStage(engine.MensSingles, StageBracket, "Main", false)
	cons := g.addStage(engine.MensSingles, StageBracket, "Cons", false)
	// Nothing routed yet: both come after the pools, never after a bracket.
	for id, want := range map[string]int{"S1": 1, main: 2, cons: 2} {
		if got := g.state().Phase(engine.MensSingles, id); got != want {
			t.Errorf("phase of %s = %d, want %d", id, got, want)
		}
	}
	// Both fed by the pools: both in phase 2.
	g.route(engine.MensSingles, "S1", main, main, cons)
	if got := g.state().Phase(engine.MensSingles, cons); got != 2 {
		t.Errorf("phase of the consolation = %d, want 2", got)
	}
}

func TestBracketIsFinal(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	main := g.addStage(engine.MensSingles, StageBracket, "Main", true)
	g.route(engine.MensSingles, "S1", main, main)

	// A bracket routes nobody on.
	g.fails(engine.CodeBracketIsFinal, func(s State) (Change, error) {
		stages := s.StageConfigs(engine.MensSingles)
		stages[0].Routing = nil
		stages[1].Routing = RanksRules("", "", "S1")
		return s.SetFormat(engine.MensSingles, stages)
	})
	if _, err := ValidateFormat([]StageConfig{
		{ID: "S1", Kind: StageBracket, Win: sets21, Routing: RanksRules("", "S2")},
		{ID: "S2", Kind: StageBracket, Win: sets21},
	}); err == nil || err.(*engine.Error).Code != engine.CodeBracketIsFinal {
		t.Errorf("preset with a bracket routing on: %v", err)
	}

	// With only a bracket left, no stage can be added: nothing would feed it.
	g.editStage(engine.MensSingles, "S1", func(st *StageConfig) { st.Routing = nil })
	g.do(func(s State) (Change, error) { return s.RemoveStage(engine.MensSingles, "S1") })
	g.fails(engine.CodeBracketIsFinal, func(s State) (Change, error) { return s.AddStage(engine.MensSingles, StagePools) })
	g.fails(engine.CodeBracketIsFinal, func(s State) (Change, error) { return s.AddStage(engine.MensSingles, StageBracket) })
}

func TestRoutingRules(t *testing.T) {
	// 22 entrants, pools of 4 or 5 (4 preferred): 5, 5, 4, 4, 4. The top
	// three of each pool and the best fourth go to the main bracket, the
	// other six to the consolation.
	setup := func(rules func(main, cons string) []RouteRule) (*game, string, string) {
		g := newGame(t, engine.MensSingles)
		clubs := make([]string, 22)
		for i := range clubs {
			clubs[i] = fmt.Sprint("club", i)
		}
		g.presentSingles(engine.MensSingles, clubs, 22)
		g.editStage(engine.MensSingles, "S1", func(st *StageConfig) {
			st.Pools, st.PoolSizes, st.CrossPool = 0, []int{4, 5}, engine.CrossPoolSameMatches
		})
		main := g.addStage(engine.MensSingles, StageBracket, "Main", true)
		cons := g.addStage(engine.MensSingles, StageBracket, "Cons", true)
		g.editStage(engine.MensSingles, "S1", func(st *StageConfig) { st.Routing = rules(main, cons) })
		g.do(func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })
		g.do(func(s State) (Change, error) { return s.ConfirmDraw(engine.MensSingles) })
		d, _ := g.state().Draw(engine.MensSingles)
		var sizes []int
		for _, p := range d.Pools {
			sizes = append(sizes, len(p))
		}
		if !slices.Equal(sizes, []int{5, 5, 4, 4, 4}) {
			t.Fatalf("pool sizes = %v", sizes)
		}
		g.finishPools(engine.MensSingles)
		return g, main, cons
	}
	byRank, main, cons := setup(func(main, cons string) []RouteRule {
		return []RouteRule{
			{Kind: RouteRanks, To: main, Ranks: []int{1, 2, 3}},
			{Kind: RouteBestOfRank, To: main, Rank: 4, Count: 1},
			{Kind: RouteRest, To: cons},
		}
	})
	toMain, toCons := byRank.state().Routed(engine.MensSingles, main), byRank.state().Routed(engine.MensSingles, cons)
	if len(toMain) != 16 || len(toCons) != 6 {
		t.Fatalf("main %d, consolation %d", len(toMain), len(toCons))
	}
	fourths := 0
	for _, e := range toMain {
		if e.Rank == 4 {
			fourths++
		}
	}
	if fourths != 1 {
		t.Errorf("%d fourths in the main bracket", fourths)
	}

	// "The 16 best, then the rest" picks the same entrants.
	best, main2, cons2 := setup(func(main, cons string) []RouteRule {
		return []RouteRule{{Kind: RouteBest, To: main, Count: 16}, {Kind: RouteRest, To: cons}}
	})
	ids := func(es []engine.BracketEntrant) []engine.EntrantID {
		var out []engine.EntrantID
		for _, e := range es {
			out = append(out, e.ID)
		}
		slices.Sort(out)
		return out
	}
	if got := best.state().Routed(engine.MensSingles, main2); !slices.Equal(ids(got), ids(toMain)) {
		t.Errorf("best 16 = %v, want %v", ids(got), ids(toMain))
	}
	if got := best.state().Routed(engine.MensSingles, cons2); len(got) != 6 {
		t.Errorf("rest = %d", len(got))
	}

	// Full standings: main bracket places 1-16 come first, then the
	// consolation's 17-22.
	byRank.fill(engine.MensSingles, main)
	byRank.fill(engine.MensSingles, cons)
	byRank.playAll(engine.MensSingles, main)
	byRank.playAll(engine.MensSingles, cons)
	rows, complete := byRank.state().FinalStandings(engine.MensSingles)
	if !complete || len(rows) != 22 || rows[15].Stage != main || rows[16].Stage != cons || rows[21].Rank != 22 {
		t.Fatalf("standings = %+v, complete %v", rows, complete)
	}
}

func TestRuleValidation(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	main := g.addStage(engine.MensSingles, StageBracket, "Main", true)
	bad := [][]RouteRule{
		{{Kind: RouteRanks, To: main}},
		{{Kind: RouteRanks, To: main, Ranks: []int{1, 1}}},
		{{Kind: RouteBestOfRank, To: main, Rank: 2}},
		{{Kind: RouteBest, To: main}},
		{{Kind: "worst", To: main, Count: 1}},
		{{Kind: RouteRest, To: "S1"}},
	}
	for _, rules := range bad {
		g.fails(engine.CodeInvalidRouting, func(s State) (Change, error) {
			stages := s.StageConfigs(engine.MensSingles)
			stages[0].Routing = rules
			return s.SetFormat(engine.MensSingles, stages)
		})
	}
	// Pool sizes: distinct, from 2 to 16.
	for _, sizes := range [][]int{{1}, {4, 4}, {17}} {
		g.fails(engine.CodeInvalidFormat, func(s State) (Change, error) {
			stages := s.StageConfigs(engine.MensSingles)
			stages[0].PoolSizes = sizes
			return s.SetFormat(engine.MensSingles, stages)
		})
	}
	// No split with the sizes allowed: the draw says so.
	g.presentSingles(engine.MensSingles, []string{"a", "b", "c", "d", "e"}, 5)
	g.editStage(engine.MensSingles, "S1", func(st *StageConfig) { st.PoolSizes = []int{4} })
	g.fails(engine.CodeNoPoolSplit, func(s State) (Change, error) { return s.DrawEntry(engine.MensSingles, rng()) })

	if _, err := ValidateFormat([]StageConfig{{ID: "P", Kind: StagePools, Pools: 2, Win: sets21, Tiebreaks: engine.DefaultTiebreaks}}); err != nil {
		t.Errorf("valid preset refused: %v", err)
	}
	if _, err := ValidateFormat(nil); err == nil {
		t.Error("empty format accepted")
	}
}
