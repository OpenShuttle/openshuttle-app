package engine

import (
	"slices"
	"testing"
)

func TestAcrossKeys(t *testing.T) {
	win := func(a, b EntrantID, pa, pb int) PoolMatch {
		return PoolMatch{A: a, B: b, Score: &Score{Sets: []SetScore{{A: pa, B: pb}}}}
	}
	// Pool A of 3: x beats everyone. Pool B of 4: y3 is third, but its only
	// win is against the last of the pool.
	a := Pool{Entrants: []EntrantID{"x1", "x2", "x3"}, Tiebreaks: DefaultTiebreaks, Matches: []PoolMatch{
		win("x1", "x2", 21, 10), win("x1", "x3", 21, 10), win("x2", "x3", 21, 10),
	}}
	b := Pool{Entrants: []EntrantID{"y1", "y2", "y3", "y4"}, Tiebreaks: DefaultTiebreaks, Matches: []PoolMatch{
		win("y1", "y2", 21, 10), win("y1", "y3", 21, 10), win("y1", "y4", 21, 10),
		win("y2", "y3", 21, 10), win("y2", "y4", 21, 10), win("y3", "y4", 21, 5),
	}}
	pools := []RankedPool{{a, RankPool(a)}, {b, RankPool(b)}}

	raw := AcrossKeys(pools, CrossPoolRaw)
	if raw["y3"] != (AcrossKey{1, -1, -6}) || raw["x3"] != (AcrossKey{0, -2, -22}) {
		t.Fatalf("raw keys = %+v / %+v", raw["y3"], raw["x3"])
	}
	// On the same number of matches, y3 loses its win against y4.
	same := AcrossKeys(pools, CrossPoolSameMatches)
	if same["y3"] != (AcrossKey{0, -2, -22}) || same["y1"] != (AcrossKey{2, 2, 22}) {
		t.Fatalf("same-matches keys = %+v / %+v", same["y3"], same["y1"])
	}
	if got := SortAcross([]EntrantID{"x3", "y3"}, raw); !slices.Equal(got, []EntrantID{"y3", "x3"}) {
		t.Errorf("raw order = %v", got)
	}
	// Equal keys keep the given order.
	if got := SortAcross([]EntrantID{"x3", "y3"}, same); !slices.Equal(got, []EntrantID{"x3", "y3"}) {
		t.Errorf("same-matches order = %v", got)
	}
	if !CrossPoolRaw.Valid() || CrossPool("x").Valid() {
		t.Error("Valid")
	}
}
