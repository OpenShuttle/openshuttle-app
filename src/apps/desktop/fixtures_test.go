package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// TestWriteFrontendFixtures writes the views of a few demo tournaments as
// JSON, for the frontend's component tests (frontend/src/testing/fixtures).
// It only runs when asked:
//
//	OPENSHUTTLE_FIXTURES=$PWD/frontend/src/testing/fixtures go test -run TestWriteFrontendFixtures .
//
// The clock is fixed and the draw seeded, so the output only changes when
// the views do. Regenerate after changing a view.
func TestWriteFrontendFixtures(t *testing.T) {
	dir := os.Getenv("OPENSHUTTLE_FIXTURES")
	if dir == "" {
		t.Skip("OPENSHUTTLE_FIXTURES not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, v any) {
		t.Helper()
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".json"), append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()

	// Before the draw: entrants registered, some checked in, seeds ranked.
	d := newDemo(t, "Tournoi de printemps", 4)
	d.entrants(engine.MensDoubles, 12)
	d.entrants(engine.WomensDoubles, 8)
	d.format(engine.MensDoubles, 3, true)
	d.format(engine.WomensDoubles, 2, true)
	for i, team := range d.svc.GetCheckInView().Teams {
		switch {
		case i%5 == 4:
			// absent
		case i%5 == 3:
			d.must(d.svc.CheckInPlayer(ctx, team.Players[0].ID)) // half a pair
		default:
			d.must(d.svc.CheckInEntrant(ctx, team.EntrantID))
		}
	}
	md := d.svc.GetDrawView().Categories[0]
	d.must(d.svc.SetSeeds(ctx, engine.MensDoubles, []engine.EntrantID{md.Unseeded[0].ID, md.Unseeded[1].ID}))
	d.must(d.svc.DrawEntry(ctx, engine.MensDoubles))
	write("before-tournament", d.svc.GetTournamentView())
	write("before-entrants", d.svc.GetEntrantsView())
	write("before-checkin", d.svc.GetCheckInView())
	write("before-draw", d.svc.GetDrawView())
	write("before-format", d.svc.GetFormatView())
	write("before-schedule", d.svc.GetScheduleView())
	write("before-desk", d.svc.GetControlDeskView())
	write("before-results", d.svc.GetResultsView())

	// In progress: pools half played, courts busy, a score pending.
	p := newDemo(t, "Tournoi d'automne", 7)
	p.entrants(engine.MensDoubles, 12)
	p.entrants(engine.WomensDoubles, 8)
	p.format(engine.MensDoubles, 3, true)
	p.format(engine.WomensDoubles, 2, true)
	p.checkInAll()
	p.draw(engine.MensDoubles, engine.WomensDoubles)
	p.play(2)
	p.callAll()
	p.endOne()
	write("progress-tournament", p.svc.GetTournamentView())
	write("progress-desk", p.svc.GetControlDeskView())
	write("progress-display", p.svc.GetDisplayView())
	write("progress-schedule", p.svc.GetScheduleView())
	write("progress-phases", p.svc.GetPhasesView())
	write("progress-history", p.svc.GetHistoryView())
	write("progress-results", p.svc.GetResultsView())
	write("progress-format", p.svc.GetFormatView())
	p.must(p.svc.ValidateSchedule(ctx))
	write("progress-schedule-validated", p.svc.GetScheduleView())

	// Finished: every match played, brackets filled, full standings.
	f := newDemo(t, "Tournoi d'été", 7)
	f.entrants(engine.MensDoubles, 12)
	f.entrants(engine.WomensDoubles, 8)
	f.format(engine.MensDoubles, 3, true)
	f.format(engine.WomensDoubles, 2, true)
	f.checkInAll()
	f.draw(engine.MensDoubles, engine.WomensDoubles)
	f.play(60)
	write("finished-tournament", f.svc.GetTournamentView())
	write("finished-phases", f.svc.GetPhasesView())
	write("finished-results", f.svc.GetResultsView())
	write("finished-desk", f.svc.GetControlDeskView())
	write("finished-display", f.svc.GetDisplayView())

	// Timed rounds: one category, timed pools, a round on court.
	r := newDemo(t, "Tournoi au temps", 4)
	r.entrants(engine.MixedDoubles, 8)
	stages := r.stages(engine.MixedDoubles)
	stages[0].Pools, stages[0].PoolSizes = 2, nil
	stages[0].Win = engine.WinCondition{Kind: engine.WinByTime, Minutes: 10}
	r.must(r.svc.SetFormat(ctx, engine.MixedDoubles, stages))
	r.checkInAll()
	r.draw(engine.MixedDoubles)
	r.must(r.svc.StartRound(ctx))
	r.clock = r.clock.Add(4 * time.Minute)
	write("rounds-desk", r.svc.GetControlDeskView())
	write("rounds-display", r.svc.GetDisplayView())

	// App-wide: presets, settings, recent files.
	presets, err := NewPresetService(f.app, func(string, any) {}, f.now).GetPresets(ctx)
	f.must(err)
	write("presets", presets)
	settings, err := NewSettingsService(f.app, func(string, any) {}, nil).GetSettings(ctx)
	f.must(err)
	settings.TournamentsDir = "/home/julie/Documents/OpenShuttle"
	write("settings", settings)
	recent, err := f.svc.ListRecentTournaments(ctx)
	f.must(err)
	for i := range recent {
		recent[i].Path = "/home/julie/Documents/OpenShuttle/" + filepath.Base(recent[i].Path)
	}
	write("recent", recent)
	preview, err := NewPresetService(f.app, func(string, any) {}, f.now).PreviewFormat(f.stages(engine.MensDoubles), 22)
	f.must(err)
	write("preview", preview)
}

// demo is a tournament built step by step with a fixed clock.
type demo struct {
	t     *testing.T
	ctx   context.Context
	app   *store.AppDB
	svc   *TournamentService
	clock time.Time
}

func newDemo(t *testing.T, name string, courts int) *demo {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	app, err := store.OpenAppDB(ctx, filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	d := &demo{t: t, ctx: ctx, app: app, clock: time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)}
	d.svc = NewTournamentService(app, func(string, any) {}, d.now, filepath.Join(dir, "backups"), nil)
	d.svc.rng = func() *rand.Rand { return rand.New(rand.NewPCG(7, 11)) }
	t.Cleanup(func() {
		d.svc.ServiceShutdown()
		app.Close()
	})
	d.must(d.svc.CreateTournamentAt(ctx, filepath.Join(dir, name+".openshuttle"), name))
	d.must(d.svc.SetCourts(ctx, courts))
	return d
}

// now ticks a second per call, so every event has its own time.
func (d *demo) now() time.Time {
	d.clock = d.clock.Add(time.Second)
	return d.clock
}

func (d *demo) must(err error) {
	d.t.Helper()
	if err != nil {
		d.t.Fatal(err)
	}
}

var (
	demoClubs = []string{"Volant des Monts", "BC Rive Droite", "Plumes du Lac", "AS Belvédère"}
	demoFirst = []string{"Lucas", "Hugo", "Paul", "Jules", "Louis", "Théo", "Nathan", "Simon", "Victor", "Kevin", "Mathis", "Félix",
		"Camille", "Léa", "Jade", "Manon", "Inès", "Chloé", "Emma", "Clara", "Alice", "Océane", "Élise", "Lou"}
	demoLast = []string{"Perrin", "Faure", "Blanc", "Masson", "Caron", "Mathieu", "Lopez", "Vidal", "Gauthier", "Brun", "Fontaine", "Simon",
		"Moreau", "Roux", "Fournier", "Morin", "Guérin", "Henry", "Lambert", "Bertrand", "Laurent", "Dumont", "Clément", "André"}
)

// entrants registers n pairs of a category.
func (d *demo) entrants(c engine.Category, n int) {
	d.must(d.svc.AddCategory(d.ctx, c))
	women := c == engine.WomensDoubles || c == engine.WomensSingles
	for i := range n {
		k := len(d.svc.GetCheckInView().Teams) * 2 % 12
		if women {
			k += 12
		}
		player := func(j int) tournament.PlayerInput {
			return tournament.PlayerInput{
				Name: fmt.Sprintf("%s %s", demoFirst[(k+j)%len(demoFirst)], demoLast[(k+j*5+i)%len(demoLast)]),
				Club: demoClubs[i%len(demoClubs)],
			}
		}
		d.must(d.svc.RegisterEntrant(d.ctx, c, []tournament.PlayerInput{player(0), player(1)}))
	}
}

func (d *demo) stages(c engine.Category) []tournament.StageConfig {
	var stages []tournament.StageConfig
	for _, fc := range d.svc.GetFormatView().Categories {
		if fc.Category == c {
			for _, st := range fc.Stages {
				stages = append(stages, st.StageConfig)
			}
		}
	}
	return stages
}

// format sets pools, then a full-placement bracket for the first two of each.
func (d *demo) format(c engine.Category, pools int, bracket bool) {
	if bracket {
		d.must(d.svc.AddStage(d.ctx, c, tournament.StageBracket))
	}
	stages := d.stages(c)
	win := engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 2, PointsPerSet: 15, Cap: 21}
	stages[0].Pools, stages[0].PoolSizes, stages[0].Win = pools, nil, win
	if bracket {
		stages[0].Routing = tournament.RanksRules("S2", "S2")
		stages[1].Name, stages[1].Win, stages[1].FullPlacement = "Tableau principal", win, true
	}
	d.must(d.svc.SetFormat(d.ctx, c, stages))
}

func (d *demo) checkInAll() {
	for _, team := range d.svc.GetCheckInView().Teams {
		d.must(d.svc.CheckInEntrant(d.ctx, team.EntrantID))
	}
}

func (d *demo) draw(categories ...engine.Category) {
	for _, c := range categories {
		d.must(d.svc.DrawEntry(d.ctx, c))
		d.must(d.svc.ConfirmDraw(d.ctx, c))
	}
}

var demoScores = []engine.Score{
	{Sets: []engine.SetScore{{A: 15, B: 11}, {A: 15, B: 9}}},
	{Sets: []engine.SetScore{{A: 13, B: 15}, {A: 15, B: 12}, {A: 15, B: 10}}},
	{Sets: []engine.SetScore{{A: 9, B: 15}, {A: 12, B: 15}}},
	{Sets: []engine.SetScore{{A: 17, B: 15}, {A: 15, B: 8}}},
}

// play runs up to n rounds of calls: every proposal called, ended and
// scored; brackets are filled once their pools are over.
func (d *demo) play(n int) {
	scored := 0
	for range n {
		called := d.callAll()
		if len(called) == 0 {
			filled := false
			for _, c := range d.svc.GetTournamentView().Categories {
				if d.svc.FillStage(d.ctx, c.Category, "S2") == nil {
					filled = true
				}
			}
			if !filled {
				return
			}
			continue
		}
		d.clock = d.clock.Add(22 * time.Minute)
		for _, id := range called {
			d.must(d.svc.EndMatch(d.ctx, id))
			d.must(d.svc.RecordScore(d.ctx, id, demoScores[scored%len(demoScores)]))
			scored++
		}
		d.clock = d.clock.Add(3 * time.Minute)
	}
}

// callAll calls every proposal of the desk.
func (d *demo) callAll() []tournament.MatchID {
	var called []tournament.MatchID
	for _, c := range d.svc.GetControlDeskView().Courts {
		if c.Proposal != nil && d.svc.CallMatch(d.ctx, c.Proposal.ID, c.Number) == nil {
			called = append(called, c.Proposal.ID)
		}
	}
	d.clock = d.clock.Add(9 * time.Minute)
	return called
}

// endOne ends the first match on court, its score left pending.
func (d *demo) endOne() {
	for _, c := range d.svc.GetControlDeskView().Courts {
		if c.Match != nil {
			d.must(d.svc.EndMatch(d.ctx, c.Match.ID))
			return
		}
	}
}
