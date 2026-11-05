package engine

import (
	"fmt"
	"maps"
	"slices"
	"testing"
)

func TestStandardOrder(t *testing.T) {
	tests := map[int][]int{
		1: {1},
		2: {1, 2},
		4: {1, 4, 2, 3},
		8: {1, 8, 4, 5, 2, 7, 3, 6},
	}
	for size, want := range tests {
		if got := StandardOrder(size); !slices.Equal(got, want) {
			t.Errorf("StandardOrder(%d) = %v, want %v", size, got, want)
		}
	}
}

func TestMeetRound(t *testing.T) {
	if MeetRound(0, 1) != 1 || MeetRound(0, 2) != 2 || MeetRound(0, 7) != 3 || MeetRound(5, 4) != 1 {
		t.Error("wrong meet rounds")
	}
}

func routed(pools, perPool int) []BracketEntrant {
	var out []BracketEntrant
	for p := range pools {
		for r := 1; r <= perPool; r++ {
			out = append(out, BracketEntrant{ID: EntrantID(fmt.Sprintf("%s%d", PoolName(p), r)), Rank: r, Pool: p})
		}
	}
	return out
}

func poolOf(id EntrantID) byte { return id[0] }

func TestSeedBracketFourPoolsTopTwo(t *testing.T) {
	slots := SeedBracket(routed(4, 2))
	if len(slots) != 8 || slots[0] != "A1" {
		t.Fatalf("slots = %v", slots)
	}
	for i := 0; i < 8; i += 2 {
		if poolOf(slots[i]) == poolOf(slots[i+1]) {
			t.Errorf("pool mates meet in round 1: %v", slots)
		}
	}
	// Winner and runner-up of each pool in opposite halves.
	for _, p := range "ABCD" {
		w := slices.Index(slots, EntrantID(string(p)+"1"))
		r := slices.Index(slots, EntrantID(string(p)+"2"))
		if MeetRound(w, r) != 3 {
			t.Errorf("%c1 and %c2 meet before the final: %v", p, p, slots)
		}
	}
	// Pool winners keep the standard seed positions.
	if slots[4] != "B1" || slots[6] != "C1" || slots[2] != "D1" {
		t.Errorf("seeds moved: %v", slots)
	}
}

func TestSeedBracketByes(t *testing.T) {
	// 3 pools, top 2: 6 entrants in a bracket of 8, the two best seeds get byes.
	slots := SeedBracket(routed(3, 2))
	if len(slots) != 8 {
		t.Fatalf("size = %d", len(slots))
	}
	byes := 0
	for i := 0; i < 8; i += 2 {
		a, b := slots[i], slots[i+1]
		if a == "" || b == "" {
			byes++
			if a != "A1" && a != "B1" && b != "A1" && b != "B1" {
				t.Errorf("bye not given to a top seed: %v", slots)
			}
			continue
		}
		if poolOf(a) == poolOf(b) {
			t.Errorf("pool mates meet in round 1: %v", slots)
		}
	}
	if byes != 2 {
		t.Errorf("%d byes, want 2: %v", byes, slots)
	}
}

func TestSeedBracketEveryEntrantOnce(t *testing.T) {
	for pools := 1; pools <= 6; pools++ {
		for per := 1; per <= 4; per++ {
			in := routed(pools, per)
			slots := SeedBracket(in)
			var got []EntrantID
			for _, s := range slots {
				if s != "" {
					got = append(got, s)
				}
			}
			if len(got) != len(in) || len(slots) != NextPowerOfTwo(len(in)) {
				t.Fatalf("%d pools x %d: slots %v", pools, per, slots)
			}
			slices.Sort(got)
			if len(slices.Compact(got)) != len(in) {
				t.Fatalf("%d pools x %d: duplicates %v", pools, per, slots)
			}
		}
	}
}

func TestBracketCounts(t *testing.T) {
	tests := []struct {
		n             int
		full          bool
		matches, rnds int
	}{
		{0, true, 0, 0},
		{1, true, 0, 0},
		{2, true, 1, 1},
		{4, false, 3, 2},
		{4, true, 4, 2},
		{6, false, 5, 3},
		{6, true, 7, 3},
		{8, true, 12, 3},
		{16, true, 32, 4},
	}
	for _, tt := range tests {
		if got := BracketMatches(tt.n, tt.full); got != tt.matches {
			t.Errorf("BracketMatches(%d, %v) = %d, want %d", tt.n, tt.full, got, tt.matches)
		}
		if got := BracketRounds(tt.n); got != tt.rnds {
			t.Errorf("BracketRounds(%d) = %d, want %d", tt.n, got, tt.rnds)
		}
	}
	for n, want := range map[int]int{0: 0, 1: 0, 2: 1, 3: 3, 4: 3, 5: 5} {
		if got := RoundRobinRounds(n); got != want {
			t.Errorf("RoundRobinRounds(%d) = %d, want %d", n, got, want)
		}
	}
}

// checkSeeding reports a pool's winner and runner-up meeting before the
// final, or pool mates meeting in round 1.
func checkSeeding(t *testing.T, name string, in []BracketEntrant, slots []EntrantID) {
	t.Helper()
	pool := map[EntrantID]int{}
	best := map[int][]int{}
	sorted := slices.Clone(in)
	slices.SortStableFunc(sorted, func(a, b BracketEntrant) int { return a.Rank - b.Rank })
	for _, e := range sorted {
		pool[e.ID] = e.Pool
		if len(best[e.Pool]) < 2 {
			best[e.Pool] = append(best[e.Pool], slices.Index(slots, e.ID))
		}
	}
	final := MeetRound(0, len(slots)-1)
	for p, two := range best {
		if len(two) == 2 && len(slots) > 2 && MeetRound(two[0], two[1]) != final {
			t.Errorf("%s: the two best of pool %d meet before the final: %v", name, p, slots)
		}
	}
	// A pool of more entrants than the first-round matches has two of them
	// in one match: then only the halves are checked.
	count := map[int]int{}
	for _, e := range in {
		count[e.Pool]++
	}
	matches := len(in) - len(slots)/2
	if slices.Max(slices.Collect(maps.Values(count))) > matches {
		return
	}
	for i := 0; i < len(slots); i += 2 {
		if slots[i] != "" && slots[i+1] != "" && pool[slots[i]] == pool[slots[i+1]] {
			t.Errorf("%s: pool mates %s and %s meet in round 1: %v", name, slots[i], slots[i+1], slots)
		}
	}
}

func TestSeedBracketRules(t *testing.T) {
	for pools := 2; pools <= 8; pools++ {
		for per := 1; per <= 4; per++ {
			in := routed(pools, per)
			checkSeeding(t, fmt.Sprintf("%d pools x %d", pools, per), in, SeedBracket(in))
			for extra := range pools {
				// Plus the best of the next rank, from any pool.
				more := append(slices.Clone(in), BracketEntrant{ID: EntrantID(fmt.Sprintf("%s%d", PoolName(extra), per+1)), Rank: per + 1, Pool: extra})
				checkSeeding(t, fmt.Sprintf("%d pools x %d + %s", pools, per, more[len(more)-1].ID), more, SeedBracket(more))
			}
		}
	}
}

// Within a rank, the given order (results across pools) decides: the best
// first takes the bye and the slot facing the weakest seed.
func TestSeedBracketBestOfRankFirst(t *testing.T) {
	var in []BracketEntrant
	for r := 1; r <= 3; r++ {
		for _, p := range []int{2, 0, 4, 1, 3} { // C best, then A, E, B, D
			in = append(in, BracketEntrant{ID: EntrantID(fmt.Sprintf("%s%d", PoolName(p), r)), Rank: r, Pool: p})
		}
	}
	slots := SeedBracket(in)
	if slots[0] != "C1" || slots[1] != "" {
		t.Errorf("the best winner should get the bye: %v", slots)
	}
	if i := slices.Index(slots, EntrantID("A1")); i != 8 {
		t.Errorf("the second best winner should take seed 2: %v", slots)
	}
	checkSeeding(t, "15 of 5 pools", in, slots)
}

// Sixteen: the best fourth comes from the top seed's pool, so it swaps with
// the closest weaker seed instead of facing its pool winner.
func TestSeedBracketBestFourthAvoidsItsWinner(t *testing.T) {
	in := routed(5, 3)
	slices.SortStableFunc(in, func(a, b BracketEntrant) int { return a.Rank - b.Rank })
	in = append(in, BracketEntrant{ID: "A4", Rank: 4, Pool: 0})
	slots := SeedBracket(in)
	if slots[0] != "A1" || slots[1] == "A4" || slices.Index(slots, EntrantID("A4")) == -1 {
		t.Errorf("slots = %v", slots)
	}
	checkSeeding(t, "16 with A4", in, slots)
}

// Five pools, top three: two of each pool share a half, but no pool mates
// meet before the semi-finals.
func TestSeedBracketPoolMatesMeetLate(t *testing.T) {
	slots := SeedBracket(routed(5, 3))
	for i, a := range slots {
		for j := i + 1; j < len(slots); j++ {
			if a != "" && slots[j] != "" && poolOf(a) == poolOf(slots[j]) && MeetRound(i, j) < 3 {
				t.Errorf("%s and %s meet in round %d: %v", a, slots[j], MeetRound(i, j), slots)
			}
		}
	}
}
