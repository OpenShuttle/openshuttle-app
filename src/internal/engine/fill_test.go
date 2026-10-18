package engine

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestFillPools(t *testing.T) {
	// Ranks 1 and 2 of four pools (A..D) into two pools.
	var in []BracketEntrant
	for rank := 1; rank <= 2; rank++ {
		for pool := range 4 {
			in = append(in, BracketEntrant{ID: EntrantID(PoolName(pool) + string(rune('0'+rank))), Rank: rank, Pool: pool})
		}
	}
	got := FillPools(in, 2)
	want := [][]EntrantID{{"A1", "D1", "B2", "C2"}, {"B1", "C1", "A2", "D2"}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("FillPools = %v, want %v", got, want)
	}

	// Same group in the snake position: the next pool of the row takes it.
	in = []BracketEntrant{
		{ID: "a1", Rank: 1, Pool: 0}, {ID: "b1", Rank: 1, Pool: 1},
		{ID: "b2", Rank: 2, Pool: 1}, {ID: "a2", Rank: 2, Pool: 0},
	}
	got = FillPools(in, 2)
	want = [][]EntrantID{{"a1", "b2"}, {"b1", "a2"}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("FillPools separation = %v, want %v", got, want)
	}

	// Uneven sizes: larger pools first, everyone placed.
	in = in[:3]
	got = FillPools(in, 2)
	if len(got[0]) != 2 || len(got[1]) != 1 {
		t.Errorf("FillPools sizes = %v", got)
	}
	if FillPools(in, 0) != nil {
		t.Error("FillPools with no pool")
	}
}

func TestDrawBracket(t *testing.T) {
	in := []DrawEntrant{
		{ID: "s1", Seed: 1, Clubs: []string{"X"}},
		{ID: "s2", Seed: 2, Clubs: []string{"Y"}},
		{ID: "a", Clubs: []string{"X"}},
		{ID: "b", Clubs: []string{"Z"}},
		{ID: "c", Clubs: []string{"W"}},
	}
	for seed := range uint64(20) {
		slots := DrawBracket(in, rand.New(rand.NewPCG(seed, 1)))
		if len(slots) != 8 {
			t.Fatalf("size = %d", len(slots))
		}
		// Seeds keep their standard slots; byes face the top seeds.
		if slots[0] != "s1" || slots[4] != "s2" || slots[1] != "" || slots[5] != "" {
			t.Fatalf("seed %d: slots = %v", seed, slots)
		}
		placed := slices.DeleteFunc(slices.Clone(slots), func(id EntrantID) bool { return id == "" })
		if len(placed) != 5 {
			t.Fatalf("placed = %v", placed)
		}
	}

	// Club mates in the first round are pulled apart when possible.
	in = []DrawEntrant{
		{ID: "a", Clubs: []string{"X"}}, {ID: "b", Clubs: []string{"X"}},
		{ID: "c", Clubs: []string{"Y"}}, {ID: "d", Clubs: []string{"Y"}},
	}
	for seed := range uint64(20) {
		slots := DrawBracket(in, rand.New(rand.NewPCG(seed, 2)))
		for i := 0; i < len(slots); i += 2 {
			if (slots[i] == "a" || slots[i] == "b") == (slots[i+1] == "a" || slots[i+1] == "b") {
				t.Fatalf("seed %d: club mates meet in round 1: %v", seed, slots)
			}
		}
	}
}
