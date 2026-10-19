package tournament

import (
	"slices"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestScheduleChainedStages(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.SetCourts(6) })
	g.confirmedPools(engine.MensSingles, 3, 12)
	s2 := g.addStage(engine.MensSingles, StagePools, "Second", false)
	s3 := g.addStage(engine.MensSingles, StageBracket, "Final", true)
	g.editStage(engine.MensSingles, s2, func(st *StageConfig) { st.Pools, st.PoolSizes = 1, nil })
	g.route(engine.MensSingles, "S1", s2, s2)
	g.route(engine.MensSingles, s2, s3, s3)

	end := map[string]time.Time{}
	first := map[string]time.Time{}
	for _, sl := range g.state().Schedule(t0).Slots {
		end[sl.Stage] = later(end[sl.Stage], sl.End)
		if f, ok := first[sl.Stage]; !ok || sl.Start.Before(f) {
			first[sl.Stage] = sl.Start
		}
	}
	if first[s2].Before(end["S1"]) || first[s3].Before(end[s2]) {
		t.Errorf("S1 ends %v, S2 %v-%v, S3 starts %v", end["S1"], first[s2], end[s2], first[s3])
	}
}

func TestSamePhaseStagesInterleave(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	main := g.addStage(engine.MensSingles, StageBracket, "Main", true)
	others := g.addStage(engine.MensSingles, StagePools, "Others", false)
	g.editStage(engine.MensSingles, others, func(st *StageConfig) { st.Pools, st.PoolSizes = 1, nil })
	g.route(engine.MensSingles, "S1", main, main, others, others)
	g.finishPools(engine.MensSingles)
	g.fill(engine.MensSingles, main)
	g.fill(engine.MensSingles, others)

	// Main and Others are both phase 2: round 1 of each, then round 2 of each…
	var blocks []string
	for _, id := range g.state().AutoOrder() {
		m, _ := g.state().Match(id)
		if k := g.state().MatchBlock(m); len(blocks) == 0 || blocks[len(blocks)-1] != k {
			blocks = append(blocks, k)
		}
	}
	want := []string{"MS/" + main + "/R1", "MS/" + others + "/A/1", "MS/" + others + "/A/2", "MS/" + main + "/R2"}
	if len(blocks) < 4 || !slices.Equal(blocks[:4], want) {
		t.Errorf("blocks = %v, want %v…", blocks, want)
	}
	// Bracket rounds carry their size: semi-finals, then finals.
	for _, sl := range g.state().Schedule(t0).Slots {
		if sl.Stage == main && sl.RoundSize != 4>>sl.Level {
			t.Errorf("slot %s level %d round size %d", sl.Match, sl.Level, sl.RoundSize)
		}
	}
}
