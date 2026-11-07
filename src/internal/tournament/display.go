package tournament

import (
	"cmp"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// TypeStageDisplaySet shows or hides a stage on the display screen.
const TypeStageDisplaySet events.Type = "StageDisplaySet"

// StageDisplaySet is the payload of TypeStageDisplaySet. Every stage is shown
// until the desk hides it.
type StageDisplaySet struct {
	Category engine.Category `json:"category"`
	Stage    string          `json:"stage"`
	Hidden   bool            `json:"hidden"`
}

// SetStageDisplay shows or hides a stage on the display screen.
func (s State) SetStageDisplay(c engine.Category, stage string, shown bool) (Change, error) {
	if s.stage(c, stage) == nil {
		return Change{}, engine.NewError(engine.CodeStageNotFound)
	}
	if s.ShownOnDisplay(c, stage) == shown {
		return Change{}, nil
	}
	return Change{TypeStageDisplaySet, StageDisplaySet{Category: c, Stage: stage, Hidden: !shown}}, nil
}

func (s *State) applyStageDisplaySet(p StageDisplaySet) {
	if s.hiddenOnDisplay == nil {
		s.hiddenOnDisplay = map[string]bool{}
	}
	s.hiddenOnDisplay[string(p.Category)+"/"+p.Stage] = p.Hidden
}

// ShownOnDisplay reports whether the display screen may show a stage.
func (s State) ShownOnDisplay(c engine.Category, stage string) bool {
	return !s.hiddenOnDisplay[string(c)+"/"+stage]
}

// StageRef names a stage of a category.
type StageRef struct {
	Category engine.Category
	Stage    string
}

// DisplayStages lists the stages the display screen rotates through, among
// the stages shown on it: the ones being played; else the ones drawn and
// about to start; else the last one over of each category. Categories in
// order, then stages in order.
func (s State) DisplayStages() []StageRef {
	var playing, ready, over []StageRef
	for _, c := range s.Categories {
		var last *StageRef
		for _, st := range s.Stages(c) {
			if !st.Confirmed || !s.ShownOnDisplay(c, st.ID) {
				continue
			}
			ref := StageRef{c, st.ID}
			switch {
			case s.StageComplete(c, st.ID):
				last = &ref
			case s.StageStarted(c, st.ID):
				playing = append(playing, ref)
			default:
				ready = append(ready, ref)
			}
		}
		if last != nil {
			over = append(over, *last)
		}
	}
	switch {
	case len(playing) > 0:
		return playing
	case len(ready) > 0:
		return ready
	}
	return over
}

// LatestScores returns the matches whose score was entered last, newest
// first, at most n.
func (s State) LatestScores(n int) []Match {
	var scored []Match
	for _, m := range s.matches {
		if m.Score != nil && m.scoreSeq > 0 {
			scored = append(scored, m)
		}
	}
	slices.SortFunc(scored, func(a, b Match) int { return cmp.Compare(b.scoreSeq, a.scoreSeq) })
	return scored[:min(n, len(scored))]
}
