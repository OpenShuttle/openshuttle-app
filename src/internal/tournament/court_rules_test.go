package tournament

import (
	"maps"
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

func TestAddCourtRules(t *testing.T) {
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
				g.fails(tt.code, func(s State) (Change, error) { return s.AddCourtRules(tt.courts, []CourtRule{tt.rule}) })
				return
			}
			g.do(func(s State) (Change, error) { return s.AddCourtRules(tt.courts, []CourtRule{tt.rule}) })
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
		return s.AddCourtRules([]int{4}, []CourtRule{rule(CourtPriority, 3, ms, main, "", 2, 1)})
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

func TestAddCourtRulesSeveralScopes(t *testing.T) {
	g, main := rulesGame(t)
	pool := func(name string, level int) CourtRule { return rule(CourtPriority, level, ms, "S1", name, 0, 0) }
	tests := []struct {
		name   string
		courts []int
		rules  []CourtRule
		code   engine.Code
		want   map[int][]CourtRule
	}{
		{
			name: "one rule per scope on each court", courts: []int{1, 2}, rules: []CourtRule{pool("A", 1), pool("B", 1)},
			want: map[int][]CourtRule{1: {pool("A", 1), pool("B", 1)}, 2: {pool("A", 1), pool("B", 1)}},
		},
		{
			name: "a scope already ruled is updated, the same scope twice kept once (the last)", courts: []int{2},
			rules: []CourtRule{pool("B", 3), pool("B", 2), rule(CourtNever, 0, ms, main, "", 0, 0)},
			want:  map[int][]CourtRule{1: {pool("A", 1), pool("B", 1)}, 2: {pool("A", 1), pool("B", 2), rule(CourtNever, 0, ms, main, "", 0, 0)}},
		},
		{name: "no rule", courts: []int{1}, code: engine.CodeInvalidSetting},
		{name: "one invalid rule refuses them all", courts: []int{3}, rules: []CourtRule{pool("A", 1), rule(CourtOnly, 0, ms, main, "A", 0, 0)}, code: engine.CodeInvalidSetting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g.t = t
			if tt.code != "" {
				g.fails(tt.code, func(s State) (Change, error) { return s.AddCourtRules(tt.courts, tt.rules) })
				return
			}
			before := len(g.log)
			g.do(func(s State) (Change, error) { return s.AddCourtRules(tt.courts, tt.rules) })
			if len(g.log) != before+1 {
				t.Errorf("%d events, want one", len(g.log)-before)
			}
			if got := g.state().CourtRules(); !maps.EqualFunc(got, tt.want, slices.Equal) {
				t.Errorf("rules = %+v, want %+v", got, tt.want)
			}
		})
	}
	g.t = t
	// One undo takes them all back.
	g.undo()
	if got := g.state().CourtRules()[2]; len(got) != 2 {
		t.Errorf("after undo = %+v", got)
	}
}

func TestCourtRulesOfRemovedStage(t *testing.T) {
	g := newGame(t, ms)
	main := g.addStage(ms, StageBracket, "Main", true)
	g.route(ms, "S1", main, main)
	g.do(func(s State) (Change, error) {
		return s.AddCourtRules([]int{2}, []CourtRule{rule(CourtNever, 0, ms, main, "", 0, 0)})
	})
	g.do(func(s State) (Change, error) {
		return s.AddCourtRules([]int{2}, []CourtRule{rule(CourtOnly, 0, ms, "S1", "", 0, 0)})
	})
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
					g.do(func(s State) (Change, error) { return s.AddCourtRules([]int{court}, []CourtRule{r}) })
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
	g.do(func(s State) (Change, error) {
		return s.AddCourtRules([]int{4}, []CourtRule{rule(CourtOnly, 0, ms, main, "", 2, 1)})
	})
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
		return s.AddCourtRules([]int{1}, []CourtRule{CourtRule{Kind: CourtOnly, Scope: wsScope}})
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
	plain := firstWave(g.state().Schedule(t0))
	// Reserved mode is gone: a file in it alternates the categories.
	g.append(Change{TypeScheduleSet, ScheduleSet{Mode: legacyReserved}})
	if got := firstWave(g.state().Schedule(t0)); !mapsEqual(got, plain) {
		t.Errorf("reserved mode = %v, want %v", got, plain)
	}
	// Court 4 reserved for MS by hand: an only rule.
	g.append(Change{TypeScheduleSet, ScheduleSet{Mode: legacyReserved, Courts: map[int]engine.Category{4: ms}}})
	want := []CourtRule{{Kind: CourtOnly, Scope: CourtScope{Category: ms}}}
	if got := g.state().CourtRules(); len(got) != 1 || !slices.Equal(got[4], want) {
		t.Errorf("reserved court = %+v", got)
	}
	// Alternate mode ignored the reserved courts: no rule.
	g.append(Change{TypeScheduleSet, ScheduleSet{Mode: "alternate", Courts: map[int]engine.Category{4: ms}}})
	if got := g.state().CourtRules(); len(got) != 0 {
		t.Errorf("alternate = %+v", got)
	}
	// A change writes rules, never courts nor a mode.
	g.append(Change{TypeScheduleSet, ScheduleSet{Mode: legacyReserved, Courts: map[int]engine.Category{4: ms}}})
	ch := g.do(func(s State) (Change, error) {
		return s.AddCourtRules([]int{1}, []CourtRule{rule(CourtNever, 0, ws, "", "", 0, 0)})
	})
	if p := ch.Data.(ScheduleSet); p.Courts != nil || p.Mode != "" || len(p.Rules) != 2 {
		t.Errorf("written = %+v", p)
	}
}

// TestCourtRulesFirstWave is a desk's setup: seven courts, MS in five pools,
// WS in two; WS first on courts 1 and 2, MS pools A and B on courts 3 to 5,
// D and E on courts 6 and 7. A file from the reserved mode follows them too.
func TestCourtRulesFirstWave(t *testing.T) {
	for _, mode := range []string{"", legacyReserved} {
		t.Run("mode "+mode, func(t *testing.T) {
			g := newGame(t, ms, ws)
			g.do(func(s State) (Change, error) { return s.SetCourts(7) })
			g.confirmedPools(ms, 5, 25)
			g.confirmedPools(ws, 2, 10)
			if mode != "" {
				g.append(Change{TypeScheduleSet, ScheduleSet{Mode: mode}})
			}
			priority := func(courts []int, c engine.Category, pools ...string) {
				for _, p := range pools {
					g.do(func(s State) (Change, error) {
						return s.AddCourtRules(courts, []CourtRule{rule(CourtPriority, 1, c, "S1", p, 0, 0)})
					})
				}
			}
			priority([]int{1, 2}, ws, "")
			priority([]int{3, 4, 5}, ms, "A", "B")
			priority([]int{6, 7}, ms, "D", "E")
			pools := map[int]string{}
			for _, sl := range startsAt(g.state().Schedule(t0), t0) {
				pools[sl.Court] = string(sl.Category) + " " + engine.PoolName(sl.Pool)
			}
			want := map[int]string{1: "WS A", 2: "WS A", 3: "MS A", 4: "MS A", 5: "MS B", 6: "MS D", 7: "MS D"}
			if !maps.Equal(pools, want) {
				t.Errorf("first wave = %v, want %v", pools, want)
			}
		})
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
