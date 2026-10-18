package tournament

import (
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestPreviewFormat(t *testing.T) {
	stages := []StageConfig{
		{ID: "S1", Kind: StagePools, PoolSizes: []int{4, 5}, Win: sets21, Tiebreaks: engine.DefaultTiebreaks, Routing: []RouteRule{
			{Kind: RouteRanks, To: "S2", Ranks: []int{1, 2, 3}},
			{Kind: RouteBestOfRank, To: "S2", Rank: 4, Count: 1},
			{Kind: RouteRest, To: "S3"},
		}},
		{ID: "S2", Kind: StageBracket, Win: sets21, FullPlacement: true},
		{ID: "S3", Kind: StageBracket, Win: sets21},
	}
	p := PreviewFormat(stages, 22, nil)
	pools, main, cons := p.Stages[0], p.Stages[1], p.Stages[2]
	if !slices.Equal(pools.Sizes, []int{5, 5, 4, 4, 4}) || pools.Matches != 10+10+6+6+6 || pools.Rounds != 5 {
		t.Errorf("pools = %+v", pools)
	}
	if !slices.Equal(pools.Rules, []int{15, 1, 6}) || pools.Kept != 0 || pools.PlaceFrom != 0 {
		t.Errorf("routing = %v, kept %d", pools.Rules, pools.Kept)
	}
	if main.Entrants != 16 || main.Byes != 0 || main.Matches != 32 || main.Phase != 2 || main.PlaceFrom != 1 || main.PlaceTo != 16 {
		t.Errorf("main = %+v", main)
	}
	// Consolation of 6 without full placement: 1, 2, 3-4 tied, 5-6 tied.
	if cons.Entrants != 6 || cons.Byes != 2 || cons.PlaceFrom != 17 || cons.PlaceTo != 22 {
		t.Errorf("consolation = %+v", cons)
	}
	if p.Ranked != 22 || !p.Ties {
		t.Errorf("ranked %d, ties %v", p.Ranked, p.Ties)
	}

	// No split possible: flagged, nothing goes on.
	p = PreviewFormat([]StageConfig{{ID: "S1", Kind: StagePools, PoolSizes: []int{4}, Tiebreaks: engine.DefaultTiebreaks}}, 5, nil)
	if !p.Stages[0].NoSplit || p.Ranked != 0 {
		t.Errorf("no split = %+v", p)
	}

	// Actual sizes of a filled stage win over the plan.
	p = PreviewFormat(stages, 22, map[string][]int{"S1": {4, 4, 4, 4, 3, 3}})
	if p.Stages[0].Entrants != 22 || !slices.Equal(p.Stages[0].Rules, []int{18, 1, 3}) {
		t.Errorf("actual = %+v", p.Stages[0])
	}
}
