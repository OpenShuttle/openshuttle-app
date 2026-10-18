package engine

import (
	"fmt"
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
