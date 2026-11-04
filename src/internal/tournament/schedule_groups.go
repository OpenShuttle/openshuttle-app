package tournament

import (
	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament/blocks"
)

// GroupColors is the size of the palette of pools and brackets on the
// schedule (--color-group-1..n): indexes wrap around past it.
const GroupColors = 8

// GroupStyle is how the schedule tells a pool or a bracket apart: its color
// index (1..GroupColors) and, for a pool, how many rounds it plays.
type GroupStyle struct {
	Color  int
	Rounds int
}

// GroupStyles gives each pool ("MD/S1/A", blocks.PoolGroup) and each bracket
// stage ("MD/S2") of every category its style. Colors follow the format in
// each category: the pools of the first stage in order, then each later
// stage's pools, or the bracket as one; a stage not filled yet counts the
// pools its preview plans.
func (s State) GroupStyles() map[string]GroupStyle {
	out := map[string]GroupStyle{}
	for _, c := range s.Categories {
		stages := s.Stages(c)
		var preview FormatPreview
		previewed := false
		n := 0
		next := func() int {
			n++
			return (n-1)%GroupColors + 1
		}
		for i, st := range stages {
			if st.Kind != StagePools {
				out[string(c)+"/"+st.ID] = GroupStyle{Color: next()}
				continue
			}
			sizes := groupSizes(st.Groups)
			if !st.Filled() {
				if !previewed {
					preview = PreviewFormat(s.StageConfigs(c), s.expectedEntrants(c), s.filledSizes(c))
					previewed = true
				}
				sizes = nil
				if i < len(preview.Stages) {
					sizes = preview.Stages[i].Sizes
				}
			}
			for pool, size := range sizes {
				out[blocks.PoolGroup(c, st.ID, pool)] = GroupStyle{Color: next(), Rounds: len(engine.RoundRobinSchedule(size))}
			}
		}
	}
	return out
}

// GroupStyleOf is the style of a pool (pool ≥ 0) or of a bracket stage.
func GroupStyleOf(styles map[string]GroupStyle, c engine.Category, stage string, pool int) GroupStyle {
	if pool >= 0 {
		return styles[blocks.PoolGroup(c, stage, pool)]
	}
	return styles[string(c)+"/"+stage]
}
