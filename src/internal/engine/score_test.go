package engine

import (
	"encoding/json"
	"errors"
	"testing"
)

var (
	bestOfThree21 = WinCondition{Kind: WinBySets, SetsToWin: 2, PointsPerSet: 21, Cap: 30}
	oneSet21      = WinCondition{Kind: WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 30}
	oneSet15NoCap = WinCondition{Kind: WinBySets, SetsToWin: 1, PointsPerSet: 15}
	oneSet11Flat  = WinCondition{Kind: WinBySets, SetsToWin: 1, PointsPerSet: 11, Cap: 11}
	timed12       = WinCondition{Kind: WinByTime, Minutes: 12}
)

func sets(scores ...[2]int) []SetScore {
	out := make([]SetScore, len(scores))
	for i, s := range scores {
		out[i] = SetScore{A: s[0], B: s[1]}
	}
	return out
}

func TestWinConditionValidate(t *testing.T) {
	tests := []struct {
		name string
		w    WinCondition
		ok   bool
	}{
		{"best of three", bestOfThree21, true},
		{"no cap", oneSet15NoCap, true},
		{"cap equals points", oneSet11Flat, true},
		{"timed", timed12, true},
		{"zero sets", WinCondition{Kind: WinBySets, PointsPerSet: 21}, false},
		{"zero points", WinCondition{Kind: WinBySets, SetsToWin: 1}, false},
		{"cap below points", WinCondition{Kind: WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 20}, false},
		{"timed without duration", WinCondition{Kind: WinByTime}, false},
		{"timed too long", WinCondition{Kind: WinByTime, Minutes: 61}, false},
		{"four sets to win", WinCondition{Kind: WinBySets, SetsToWin: 4, PointsPerSet: 21}, false},
		{"cap too high", WinCondition{Kind: WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 100}, false},
		{"unknown kind", WinCondition{Kind: "golf"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.w.Validate()
			if (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestValidateScore(t *testing.T) {
	tests := []struct {
		name    string
		w       WinCondition
		score   Score
		want    Side
		code    Code
		codeSet int
	}{
		// Standard 21-point sets with a cap at 30.
		{"straight sets", bestOfThree21, Score{Sets: sets([2]int{21, 15}, [2]int{21, 19})}, SideA, "", 0},
		{"three sets", bestOfThree21, Score{Sets: sets([2]int{18, 21}, [2]int{21, 10}, [2]int{17, 21})}, SideB, "", 0},
		{"deuce won by two", oneSet21, Score{Sets: sets([2]int{24, 22})}, SideA, "", 0},
		{"cap point", oneSet21, Score{Sets: sets([2]int{29, 30})}, SideB, "", 0},
		{"28-30 at the cap", oneSet21, Score{Sets: sets([2]int{30, 28})}, SideA, "", 0},
		{"21-20 is not over", oneSet21, Score{Sets: sets([2]int{21, 20})}, SideNone, CodeInvalidSetScore, 1},
		{"25-21 overshoots", oneSet21, Score{Sets: sets([2]int{25, 21})}, SideNone, CodeInvalidSetScore, 1},
		{"20-18 is too short", oneSet21, Score{Sets: sets([2]int{20, 18})}, SideNone, CodeInvalidSetScore, 1},
		{"beyond the cap", oneSet21, Score{Sets: sets([2]int{31, 29})}, SideNone, CodeInvalidSetScore, 1},
		{"tied set", oneSet21, Score{Sets: sets([2]int{21, 21})}, SideNone, CodeInvalidSetScore, 1},
		{"negative points", oneSet21, Score{Sets: sets([2]int{21, -1})}, SideNone, CodeInvalidSetScore, 1},
		{"bad second set", bestOfThree21, Score{Sets: sets([2]int{21, 15}, [2]int{22, 21})}, SideNone, CodeInvalidSetScore, 2},

		// Set count.
		{"no set", oneSet21, Score{}, SideNone, CodeInvalidSetCount, 0},
		{"one set in best of three", bestOfThree21, Score{Sets: sets([2]int{21, 15})}, SideNone, CodeInvalidSetCount, 0},
		{"one set each", bestOfThree21, Score{Sets: sets([2]int{21, 15}, [2]int{15, 21})}, SideNone, CodeInvalidSetCount, 0},
		{"third set after a 2-0", bestOfThree21, Score{Sets: sets([2]int{21, 15}, [2]int{21, 15}, [2]int{15, 21})}, SideNone, CodeInvalidSetCount, 0},
		{"four sets", bestOfThree21, Score{Sets: sets([2]int{21, 15}, [2]int{15, 21}, [2]int{21, 15}, [2]int{21, 15})}, SideNone, CodeInvalidSetCount, 0},
		{"golden point on sets", oneSet21, Score{Sets: sets([2]int{21, 15}), GoldenPoint: true}, SideNone, CodeInvalidScore, 0},

		// No cap: deuce goes on.
		{"long deuce", oneSet15NoCap, Score{Sets: sets([2]int{33, 35})}, SideB, "", 0},
		{"no cap needs two points", oneSet15NoCap, Score{Sets: sets([2]int{31, 30})}, SideNone, CodeInvalidSetScore, 1},

		// Cap equals points: no deuce.
		{"first to 11", oneSet11Flat, Score{Sets: sets([2]int{11, 10})}, SideA, "", 0},
		{"12 in a flat set", oneSet11Flat, Score{Sets: sets([2]int{12, 10})}, SideNone, CodeInvalidSetScore, 1},

		// Timed.
		{"timed lead", timed12, Score{Sets: sets([2]int{17, 12})}, SideA, "", 0},
		{"timed low score", timed12, Score{Sets: sets([2]int{2, 3})}, SideB, "", 0},
		{"timed golden point", timed12, Score{Sets: sets([2]int{15, 16}), GoldenPoint: true}, SideB, "", 0},
		{"timed tie needs golden point", timed12, Score{Sets: sets([2]int{15, 15})}, SideNone, CodeInvalidTimedScore, 1},
		{"golden point is one point", timed12, Score{Sets: sets([2]int{18, 15}), GoldenPoint: true}, SideNone, CodeInvalidTimedScore, 1},
		{"timed negative", timed12, Score{Sets: sets([2]int{-1, 3})}, SideNone, CodeInvalidTimedScore, 1},
		{"timed two sets", timed12, Score{Sets: sets([2]int{10, 3}, [2]int{4, 3})}, SideNone, CodeInvalidSetCount, 0},

		{"invalid win condition", WinCondition{}, Score{Sets: sets([2]int{21, 3})}, SideNone, CodeInvalidWinCondition, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateScore(tt.w, tt.score)
			if got != tt.want {
				t.Errorf("winner = %v, want %v", got, tt.want)
			}
			if tt.code == "" {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("error = %v, want code %s", err, tt.code)
			}
			if e.Code != tt.code || e.Set != tt.codeSet {
				t.Errorf("error = %s (set %d), want %s (set %d)", e.Code, e.Set, tt.code, tt.codeSet)
			}
		})
	}
}

func TestScoreTotals(t *testing.T) {
	s := Score{Sets: sets([2]int{18, 21}, [2]int{21, 10}, [2]int{21, 19})}
	if a, b := s.SetsWon(); a != 2 || b != 1 {
		t.Errorf("SetsWon() = %d, %d, want 2, 1", a, b)
	}
	if a, b := s.Points(); a != 60 || b != 50 {
		t.Errorf("Points() = %d, %d, want 60, 50", a, b)
	}
}

func TestErrorJSON(t *testing.T) {
	// The frontend reads {code, set}: keep these names even if an error is
	// marshaled by a generic JSON encoder.
	for _, tt := range []struct {
		err  *Error
		want string
	}{
		{&Error{Code: CodeInvalidSetScore, Set: 2}, `{"code":"INVALID_SET_SCORE","set":2}`},
		{NewError(CodeInvalidScore), `{"code":"INVALID_SCORE"}`},
	} {
		b, err := json.Marshal(tt.err)
		if err != nil || string(b) != tt.want {
			t.Errorf("json = %s, %v; want %s", b, err, tt.want)
		}
	}
}
