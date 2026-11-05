package forecast

import (
	"maps"
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestRunCourtRules(t *testing.T) {
	m := func(id string, a, b string) Item {
		return Item{Key: "MD/S1/A/1", Category: md, Stage: "S1", Match: id, A: engine.EntrantID("m" + a), B: engine.EntrantID("m" + b), Minutes: 20}
	}
	w := func(id, pool string, a, b string) Item {
		p := 0
		if pool == "B" {
			p = 1
		}
		return Item{Key: "WD/S1/" + pool + "/1", Category: wd, Stage: "S1", Pool: p, Match: id, A: engine.EntrantID("w" + a), B: engine.EntrantID("w" + b), Minutes: 20}
	}
	final := Item{Key: "MD/S2/R1", Category: md, Stage: "S2", Pool: -1, RoundSize: 2, PlaceFrom: 1, Match: "F", After: []string{"S1"}, Minutes: 20}
	third := Item{Key: "MD/S2/R1/P3", Category: md, Stage: "S2", Pool: -1, RoundSize: 2, PlaceFrom: 3, Match: "T3", After: []string{"S1"}, Minutes: 20}
	semi := Item{Key: "MD/S2/R0", Category: md, Stage: "S2", Pool: -1, RoundSize: 4, PlaceFrom: 1, Match: "SF", After: []string{"S1"}, Minutes: 20}
	wdPools := Scope{Category: wd, Stage: "S1"}
	tests := []struct {
		name string
		in   Input
		want map[string]placed
		off  []string
	}{
		{
			name: "without rules, courts follow the order",
			in:   Input{Courts: 2, Items: []Item{m("m1", "a", "b"), m("m2", "c", "d"), w("w1", "A", "a", "b")}},
			want: map[string]placed{"m1": {1, 0, false}, "m2": {2, 0, false}, "w1": {1, 20, false}},
		},
		{
			name: "a priority court takes its scope before the order",
			in: Input{Courts: 2, Items: []Item{m("m1", "a", "b"), m("m2", "c", "d"), w("w1", "A", "a", "b")},
				Rules: map[int][]Rule{1: {{Kind: RulePriority, Level: 1, Scope: wdPools}}}},
			want: map[string]placed{"w1": {1, 0, false}, "m1": {2, 0, false}, "m2": {1, 20, false}},
		},
		{
			name: "a priority court free with others takes its scope first, whatever its number",
			in: Input{Courts: 2, Items: []Item{m("m1", "a", "b"), m("m2", "c", "d"), w("w1", "A", "a", "b")},
				Rules: map[int][]Rule{2: {{Kind: RulePriority, Level: 1, Scope: wdPools}}}},
			want: map[string]placed{"w1": {2, 0, false}, "m1": {1, 0, false}, "m2": {1, 20, false}},
		},
		{
			name: "the lowest level first, the order on a tie",
			in: Input{Courts: 1, Items: []Item{w("a1", "A", "a", "b"), w("b1", "B", "c", "d"), m("m1", "a", "b")},
				Rules: map[int][]Rule{1: {
					{Kind: RulePriority, Level: 2, Scope: Scope{Category: wd, Stage: "S1", Pool: 1}},
					{Kind: RulePriority, Level: 1, Scope: Scope{Category: wd, Stage: "S1", Pool: 2}},
					{Kind: RulePriority, Level: 1, Scope: Scope{Category: md}},
				}}},
			want: map[string]placed{"b1": {1, 0, false}, "m1": {1, 20, false}, "a1": {1, 40, false}},
		},
		{
			name: "a priority ends with its scope: the court follows the order again",
			in: Input{Courts: 2, Items: []Item{m("m1", "a", "b"), m("m2", "c", "d"), m("m3", "e", "f"), w("w1", "A", "a", "b")},
				Rules: map[int][]Rule{2: {{Kind: RulePriority, Level: 1, Scope: wdPools}}}},
			want: map[string]placed{"w1": {2, 0, false}, "m1": {1, 0, false}, "m2": {1, 20, false}, "m3": {2, 20, false}},
		},
		{
			name: "an only court takes nothing else, then anything once its scope is over",
			in: Input{Courts: 2, Items: []Item{m("m1", "a", "b"), m("m2", "c", "d"), m("m3", "e", "f"), w("w1", "A", "a", "b")},
				Rules: map[int][]Rule{1: {{Kind: RuleOnly, Scope: Scope{Category: wd}}}}},
			want: map[string]placed{"w1": {1, 0, false}, "m1": {2, 0, false}, "m2": {1, 20, false}, "m3": {2, 20, false}},
		},
		{
			name: "an only court waits idle for its round, and gets it",
			in: Input{Courts: 2, Items: []Item{m("m1", "a", "b"), m("m2", "c", "d"), final, third},
				Rules: map[int][]Rule{2: {{Kind: RuleOnly, Scope: Scope{Category: md, Stage: "S2", RoundSize: 2, PlaceFrom: 1}}}}},
			want: map[string]placed{"m1": {1, 0, false}, "m2": {1, 20, false}, "F": {2, 40, false}, "T3": {1, 40, false}},
		},
		{
			name: "several only rules: any of their scopes",
			in: Input{Courts: 2, Items: []Item{m("m1", "a", "b"), w("a1", "A", "a", "b"), w("b1", "B", "c", "d")},
				Rules: map[int][]Rule{1: {
					{Kind: RuleOnly, Scope: Scope{Category: wd, Stage: "S1", Pool: 1}},
					{Kind: RuleOnly, Scope: Scope{Category: wd, Stage: "S1", Pool: 2}},
				}}},
			want: map[string]placed{"a1": {1, 0, false}, "m1": {2, 0, false}, "b1": {1, 20, false}},
		},
		{
			name: "a court never takes its forbidden scope",
			in: Input{Courts: 2, Items: []Item{w("w1", "A", "a", "b"), w("w2", "A", "c", "d"), m("m1", "a", "b")},
				Rules: map[int][]Rule{1: {{Kind: RuleNever, Scope: Scope{Category: wd}}}}},
			want: map[string]placed{"m1": {1, 0, false}, "w1": {2, 0, false}, "w2": {2, 20, false}},
		},
		{
			name: "rules leaving an item no court can take are dropped, the item flagged",
			in: Input{Courts: 1, Items: []Item{m("m1", "a", "b"), w("w1", "A", "a", "b")},
				Rules: map[int][]Rule{1: {{Kind: RuleNever, Scope: Scope{Category: wd}}}}},
			want: map[string]placed{"m1": {1, 0, false}, "w1": {1, 20, false}},
			off:  []string{"w1"},
		},
		{
			// Court 1 never takes the pools, court 2 waits for the semi-final
			// only: the pool match goes against the rules at once (it used to
			// start in 2038, freezing the timeline).
			name: "impossible rules: the stuck item placed now, the others keep their rules",
			in: Input{Courts: 2, Items: []Item{m("m1", "a", "b"), semi},
				Rules: map[int][]Rule{
					1: {{Kind: RuleNever, Scope: Scope{Category: md, Stage: "S1"}}},
					2: {{Kind: RuleOnly, Scope: Scope{Category: md, Stage: "S2", RoundSize: 4, PlaceFrom: 1}}},
				}},
			want: map[string]placed{"m1": {2, 0, false}, "SF": {2, 20, false}},
			off:  []string{"m1"},
		},
		{
			// No court may take WD: once MD is placed, its matches share the
			// courts at once, not one by one.
			name: "items no court can take spread over the courts",
			in: Input{Courts: 3, Items: []Item{m("m1", "a", "b"), m("m2", "c", "d"), w("w1", "A", "a", "b"), w("w2", "A", "c", "d")},
				Rules: map[int][]Rule{
					1: {{Kind: RuleNever, Scope: Scope{Category: wd}}},
					2: {{Kind: RuleNever, Scope: Scope{Category: wd}}},
					3: {{Kind: RuleNever, Scope: Scope{Category: wd}}},
				}},
			want: map[string]placed{"m1": {1, 0, false}, "m2": {2, 0, false}, "w1": {1, 20, false}, "w2": {2, 20, false}},
			off:  []string{"w1", "w2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.Now = t0
			res := Run(tt.in)
			got := summary(res)
			if !maps.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			var off []string
			for _, sl := range res.Slots {
				if sl.OffRules {
					off = append(off, sl.Match)
				}
			}
			if !slices.Equal(off, tt.off) {
				t.Errorf("off rules %v, want %v", off, tt.off)
			}
		})
	}
}
