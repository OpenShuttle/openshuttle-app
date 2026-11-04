package main

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestClubBadges(t *testing.T) {
	colors := map[string]int{"Lyon": 0, "Caen": 1, "Albi": 12}
	mate := func(clubs ...string) tournament.Entrant {
		e := tournament.Entrant{}
		for _, c := range clubs {
			e.Players = append(e.Players, tournament.Player{Club: c})
		}
		return e
	}
	tests := []struct {
		name   string
		clubs  []string
		inPool bool
		mates  []tournament.Entrant
		want   []ClubBadge
	}{
		{"unclubbed", nil, true, nil, []ClubBadge{}},
		{"outside a pool", []string{"Lyon"}, false, nil, []ClubBadge{{"Lyon", 1, 0}}},
		{"alone in its pool", []string{"Caen"}, true, nil, []ClubBadge{{"Caen", 2, 1}}},
		{"palette wraps", []string{"Albi"}, true, nil, []ClubBadge{{"Albi", 3, 1}}},
		{"two mates", []string{"Lyon"}, true, []tournament.Entrant{mate("Lyon"), mate("Lyon", "Caen")}, []ClubBadge{{"Lyon", 1, 3}}},
		{"mixed pair, one club shared", []string{"Lyon", "Caen"}, true, []tournament.Entrant{mate("Caen", "Caen")}, []ClubBadge{{"Lyon", 1, 1}, {"Caen", 2, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clubBadges(tt.clubs, colors, tt.inPool, tt.mates); !slices.Equal(got, tt.want) {
				t.Errorf("clubBadges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTier(t *testing.T) {
	tests := []struct{ seed, pools, want int }{
		{0, 5, 0}, {1, 0, 0}, {1, 5, 1}, {5, 5, 1}, {6, 5, 2}, {10, 5, 2}, {11, 5, 3}, {3, 1, 3},
	}
	for _, tt := range tests {
		if got := tier(tt.seed, tt.pools); got != tt.want {
			t.Errorf("tier(%d, %d) = %d, want %d", tt.seed, tt.pools, got, tt.want)
		}
	}
}

func TestFormatAndDraw(t *testing.T) {
	f := newFixture(t)
	f.svc.rng = func() *rand.Rand { return rand.New(rand.NewPCG(3, 4)) }
	ctx := f.ctx
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "t.openshuttle"), "Plume"))
	must(f.svc.AddCategory(ctx, engine.MensSingles))

	for i := range 7 {
		club := []string{"Lyon", "Paris"}[i%2]
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{
			{Name: fmt.Sprint("P N", i), Club: club},
		}))
	}

	fv := f.svc.GetFormatView()
	fc := fv.Categories[0]
	if !fc.TimedAllowed || reg(fc).Entrants != 7 || previewOf(fc, PreviewPresent).Entrants != 0 || reg(fc).Stages[0].Matches != 6+3 || len(reg(fc).Stages[0].Sizes) != 2 {
		t.Fatalf("format = %+v", fc)
	}

	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools = 3 }))
	if fc = f.svc.GetFormatView().Categories[0]; reg(fc).Stages[0].Pools != 3 || reg(fc).Stages[0].Matches != 3+1+1 {
		t.Errorf("after 3 pools = %+v", fc)
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) { stage.Pools = 2 }))

	// Nobody present: cannot draw.
	dv := f.svc.GetDrawView().Categories[0]
	if dv.Status != DrawNone || dv.CanDraw {
		t.Fatalf("draw before check-in = %+v", dv)
	}
	wantCode(t, f.svc.DrawEntry(ctx, engine.MensSingles), engine.CodeNotEnoughEntrants)

	for _, team := range f.svc.GetCheckInView().Teams {
		must(f.svc.CheckInEntrant(ctx, team.EntrantID))
	}
	if dv = f.svc.GetDrawView().Categories[0]; !dv.CanDraw || dv.Planned != 2 || len(dv.Unseeded) != 7 || len(dv.Ranking) != 0 {
		t.Fatalf("draw view after check-in = %+v", dv)
	}
	first, second := dv.Unseeded[3].ID, dv.Unseeded[5].ID
	must(f.svc.SetSeeds(ctx, engine.MensSingles, []engine.EntrantID{second, first}))
	must(f.svc.SetSeeds(ctx, engine.MensSingles, []engine.EntrantID{first, second}))
	if dv = f.svc.GetDrawView().Categories[0]; len(dv.Ranking) != 2 || dv.Ranking[0].ID != first || len(dv.Unseeded) != 5 {
		t.Fatalf("seeds = %+v", dv.Ranking)
	}
	must(f.svc.DrawEntry(ctx, engine.MensSingles))

	dv = f.svc.GetDrawView().Categories[0]
	if dv.Status != DrawDraft || len(dv.Pools) != 2 || dv.Pools[0].Name != "A" || len(dv.Pools[0].Entrants)+len(dv.Pools[1].Entrants) != 7 {
		t.Fatalf("draw = %+v", dv)
	}
	if dv.Ranking[1].Tier != 1 || dv.Unseeded[0].Tier != 0 {
		t.Errorf("levels: rank 2 of 2 pools = %d, unranked = %d", dv.Ranking[1].Tier, dv.Unseeded[0].Tier)
	}
	if dv.Pools[0].Entrants[0].ID != first || dv.Pools[0].Entrants[0].Seed != 1 {
		t.Errorf("top seed should open pool A: %+v", dv.Pools[0].Entrants[0])
	}
	// 4 Lyon and 3 Paris in 2 pools: same-club pairs are unavoidable and reported.
	conflicts := 0
	for _, p := range dv.Pools {
		for _, e := range p.Entrants {
			conflicts += len(e.SameClub)
		}
	}
	if conflicts == 0 {
		t.Error("same-club pairs not reported")
	}
	// Club badges: Lyon registered first takes color 1, Paris color 2; in a
	// pool, each badge counts the entrants holding the club.
	for _, p := range dv.Pools {
		held := map[string]int{}
		for _, e := range p.Entrants {
			held[e.Clubs[0].Name]++
		}
		for _, e := range p.Entrants {
			b := e.Clubs[0]
			if want := map[string]int{"Lyon": 1, "Paris": 2}[b.Name]; b.Color != want || b.Count != held[b.Name] {
				t.Errorf("pool %s badge = %+v, want color %d count %d", p.Name, b, want, held[b.Name])
			}
		}
	}

	moved := dv.Pools[1].Entrants[0].ID
	must(f.svc.MoveEntrant(ctx, moved, 0))
	if a := f.svc.GetTournamentView().LastAction; a.Type != "EntrantMoved" || a.Params["pool"] != "A" || a.Params["names"] == "" {
		t.Errorf("move action = %+v", a)
	}
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))
	if dv = f.svc.GetDrawView().Categories[0]; dv.Status != DrawConfirmed {
		t.Errorf("status = %s", dv.Status)
	}
	if fc = f.svc.GetFormatView().Categories[0]; !reg(fc).Stages[0].Confirmed {
		t.Error("pools should be locked")
	}
	must(f.svc.Undo(ctx))
	if dv = f.svc.GetDrawView().Categories[0]; dv.Status != DrawDraft {
		t.Errorf("status after undo = %s", dv.Status)
	}
}
