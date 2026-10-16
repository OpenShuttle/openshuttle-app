package engine

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestPoolSizes(t *testing.T) {
	tests := []struct {
		n, pools int
		want     []int
	}{
		{16, 4, []int{4, 4, 4, 4}},
		{14, 4, []int{4, 4, 3, 3}},
		{7, 2, []int{4, 3}},
		{3, 1, []int{3}},
		{0, 2, []int{0, 0}},
		{5, 0, nil},
	}
	for _, tt := range tests {
		if got := PoolSizes(tt.n, tt.pools); !slices.Equal(got, tt.want) {
			t.Errorf("PoolSizes(%d, %d) = %v, want %v", tt.n, tt.pools, got, tt.want)
		}
	}
}

func TestRoundRobinMatches(t *testing.T) {
	for n, want := range map[int]int{0: 0, 1: 0, 2: 1, 3: 3, 4: 6, 5: 10} {
		if got := RoundRobinMatches(n); got != want {
			t.Errorf("RoundRobinMatches(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestPoolName(t *testing.T) {
	for i, want := range map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB"} {
		if got := PoolName(i); got != want {
			t.Errorf("PoolName(%d) = %s, want %s", i, got, want)
		}
	}
}

func entrants(n int, club func(i int) string, seeds map[int]int) []DrawEntrant {
	out := make([]DrawEntrant, n)
	for i := range out {
		out[i] = DrawEntrant{ID: EntrantID(fmt.Sprintf("e%d", i)), Seed: seeds[i], Clubs: []string{club(i)}}
	}
	return out
}

func TestDrawPoolsSizesAndCompleteness(t *testing.T) {
	es := entrants(14, func(i int) string { return fmt.Sprint(i) }, nil)
	got := DrawPools(es, 4, rand.New(rand.NewPCG(1, 2)))
	var all []EntrantID
	for i, pool := range got {
		if len(pool) != PoolSizes(14, 4)[i] {
			t.Errorf("pool %d has %d entrants", i, len(pool))
		}
		all = append(all, pool...)
	}
	slices.Sort(all)
	if len(slices.Compact(all)) != 14 {
		t.Errorf("entrants lost or duplicated: %v", all)
	}
}

func TestDrawPoolsSeedsSnake(t *testing.T) {
	// 6 seeds in 4 pools: A, B, C, D, then D, C.
	seeds := map[int]int{0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 6}
	es := entrants(16, func(i int) string { return fmt.Sprint(i) }, seeds)
	got := DrawPools(es, 4, rand.New(rand.NewPCG(7, 7)))
	want := map[EntrantID]int{"e0": 0, "e1": 1, "e2": 2, "e3": 3, "e4": 3, "e5": 2}
	for id, pool := range want {
		if !slices.Contains(got[pool], id) {
			t.Errorf("seed %s not in pool %s: %v", id, PoolName(pool), got)
		}
	}
	for p := range got {
		if got[p][0] != es[p].ID {
			t.Errorf("pool %s should start with its top seed, got %v", PoolName(p), got[p])
		}
	}
}

func TestDrawPoolsSeparatesClubs(t *testing.T) {
	// 4 clubs of 4 in 4 pools: a perfect separation exists. Whatever the
	// shuffle, no pool should hold two players of the same club.
	es := entrants(16, func(i int) string { return fmt.Sprint("club", i%4) }, nil)
	for seed := range uint64(50) {
		got := DrawPools(es, 4, rand.New(rand.NewPCG(seed, 1)))
		for p, pool := range got {
			seen := map[string]bool{}
			for _, id := range pool {
				club := es[slices.IndexFunc(es, func(e DrawEntrant) bool { return e.ID == id })].Clubs[0]
				if seen[club] {
					t.Fatalf("seed %d: pool %s has two players of %s: %v", seed, PoolName(p), club, pool)
				}
				seen[club] = true
			}
		}
	}
}

func TestDrawPoolsIsReproducible(t *testing.T) {
	es := entrants(10, func(i int) string { return "same" }, map[int]int{3: 1})
	a := DrawPools(es, 3, rand.New(rand.NewPCG(42, 0)))
	b := DrawPools(es, 3, rand.New(rand.NewPCG(42, 0)))
	for i := range a {
		if !slices.Equal(a[i], b[i]) {
			t.Fatalf("same seed, different draws: %v vs %v", a, b)
		}
	}
}

func TestDrawPoolsMoreSeedsThanRoom(t *testing.T) {
	// Every entrant seeded, 5 in 2 pools of 3 and 2: all placed once.
	seeds := map[int]int{0: 1, 1: 2, 2: 3, 3: 4, 4: 5}
	es := entrants(5, func(i int) string { return fmt.Sprint(i) }, seeds)
	got := DrawPools(es, 2, rand.New(rand.NewPCG(1, 1)))
	if len(got[0]) != 3 || len(got[1]) != 2 {
		t.Errorf("sizes = %d, %d", len(got[0]), len(got[1]))
	}
}
