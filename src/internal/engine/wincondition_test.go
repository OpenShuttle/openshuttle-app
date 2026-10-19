package engine

import "testing"

func TestEstimatedMinutes(t *testing.T) {
	sets := func(n, points int) WinCondition {
		return WinCondition{Kind: WinBySets, SetsToWin: n, PointsPerSet: points, Cap: points}
	}
	tests := []struct {
		name string
		win  WinCondition
		want int
	}{
		{"2 sets of 15", sets(2, 15), 22},
		{"2 sets of 21", sets(2, 21), 30},
		{"1 set of 15", sets(1, 15), 11},
		{"3 sets of 11", sets(3, 11), 25},
		{"1 set of 21", sets(1, 21), 15},
		{"timed 10 min", WinCondition{Kind: WinByTime, Minutes: 10}, 12},
		{"estimate set", WinCondition{Kind: WinBySets, SetsToWin: 2, PointsPerSet: 21, Estimate: 35}, 35},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.win.EstimatedMinutes(); got != tt.want {
				t.Errorf("estimate = %d, want %d", got, tt.want)
			}
		})
	}
	if err := (WinCondition{Kind: WinBySets, SetsToWin: 1, PointsPerSet: 21, Estimate: 181}).Validate(); err == nil {
		t.Error("estimate over the limit accepted")
	}
}
