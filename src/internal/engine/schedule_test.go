package engine

import "testing"

func TestRoundRobinSchedule(t *testing.T) {
	for n := 0; n <= 9; n++ {
		rounds := RoundRobinSchedule(n)
		seen := map[[2]int]bool{}
		for r, round := range rounds {
			busy := map[int]bool{}
			for _, p := range round {
				if p[0] >= p[1] || p[0] < 0 || p[1] >= n {
					t.Fatalf("n=%d: bad pair %v", n, p)
				}
				if busy[p[0]] || busy[p[1]] {
					t.Fatalf("n=%d round %d: an entrant plays twice: %v", n, r, round)
				}
				busy[p[0]], busy[p[1]] = true, true
				if seen[p] {
					t.Fatalf("n=%d: pair %v twice", n, p)
				}
				seen[p] = true
			}
		}
		if len(seen) != RoundRobinMatches(n) {
			t.Errorf("n=%d: %d matches, want %d", n, len(seen), RoundRobinMatches(n))
		}
		wantRounds := n - 1
		if n%2 == 1 {
			wantRounds = n
		}
		if n >= 2 && len(rounds) != wantRounds {
			t.Errorf("n=%d: %d rounds, want %d", n, len(rounds), wantRounds)
		}
	}
}
