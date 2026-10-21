package blocks

import (
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestKeys(t *testing.T) {
	tests := []struct {
		got, want string
		round     bool
	}{
		{PoolKey(engine.MensDoubles, "S1", 0, 1), "MD/S1/A/2", true},
		{PoolGroup(engine.MensDoubles, "S1", 2), "MD/S1/C", false},
		{BracketKey(engine.MensDoubles, "S2", 0, 1), "MD/S2/R1", false},
		{BracketKey(engine.MensDoubles, "S2", 1, 5), "MD/S2/R2/P5", true},
	}
	for _, tt := range tests {
		if tt.got != tt.want || IsRound(tt.got) != tt.round {
			t.Errorf("key %q (round %v), want %q (round %v)", tt.got, IsRound(tt.got), tt.want, tt.round)
		}
	}
}

// pool builds the blocks of a pool: one per round, its matches named after
// the key.
func pool(c engine.Category, p, rounds int) [][]Block {
	var waves [][]Block
	for r := range rounds {
		k := PoolKey(c, "S1", p, r)
		g := PoolGroup(c, "S1", p)
		waves = append(waves, []Block{{Key: k, Group: g, Series: g, Round: r, Place: p, Matches: []string{k + "#1"}}})
	}
	return waves
}

// merge puts the waves of several pools side by side.
func merge(pools ...[][]Block) [][]Block {
	var out [][]Block
	for _, p := range pools {
		for k, w := range p {
			for len(out) <= k {
				out = append(out, nil)
			}
			out[k] = append(out[k], w...)
		}
	}
	return out
}

func keys(bs []Block) []string {
	var out []string
	for _, b := range bs {
		out = append(out, b.Key)
	}
	return out
}

func TestOrder(t *testing.T) {
	md, wd := engine.MensDoubles, engine.WomensDoubles
	waves := map[engine.Category][][]Block{
		md: merge(pool(md, 0, 2), pool(md, 1, 2)),
		wd: pool(wd, 0, 1),
	}
	tests := []struct {
		name       string
		categories []engine.Category
		stored     []string
		want       []string
	}{
		{"waves side by side, men first", []engine.Category{md, wd}, nil,
			[]string{"MD/S1/A/1", "MD/S1/B/1", "WD/S1/A/1", "MD/S1/A/2", "MD/S1/B/2"}},
		{"women open each wave", []engine.Category{wd, md}, nil,
			[]string{"WD/S1/A/1", "MD/S1/A/1", "MD/S1/B/1", "MD/S1/A/2", "MD/S1/B/2"}},
		{"older files: pool B first in each wave", []engine.Category{md, wd}, []string{"MD/S1/B"},
			[]string{"MD/S1/B/1", "MD/S1/A/1", "WD/S1/A/1", "MD/S1/B/2", "MD/S1/A/2"}},
		{"desk order holds, new blocks after", []engine.Category{md, wd}, []string{"MD/S1/B/1", "MD/S1/B/2", "MD/S1/A/1"},
			[]string{"MD/S1/B/1", "MD/S1/B/2", "MD/S1/A/1", "WD/S1/A/1", "MD/S1/A/2"}},
		{"a pool's rounds keep their order", []engine.Category{md, wd}, []string{"MD/S1/A/2", "MD/S1/A/1"},
			[]string{"MD/S1/A/1", "MD/S1/A/2", "MD/S1/B/1", "WD/S1/A/1", "MD/S1/B/2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keys(Order(waves, tt.categories, tt.stored)); !slices.Equal(got, tt.want) {
				t.Errorf("order = %v\nwant    %v", got, tt.want)
			}
		})
	}
}

func TestSortByPlace(t *testing.T) {
	waves := [][]Block{{{Key: "P5", Place: 5}, {Key: "main", Place: 1}}}
	SortByPlace(waves)
	if waves[0][0].Key != "main" {
		t.Errorf("wave = %v", keys(waves[0]))
	}
}

func TestMove(t *testing.T) {
	md := engine.MensDoubles
	order := Order(map[engine.Category][][]Block{md: merge(pool(md, 0, 2), pool(md, 1, 2))}, []engine.Category{md}, nil)
	tests := []struct {
		name, key, before string
		want              []string
		ok                bool
	}{
		{"before the first", "MD/S1/B/1", "MD/S1/A/1", []string{"MD/S1/B/1", "MD/S1/A/1", "MD/S1/A/2", "MD/S1/B/2"}, true},
		{"to the end", "MD/S1/A/1", "", []string{"MD/S1/B/1", "MD/S1/A/1", "MD/S1/B/2", "MD/S1/A/2"}, true},
		{"a later round first gives its place to round 1", "MD/S1/B/2", "MD/S1/A/1", []string{"MD/S1/B/1", "MD/S1/A/1", "MD/S1/B/2", "MD/S1/A/2"}, true},
		{"on itself", "MD/S1/A/2", "MD/S1/A/2", []string{"MD/S1/A/1", "MD/S1/B/1", "MD/S1/A/2", "MD/S1/B/2"}, true},
		{"unknown block", "MD/S1/Z/1", "", nil, false},
		{"unknown target", "MD/S1/A/1", "MD/S1/Z/1", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Move(order, tt.key, tt.before)
			if ok != tt.ok || !slices.Equal(got, tt.want) {
				t.Errorf("Move = %v, %v; want %v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
