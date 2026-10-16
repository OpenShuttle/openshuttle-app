package engine

// Side is one of the two sides of a match.
type Side int

const (
	SideNone Side = iota
	SideA
	SideB
)

// Opponent returns the other side.
func (s Side) Opponent() Side {
	switch s {
	case SideA:
		return SideB
	case SideB:
		return SideA
	}
	return SideNone
}

// SetScore is the points of each side in one set.
type SetScore struct {
	A int `json:"a"`
	B int `json:"b"`
}

// Winner returns the side with more points, or SideNone on a tie.
func (s SetScore) Winner() Side {
	switch {
	case s.A > s.B:
		return SideA
	case s.B > s.A:
		return SideB
	}
	return SideNone
}

func (s SetScore) points(side Side) (own, other int) {
	if side == SideA {
		return s.A, s.B
	}
	return s.B, s.A
}

// Score is the result of a played match. A timed match has exactly one set
// holding the final points, golden point included.
type Score struct {
	Sets []SetScore `json:"sets"`
	// GoldenPoint is set when a timed match was tied at the whistle and
	// decided by a golden point. The golden point is counted in Sets.
	GoldenPoint bool `json:"goldenPoint,omitempty"`
}

// SetsWon returns the number of sets won by each side.
func (s Score) SetsWon() (a, b int) {
	for _, set := range s.Sets {
		switch set.Winner() {
		case SideA:
			a++
		case SideB:
			b++
		}
	}
	return a, b
}

// Winner returns the side that won more sets, or SideNone. It assumes the
// score was checked with ValidateScore.
func (s Score) Winner() Side {
	a, b := s.SetsWon()
	return SetScore{A: a, B: b}.Winner()
}

// Points returns the total points scored by each side.
func (s Score) Points() (a, b int) {
	for _, set := range s.Sets {
		a += set.A
		b += set.B
	}
	return a, b
}

// ValidateScore checks a score against the win condition and returns the
// winning side.
func ValidateScore(w WinCondition, s Score) (Side, error) {
	if err := w.Validate(); err != nil {
		return SideNone, err
	}
	if w.Kind == WinByTime {
		return validateTimedScore(s)
	}
	return validateSetScore(w, s)
}

func validateTimedScore(s Score) (Side, error) {
	if len(s.Sets) != 1 {
		return SideNone, newError(CodeInvalidSetCount)
	}
	set := s.Sets[0]
	if set.A < 0 || set.B < 0 {
		return SideNone, &Error{Code: CodeInvalidTimedScore, Set: 1}
	}
	winner := set.Winner()
	if winner == SideNone {
		// A tie must be decided by a golden point, which is counted in the score.
		return SideNone, &Error{Code: CodeInvalidTimedScore, Set: 1}
	}
	if s.GoldenPoint {
		own, other := set.points(winner)
		if own-other != 1 {
			return SideNone, &Error{Code: CodeInvalidTimedScore, Set: 1}
		}
	}
	return winner, nil
}

func validateSetScore(w WinCondition, s Score) (Side, error) {
	if s.GoldenPoint {
		return SideNone, newError(CodeInvalidScore)
	}
	if len(s.Sets) < w.SetsToWin || len(s.Sets) > w.MaxSets() {
		return SideNone, newError(CodeInvalidSetCount)
	}
	var wonA, wonB int
	for i, set := range s.Sets {
		if wonA == w.SetsToWin || wonB == w.SetsToWin {
			// The match was already over.
			return SideNone, newError(CodeInvalidSetCount)
		}
		if !validSet(w, set) {
			return SideNone, &Error{Code: CodeInvalidSetScore, Set: i + 1}
		}
		if set.Winner() == SideA {
			wonA++
		} else {
			wonB++
		}
	}
	switch {
	case wonA == w.SetsToWin:
		return SideA, nil
	case wonB == w.SetsToWin:
		return SideB, nil
	}
	return SideNone, newError(CodeInvalidSetCount)
}

// validSet reports whether a set ended legally under the win condition.
func validSet(w WinCondition, set SetScore) bool {
	winner := set.Winner()
	if winner == SideNone || set.A < 0 || set.B < 0 {
		return false
	}
	won, lost := set.points(winner)
	target, limit := w.PointsPerSet, w.Cap

	if limit == target {
		// No deuce: first to the target wins.
		return won == target
	}
	if won == target {
		return lost <= target-2
	}
	if won < target || (limit != 0 && won > limit) {
		return false
	}
	// Deuce: a 2-point lead, or the cap point.
	return won-lost == 2 || (won == limit && lost == limit-1)
}
