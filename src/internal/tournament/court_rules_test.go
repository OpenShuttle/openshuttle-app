package tournament

import (
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

const ms, ws = engine.MensSingles, engine.WomensSingles

// rulesGame is two categories in pools (MS: two pools of 4, WS: one pool
// of 4), MS with a full-placement bracket after its pools, on 4 courts.
func rulesGame(t *testing.T) (*game, string) {
	g := newGame(t, ms, ws)
	g.do(func(s State) (Change, error) { return s.SetCourts(4) })
	g.confirmedPools(ms, 2, 8)
	g.confirmedPools(ws, 1, 4)
	main := g.addStage(ms, StageBracket, "Main", true)
	g.route(ms, "S1", main, main)
	return g, main
}

func rule(kind CourtRuleKind, level int, c engine.Category, stage, pool string, round, from int) CourtRule {
	return CourtRule{Kind: kind, Level: level, Scope: CourtScope{Category: c, Stage: stage, Pool: pool, Round: round, From: from}}
}

func TestAddCourtRule(t *testing.T) {
	g, main := rulesGame(t)
	tests := []struct {
		name   string
		courts []int
		rule   CourtRule
		code   engine.Code
	}{
		{"a category", []int{1, 2}, rule(CourtPriority, 1, ws, "", "", 0, 0), ""},
		{"a pool", []int{3}, rule(CourtPriority, 2, ms, "S1", "B", 0, 0), ""},
		{"the final", []int{4}, rule(CourtOnly, 0, ms, main, "", 2, 1), ""},
		{"a bracket round defaults to the main draw", []int{4}, rule(CourtNever, 0, ms, main, "", 4, 0), ""},
		{"no court", nil, rule(CourtOnly, 0, ms, "", "", 0, 0), engine.CodeInvalidCourt},
		{"a court out of range", []int{5}, rule(CourtOnly, 0, ms, "", "", 0, 0), engine.CodeInvalidCourt},
		{"an unknown kind", []int{1}, rule("sometimes", 0, ms, "", "", 0, 0), engine.CodeInvalidSetting},
		{"a level out of range", []int{1}, rule(CourtPriority, 10, ms, "", "", 0, 0), engine.CodeInvalidSetting},
		{"a category not in the tournament", []int{1}, rule(CourtOnly, 0, engine.MixedDoubles, "", "", 0, 0), engine.CodeCategoryNotFound},
		{"an unknown stage", []int{1}, rule(CourtOnly, 0, ms, "S9", "", 0, 0), engine.CodeStageNotFound},
		{"a pool of a bracket", []int{1}, rule(CourtOnly, 0, ms, main, "A", 0, 0), engine.CodeInvalidSetting},
		{"a round of pools", []int{1}, rule(CourtOnly, 0, ms, "S1", "", 2, 1), engine.CodeInvalidSetting},
		{"a round of 3", []int{1}, rule(CourtOnly, 0, ms, main, "", 3, 1), engine.CodeInvalidSetting},
		{"a pool without stage", []int{1}, rule(CourtOnly, 0, ms, "", "A", 0, 0), engine.CodeInvalidSetting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g.t = t
			if tt.code != "" {
				g.fails(tt.code, func(s State) (Change, error) { return s.AddCourtRule(tt.courts, tt.rule) })
				return
			}
			g.do(func(s State) (Change, error) { return s.AddCourtRule(tt.courts, tt.rule) })
		})
	}
	g.t = t
	got := g.state().CourtRules()
	want := map[int][]CourtRule{
		1: {rule(CourtPriority, 1, ws, "", "", 0, 0)},
		2: {rule(CourtPriority, 1, ws, "", "", 0, 0)},
		3: {rule(CourtPriority, 2, ms, "S1", "B", 0, 0)},
		4: {rule(CourtOnly, 0, ms, main, "", 2, 1), rule(CourtNever, 0, ms, main, "", 4, 1)},
	}
	for court, rs := range want {
		if !slices.Equal(got[court], rs) {
			t.Errorf("court %d rules = %+v, want %+v", court, got[court], rs)
		}
	}

	// One rule per scope: adding it again replaces it; the rules sort only,
	// priorities by level, never.
	g.do(func(s State) (Change, error) {
		return s.AddCourtRule([]int{4}, rule(CourtPriority, 3, ms, main, "", 2, 1))
	})
	if got := g.state().CourtRules()[4]; len(got) != 2 || got[0].Kind != CourtPriority || got[0].Level != 3 || got[1].Kind != CourtNever {
		t.Errorf("replaced = %+v", got)
	}
	g.do(func(s State) (Change, error) {
		return s.RemoveCourtRule(4, CourtScope{Category: ms, Stage: main, Round: 4, From: 1})
	})
	g.do(func(s State) (Change, error) { return s.ClearCourtRules(1) })
	if got := g.state().CourtRules(); len(got[4]) != 1 || len(got[1]) != 0 || len(got[2]) != 1 {
		t.Errorf("after removing = %+v", got)
	}
	g.undo()
	if got := g.state().CourtRules(); len(got[1]) != 1 {
		t.Errorf("undo keeps the rules of court 1 = %+v", got)
	}
	// Nothing changes: no event.
	if ch := g.do(func(s State) (Change, error) { return s.RemoveCourtRule(3, CourtScope{Category: ws}) }); !ch.None() {
		t.Errorf("removing a missing rule = %+v", ch)
	}

}

func TestCourtRulesOfRemovedStage(t *testing.T) {
	g := newGame(t, ms)
	main := g.addStage(ms, StageBracket, "Main", true)
	g.route(ms, "S1", main, main)
	g.do(func(s State) (Change, error) {
		return s.AddCourtRule([]int{2}, rule(CourtNever, 0, ms, main, "", 0, 0))
	})
	g.do(func(s State) (Change, error) { return s.AddCourtRule([]int{2}, rule(CourtOnly, 0, ms, "S1", "", 0, 0)) })
	g.do(func(s State) (Change, error) { return s.RemoveStage(ms, main) })
	if got := g.state().CourtRules(); len(got[2]) != 1 || got[2][0].Kind != CourtOnly {
		t.Errorf("rules of a removed stage = %+v", got)
	}
}

// firstWave gives the category each court starts with.
func firstWave(sc Schedule) map[int]engine.Category {
	out := map[int]engine.Category{}
	for _, s := range startsAt(sc, t0) {
		out[s.Court] = s.Category
	}
	return out
}

func TestCourtRulesForecast(t *testing.T) {
	tests := []struct {
		name  string
		rules map[int][]CourtRule
		want  map[int]engine.Category
	}{
		// Alternate: round 1 of MS (two pools, four matches) fills the courts.
		{"without rules, the order", nil, map[int]engine.Category{1: ms, 2: ms, 3: ms, 4: ms}},
		{
			"WS first on courts 1 and 2",
			map[int][]CourtRule{1: {rule(CourtPriority, 1, ws, "", "", 0, 0)}, 2: {rule(CourtPriority, 1, ws, "", "", 0, 0)}},
			map[int]engine.Category{1: ws, 2: ws, 3: ms, 4: ms},
		},
		{
			"pools A and B of MS first on courts 3 and 4, same level",
			map[int][]CourtRule{
				1: {rule(CourtPriority, 1, ws, "", "", 0, 0)}, 2: {rule(CourtPriority, 1, ws, "", "", 0, 0)},
				3: {rule(CourtPriority, 1, ms, "S1", "A", 0, 0), rule(CourtPriority, 1, ms, "S1", "B", 0, 0)},
				4: {rule(CourtPriority, 1, ms, "S1", "A", 0, 0), rule(CourtPriority, 1, ms, "S1", "B", 0, 0)},
			},
			map[int]engine.Category{1: ws, 2: ws, 3: ms, 4: ms},
		},
		{
			"court 4 never MS, court 3 only WS",
			map[int][]CourtRule{4: {rule(CourtNever, 0, ms, "", "", 0, 0)}, 3: {rule(CourtOnly, 0, ws, "", "", 0, 0)}},
			map[int]engine.Category{1: ms, 2: ms, 3: ws, 4: ws},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, _ := rulesGame(t)
			for court, rs := range tt.rules {
				for _, r := range rs {
					g.do(func(s State) (Change, error) { return s.AddCourtRule([]int{court}, r) })
				}
			}
			st := g.state()
			if got := firstWave(st.Schedule(t0)); !mapsEqual(got, tt.want) {
				t.Errorf("first wave = %v, want %v", got, tt.want)
			}
			// The desk is proposed the same.
			props := map[int]engine.Category{}
			for court, p := range st.Proposals(t0) {
				props[court] = p.Category
			}
			if !mapsEqual(props, tt.want) {
				t.Errorf("proposals = %v, want %v", props, tt.want)
			}
		})
	}
}

func mapsEqual(a, b map[int]engine.Category) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestCourtRulesFinalOnItsCourt(t *testing.T) {
	g, main := rulesGame(t)
	g.do(func(s State) (Change, error) { return s.AddCourtRule([]int{4}, rule(CourtOnly, 0, ms, main, "", 2, 1)) })
	sc := g.state().Schedule(t0)
	var final *ScheduleSlot
	for i, s := range sc.Slots {
		if s.Court == 4 && (s.Stage != main || s.RoundSize != 2 || s.PlaceFrom != 1) {
			t.Errorf("court 4 plays %+v", s)
		}
		if s.Stage == main && s.RoundSize == 2 && s.PlaceFrom == 1 {
			final = &sc.Slots[i]
		}
	}
	if final == nil || final.Court != 4 {
		t.Errorf("final = %+v", final)
	}
}

func TestCourtRulesEnd(t *testing.T) {
	g, _ := rulesGame(t)
	wsScope := CourtScope{Category: ws}
	g.do(func(s State) (Change, error) {
		return s.AddCourtRule([]int{1}, CourtRule{Kind: CourtOnly, Scope: wsScope})
	})
	m := g.state().Matches()[0]
	if m.Category != ms {
		t.Fatalf("first match %+v", m)
	}
	st := g.state()
	if gate := st.CourtGate(); !st.CourtScopeLeft(wsScope) || gate(1, m) || !gate(2, m) {
		t.Errorf("court 1 only WS while WS plays")
	}
	g.playAll(ws, "S1")
	st = g.state()
	if st.CourtScopeLeft(wsScope) || !st.CourtGate()(1, m) {
		t.Errorf("court 1 free once WS is over")
	}
	for _, s := range startsAt(st.Schedule(g.now), g.now) {
		if s.Category != ms {
			t.Errorf("slot %+v", s)
		}
	}
	if len(startsAt(st.Schedule(g.now), g.now)) != 4 {
		t.Errorf("court 1 takes MS once WS is over: %v", firstWave(st.Schedule(g.now)))
	}
}

func TestCourtRulesOlderFiles(t *testing.T) {
	g, _ := rulesGame(t)
	// Reserved mode, court 4 reserved for MS by hand: an only rule.
	g.append(Change{TypeScheduleSet, ScheduleSet{Mode: ScheduleReserved, Courts: map[int]engine.Category{4: ms}}})
	want := []CourtRule{{Kind: CourtOnly, Scope: CourtScope{Category: ms}}}
	if got := g.state().CourtRules(); len(got) != 1 || !slices.Equal(got[4], want) {
		t.Errorf("reserved court = %+v", got)
	}
	if r := g.state().Schedule(t0).Reserved; r[4] != "" || len(r) != 3 {
		t.Errorf("shared courts = %v", r)
	}
	// Alternate mode ignored the reserved courts: no rule.
	g.append(Change{TypeScheduleSet, ScheduleSet{Mode: ScheduleAlternate, Courts: map[int]engine.Category{4: ms}}})
	if got := g.state().CourtRules(); len(got) != 0 {
		t.Errorf("alternate = %+v", got)
	}
	// A change writes rules, never courts.
	g.append(Change{TypeScheduleSet, ScheduleSet{Mode: ScheduleReserved, Courts: map[int]engine.Category{4: ms}}})
	ch := g.do(func(s State) (Change, error) { return s.AddCourtRule([]int{1}, rule(CourtNever, 0, ws, "", "", 0, 0)) })
	if p := ch.Data.(ScheduleSet); p.Courts != nil || len(p.Rules) != 2 {
		t.Errorf("written = %+v", p)
	}
}

func TestCourtScopes(t *testing.T) {
	g, main := rulesGame(t)
	got := g.state().CourtScopes()
	want := []CourtScope{
		{Category: ms}, {Category: ms, Stage: "S1"}, {Category: ms, Stage: "S1", Pool: "A"}, {Category: ms, Stage: "S1", Pool: "B"},
		{Category: ms, Stage: main},
		{Category: ms, Stage: main, Round: 4, From: 1}, {Category: ms, Stage: main, Round: 2, From: 1},
		{Category: ms, Stage: main, Round: 2, From: 3},
		{Category: ws}, {Category: ws, Stage: "S1"}, {Category: ws, Stage: "S1", Pool: "A"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("scopes =\n%+v\nwant\n%+v", got, want)
	}
}
