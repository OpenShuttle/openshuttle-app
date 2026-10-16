package engine

import (
	"testing"
)

// sc builds a score from "a-b" pairs: sc(21, 15, 21, 19) is 21-15 21-19.
func sc(points ...int) *Score {
	s := &Score{}
	for i := 0; i+1 < len(points); i += 2 {
		s.Sets = append(s.Sets, SetScore{A: points[i], B: points[i+1]})
	}
	return s
}

func match(a, b EntrantID, s *Score) PoolMatch {
	return PoolMatch{A: a, B: b, Score: s}
}

type wantRow struct {
	entrant    EntrantID
	rank       int
	drawNeeded bool
}

func checkRows(t *testing.T, got PoolStandings, want []wantRow) {
	t.Helper()
	if len(got.Rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got.Rows), len(want))
	}
	for i, w := range want {
		g := got.Rows[i]
		if g.Entrant != w.entrant || g.Rank != w.rank || g.DrawNeeded != w.drawNeeded {
			t.Errorf("row %d = {%s rank %d draw %v}, want {%s rank %d draw %v}",
				i, g.Entrant, g.Rank, g.DrawNeeded, w.entrant, w.rank, w.drawNeeded)
		}
	}
}

func TestRankPool(t *testing.T) {
	entrants := []EntrantID{"A", "B", "C", "D"}

	tests := []struct {
		name     string
		pool     Pool
		want     []wantRow
		complete bool
	}{
		{
			name: "wins decide",
			pool: Pool{
				Entrants: entrants,
				Matches: []PoolMatch{
					match("A", "B", sc(21, 10, 21, 10)),
					match("A", "C", sc(21, 10, 21, 10)),
					match("A", "D", sc(21, 10, 21, 10)),
					match("B", "C", sc(21, 10, 21, 10)),
					match("B", "D", sc(21, 10, 21, 10)),
					match("C", "D", sc(21, 10, 21, 10)),
				},
				Tiebreaks: DefaultTiebreaks,
			},
			want:     []wantRow{{"A", 1, false}, {"B", 2, false}, {"C", 3, false}, {"D", 4, false}},
			complete: true,
		},
		{
			name: "head-to-head beats set difference between two",
			pool: Pool{
				Entrants: entrants,
				Matches: []PoolMatch{
					match("A", "B", sc(21, 19, 15, 21, 22, 20)), // A beats B 2-1
					match("A", "C", sc(10, 21, 10, 21)),         // C beats A
					match("A", "D", sc(21, 19, 15, 21, 22, 20)), // A beats D 2-1
					match("B", "C", sc(21, 10, 21, 10)),
					match("B", "D", sc(21, 10, 21, 10)),
					match("C", "D", sc(10, 21, 10, 21)), // D beats C
				},
				Tiebreaks: DefaultTiebreaks,
			},
			// A and B: 2 wins, B has the better set difference (+3 vs 0) but
			// lost to A. C and D: 1 win, D beat C.
			want:     []wantRow{{"A", 1, false}, {"B", 2, false}, {"D", 3, false}, {"C", 4, false}},
			complete: true,
		},
		{
			name: "pair left from a three-way tie restarts at head-to-head",
			pool: Pool{
				Entrants: entrants,
				Matches: []PoolMatch{
					match("A", "B", sc(21, 19, 21, 19)),         // A beats B
					match("A", "C", sc(15, 21, 21, 15, 15, 21)), // C beats A 2-1
					match("A", "D", sc(21, 0, 0, 21, 21, 0)),    // A beats D 2-1
					match("B", "C", sc(21, 0, 21, 0)),           // B beats C
					match("B", "D", sc(21, 0, 21, 0)),
					match("C", "D", sc(21, 0, 21, 0)),
				},
				Tiebreaks: DefaultTiebreaks,
			},
			// A, B, C: 2 wins. Set difference: A +2, B +2, C +1, so C is third.
			// Then A and B restart: head-to-head gives A, although B has the
			// better point difference.
			want:     []wantRow{{"A", 1, false}, {"B", 2, false}, {"C", 3, false}, {"D", 4, false}},
			complete: true,
		},
		{
			name: "full tie needs a draw",
			pool: Pool{
				Entrants: []EntrantID{"A", "B", "C"},
				Matches: []PoolMatch{
					match("A", "B", sc(21, 10)),
					match("B", "C", sc(21, 10)),
					match("C", "A", sc(21, 10)),
				},
				Tiebreaks: DefaultTiebreaks,
			},
			want:     []wantRow{{"A", 1, true}, {"B", 1, true}, {"C", 1, true}},
			complete: true,
		},
		{
			name: "lots break a full tie",
			pool: Pool{
				Entrants: []EntrantID{"A", "B", "C"},
				Matches: []PoolMatch{
					match("A", "B", sc(21, 10)),
					match("B", "C", sc(21, 10)),
					match("C", "A", sc(21, 10)),
				},
				Tiebreaks: DefaultTiebreaks,
				DrawLots:  map[EntrantID]int{"A": 3, "B": 1, "C": 2},
			},
			want:     []wantRow{{"B", 1, false}, {"C", 2, false}, {"A", 3, false}},
			complete: true,
		},
		{
			name: "no draw requested while the pool is running",
			pool: Pool{
				Entrants: []EntrantID{"A", "B", "C"},
				Matches: []PoolMatch{
					match("A", "B", nil),
					match("B", "C", nil),
					match("C", "A", nil),
				},
				Tiebreaks: DefaultTiebreaks,
			},
			want:     []wantRow{{"A", 1, false}, {"B", 1, false}, {"C", 1, false}},
			complete: false,
		},
		{
			name: "no draw requested when the stage does not use it",
			pool: Pool{
				Entrants: []EntrantID{"A", "B", "C"},
				Matches: []PoolMatch{
					match("A", "B", sc(21, 10)),
					match("B", "C", sc(21, 10)),
					match("C", "A", sc(21, 10)),
				},
				Tiebreaks: []Tiebreak{TiebreakWins, TiebreakSetDifference},
			},
			want:     []wantRow{{"A", 1, false}, {"B", 1, false}, {"C", 1, false}},
			complete: true,
		},
		{
			name: "custom order: point difference before head-to-head",
			pool: Pool{
				Entrants: []EntrantID{"A", "B", "C"},
				Matches: []PoolMatch{
					match("A", "B", sc(21, 19)), // A beats B by 2
					match("A", "C", sc(10, 21)), // C beats A by 11
					match("B", "C", sc(21, 5)),  // B beats C by 16
				},
				Tiebreaks: []Tiebreak{TiebreakWins, TiebreakPointDifference, TiebreakHeadToHead},
			},
			// All 1 win. Points: A -9, B +14, C -5.
			want:     []wantRow{{"B", 1, false}, {"C", 2, false}, {"A", 3, false}},
			complete: true,
		},
		{
			name: "withdrawn entrant ranks last and its matches are walkovers",
			pool: Pool{
				Entrants: entrants,
				Matches: []PoolMatch{
					match("A", "B", sc(21, 10)),
					match("A", "C", sc(21, 10)),
					match("A", "D", sc(10, 21)), // played, but D withdrew
					match("B", "C", sc(21, 10)),
					match("B", "D", nil),
					match("C", "D", nil),
				},
				Withdrawn: map[EntrantID]bool{"D": true},
				Tiebreaks: DefaultTiebreaks,
			},
			want:     []wantRow{{"A", 1, false}, {"B", 2, false}, {"C", 3, false}, {"D", 4, false}},
			complete: true,
		},
		{
			name: "two withdrawn entrants rank last in draw order",
			pool: Pool{
				Entrants: entrants,
				Matches: []PoolMatch{
					match("A", "B", sc(21, 10)),
					match("A", "C", nil),
					match("A", "D", nil),
					match("B", "C", nil),
					match("B", "D", nil),
					match("C", "D", nil),
				},
				Withdrawn: map[EntrantID]bool{"A": true, "C": true},
				Tiebreaks: DefaultTiebreaks,
			},
			// B and D both win their two walkovers; B vs D is not played.
			want:     []wantRow{{"B", 1, false}, {"D", 1, false}, {"A", 3, false}, {"C", 4, false}},
			complete: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RankPool(tt.pool)
			if got.Complete != tt.complete {
				t.Errorf("Complete = %v, want %v", got.Complete, tt.complete)
			}
			checkRows(t, got, tt.want)
		})
	}
}

func TestRankPoolWalkoverStats(t *testing.T) {
	got := RankPool(Pool{
		Entrants: []EntrantID{"A", "B", "C"},
		Matches: []PoolMatch{
			match("A", "B", sc(21, 15, 21, 18)),
			match("A", "C", sc(10, 21, 10, 21)), // C withdrew afterwards
			match("B", "C", nil),
		},
		Withdrawn: map[EntrantID]bool{"C": true},
		Tiebreaks: DefaultTiebreaks,
	})

	want := map[EntrantID]Standing{
		"A": {Played: 2, Wins: 2, Walkovers: 1, SetsWon: 2, SetsLost: 0, PointsWon: 42, PointsLost: 33},
		"B": {Played: 2, Wins: 1, Losses: 1, Walkovers: 1, SetsWon: 0, SetsLost: 2, PointsWon: 33, PointsLost: 42},
		"C": {Played: 2, Losses: 2, Walkovers: 2, Withdrawn: true},
	}
	for _, row := range got.Rows {
		w := want[row.Entrant]
		w.Entrant, w.Rank = row.Entrant, row.Rank
		if row != w {
			t.Errorf("%s = %+v, want %+v", row.Entrant, row, w)
		}
	}
}

func TestRankPoolTimedMatches(t *testing.T) {
	got := RankPool(Pool{
		Entrants: []EntrantID{"A", "B", "C"},
		Matches: []PoolMatch{
			match("A", "B", &Score{Sets: []SetScore{{A: 15, B: 16}}, GoldenPoint: true}),
			match("A", "C", sc(18, 9)),
			match("B", "C", sc(8, 12)),
		},
		Tiebreaks: DefaultTiebreaks,
	})
	// All 1 win, set difference 0. Points: A +8, B -3, C -5.
	checkRows(t, got, []wantRow{{"A", 1, false}, {"B", 2, false}, {"C", 3, false}})
	if got.Rows[0].PointsWon != 33 || got.Rows[0].PointsLost != 25 {
		t.Errorf("A points = %d/%d, want 33/25", got.Rows[0].PointsWon, got.Rows[0].PointsLost)
	}
}
