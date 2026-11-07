package tournament

import (
	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// TypeStageDisplaySet shows or hides a stage on the display screen.
const TypeStageDisplaySet events.Type = "StageDisplaySet"

// StageDisplaySet is the payload of TypeStageDisplaySet. A stage the desk
// never set is shown once it started.
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
	if s.displayed == nil {
		s.displayed = map[string]bool{}
	}
	s.displayed[string(p.Category)+"/"+p.Stage] = !p.Hidden
}

// ShownOnDisplay reports whether the display screen shows a stage: as the
// desk set it, else once a match of the stage started.
func (s State) ShownOnDisplay(c engine.Category, stage string) bool {
	if shown, ok := s.displayed[string(c)+"/"+stage]; ok {
		return shown
	}
	return s.StageStarted(c, stage)
}

// StageRef names a stage of a category.
type StageRef struct {
	Category engine.Category
	Stage    string
}

// DisplayStages lists the stages the display screen rotates through: the
// stages shown on it and not over; else, once every one is over, the shown
// ones (the final bracket stays on screen). Categories in order, then stages
// in order.
func (s State) DisplayStages() []StageRef {
	var playing, over []StageRef
	for _, c := range s.Categories {
		for _, st := range s.Stages(c) {
			if !st.Confirmed || !s.ShownOnDisplay(c, st.ID) {
				continue
			}
			if s.StageComplete(c, st.ID) {
				over = append(over, StageRef{c, st.ID})
			} else {
				playing = append(playing, StageRef{c, st.ID})
			}
		}
	}
	if len(playing) > 0 {
		return playing
	}
	return over
}
