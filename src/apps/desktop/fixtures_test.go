package main

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
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
		if tv, ok := v.(TournamentView); ok {
			tv.Path = "/home/julie/Documents/OpenShuttle/" + filepath.Base(tv.Path) // not the test's temp dir
			v = tv
		}
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
	d.must(nil)
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
	p.must(nil)
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
	f.must(nil)
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
	*r.clock = r.clock.Add(4 * time.Minute)
	r.must(nil)
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

// demo is a tournament built step by step with a fixed clock, by the
// sample builder.
type demo struct {
	*sampleBuilder
	t   *testing.T
	app *store.AppDB
}

func newDemo(t *testing.T, name string, courts int) *demo {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	app, err := store.OpenAppDB(ctx, filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	d := &demo{t: t, app: app, sampleBuilder: &sampleBuilder{ctx: ctx, clock: &clock}}
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
	*d.clock = d.clock.Add(time.Second)
	return *d.clock
}

func (d *demo) must(err error) {
	d.t.Helper()
	if err == nil {
		err = d.err
	}
	if err != nil {
		d.t.Fatal(err)
	}
}
