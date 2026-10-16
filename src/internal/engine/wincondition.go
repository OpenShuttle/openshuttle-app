package engine

// WinConditionKind says how a match of a stage is won.
type WinConditionKind string

const (
	// WinBySets: first to win SetsToWin sets, each set played to PointsPerSet.
	WinBySets WinConditionKind = "sets"
	// WinByTime: a single period of Duration; the leader at the whistle wins,
	// a tie is decided by a golden point.
	WinByTime WinConditionKind = "timed"
)

// WinCondition is how a match is won in a stage. It is stored in events:
// keep the JSON names.
type WinCondition struct {
	Kind WinConditionKind `json:"kind"`

	// Set-based stages.
	SetsToWin    int `json:"setsToWin,omitempty"`    // 1 for a single set, 2 for best of three
	PointsPerSet int `json:"pointsPerSet,omitempty"` // usually 21
	// Cap is the point at which a deuce ends: at Cap-1 all, the next point
	// wins (30 in standard rules). 0 means no cap: a set always needs a
	// 2-point lead. Cap == PointsPerSet means no deuce: first to the points wins.
	Cap int `json:"cap,omitempty"`

	// Timed stages: the duration of a round, in minutes.
	Minutes int `json:"minutes,omitempty"`
}

// Limits of a win condition, to keep forms sane.
const (
	MaxSetsToWin    = 3
	MaxPointsPerSet = 99
	MaxMinutes      = 60
)

// Validate checks that the win condition can be played.
func (w WinCondition) Validate() error {
	switch w.Kind {
	case WinBySets:
		if w.SetsToWin < 1 || w.SetsToWin > MaxSetsToWin || w.PointsPerSet < 1 || w.PointsPerSet > MaxPointsPerSet {
			return newError(CodeInvalidWinCondition)
		}
		if w.Cap != 0 && (w.Cap < w.PointsPerSet || w.Cap > MaxPointsPerSet) {
			return newError(CodeInvalidWinCondition)
		}
	case WinByTime:
		if w.Minutes < 1 || w.Minutes > MaxMinutes {
			return newError(CodeInvalidWinCondition)
		}
	default:
		return newError(CodeInvalidWinCondition)
	}
	return nil
}

// MaxSets is the most sets a match can have: 2*SetsToWin-1 for set-based
// stages, 1 for timed stages.
func (w WinCondition) MaxSets() int {
	if w.Kind == WinByTime {
		return 1
	}
	return 2*w.SetsToWin - 1
}
