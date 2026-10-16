package tournament

import (
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const TypePoolStageConfigured events.Type = "PoolStageConfigured"

// MaxPools is the most pools a stage can have.
const MaxPools = 16

// PoolStage is the first stage of a category: round-robin pools.
type PoolStage struct {
	Pools     int                 `json:"pools"`
	Win       engine.WinCondition `json:"win"`
	Tiebreaks []engine.Tiebreak   `json:"tiebreaks"`
}

// DefaultPoolStage is the stage of a new category: 2 pools, best of three
// sets of 21 capped at 30, default tiebreaks.
func DefaultPoolStage() PoolStage {
	return PoolStage{
		Pools:     2,
		Win:       engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 2, PointsPerSet: 21, Cap: 30},
		Tiebreaks: slices.Clone(engine.DefaultTiebreaks),
	}
}

// PoolStageConfigured is the payload of TypePoolStageConfigured.
type PoolStageConfigured struct {
	Category engine.Category `json:"category"`
	PoolStage
}

// PoolStage returns the pool stage of a category.
func (s State) PoolStage(c engine.Category) PoolStage {
	if st, ok := s.stages[c]; ok {
		return st
	}
	st := DefaultPoolStage()
	// Timed pools by default, as long as a single category plays: rounds
	// need every court.
	if s.defaultTimed && len(s.Categories) == 1 {
		st.Win = engine.WinCondition{Kind: engine.WinByTime, Minutes: DefaultTimedMinutes}
	}
	return st
}

// DefaultTimedMinutes is the duration of a timed match by default.
const DefaultTimedMinutes = 12

// ConfigurePoolStage sets the pool stage of a category. A timed win condition
// needs a single category. Changing the number of pools discards a draft
// draw, and is refused once the draw is confirmed.
func (s State) ConfigurePoolStage(c engine.Category, st PoolStage) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	if st.Pools < 1 || st.Pools > MaxPools || !validTiebreaks(st.Tiebreaks) {
		return Change{}, engine.NewError(engine.CodeInvalidFormat)
	}
	if err := st.Win.Validate(); err != nil {
		return Change{}, err
	}
	if st.Win.Kind == engine.WinByTime && len(s.Categories) > 1 {
		return Change{}, engine.NewError(engine.CodeTimedNeedsOneCategory)
	}
	if st.Win.Kind == engine.WinByTime {
		// Fields of the other kind are not stored.
		st.Win = engine.WinCondition{Kind: engine.WinByTime, Minutes: st.Win.Minutes}
	} else {
		st.Win.Minutes = 0
	}
	current := s.PoolStage(c)
	if d := s.draws[c]; d != nil && d.Confirmed && st.Pools != current.Pools {
		return Change{}, engine.NewError(engine.CodeDrawConfirmed)
	}
	if st.Pools == current.Pools && st.Win == current.Win && slices.Equal(st.Tiebreaks, current.Tiebreaks) {
		return Change{}, nil
	}
	st.Tiebreaks = slices.Clone(st.Tiebreaks)
	return Change{TypePoolStageConfigured, PoolStageConfigured{Category: c, PoolStage: st}}, nil
}

// validTiebreaks accepts a non-empty list of known criteria, each at most once.
func validTiebreaks(tbs []engine.Tiebreak) bool {
	if len(tbs) == 0 {
		return false
	}
	seen := map[engine.Tiebreak]bool{}
	for _, tb := range tbs {
		if seen[tb] || !slices.Contains(engine.DefaultTiebreaks, tb) {
			return false
		}
		seen[tb] = true
	}
	return true
}

// hasTimedStage reports whether a category plays timed matches.
// Only stages set in the format count: a timed default gives way to sets
// when a second category is added.
func (s State) hasTimedStage() bool {
	for _, c := range s.Categories {
		if st, ok := s.stages[c]; ok && st.Win.Kind == engine.WinByTime {
			return true
		}
		for _, b := range s.brackets[c] {
			if b.Win.Kind == engine.WinByTime {
				return true
			}
		}
	}
	return false
}

func (s *State) applyPoolStageConfigured(p PoolStageConfigured) {
	if s.stages == nil {
		s.stages = map[engine.Category]PoolStage{}
	}
	if d := s.draws[p.Category]; d != nil && !d.Confirmed && len(d.Pools) != p.Pools {
		delete(s.draws, p.Category)
	}
	s.stages[p.Category] = p.PoolStage
}
