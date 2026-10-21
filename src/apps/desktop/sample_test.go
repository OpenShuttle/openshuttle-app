package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestCreateSampleTournament(t *testing.T) {
	f := newBareFixture(t)
	dir := filepath.Join(f.dir, "Tournois")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f.dialogs.folder = dir
	if _, err := f.settings.ChooseTournamentsDir(f.ctx); err != nil {
		t.Fatal(err)
	}

	path, err := f.svc.CreateSampleTournament(f.ctx, "Tournoi d'exemple")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "Tournoi d'exemple.openshuttle") {
		t.Errorf("path = %s", path)
	}
	v := f.svc.GetTournamentView()
	if !v.Open || v.Name != "Tournoi d'exemple" || len(v.Categories) != 2 || !v.Progress.Started {
		t.Fatalf("tournament = %+v", v)
	}
	desk := f.svc.GetControlDeskView()
	playing, pending := 0, len(desk.Pending)
	for _, c := range desk.Courts {
		if c.Match != nil && c.State == "playing" {
			playing++
			if since := f.clock.Sub(*c.Match.CalledAt); since <= 0 || since > 30*60e9 {
				t.Errorf("court %d playing for %v", c.Number, since)
			}
		}
	}
	if playing == 0 || pending != 1 || desk.Courts[0].Match == nil {
		t.Errorf("desk: %d playing, %d pending", playing, pending)
	}
	if r := f.svc.GetResultsView(); len(r.Categories) != 2 || r.Categories[0].Category != engine.MensDoubles {
		t.Errorf("results = %+v", r.Categories)
	}

	// A second one gets a number; a bad name is refused.
	again, err := f.svc.CreateSampleTournament(f.ctx, "Tournoi d'exemple")
	if err != nil || filepath.Base(again) != "Tournoi d'exemple (2).openshuttle" {
		t.Errorf("second sample = %s, %v", again, err)
	}
	wantCode(t, func() error { _, err := f.svc.CreateSampleTournament(f.ctx, "  "); return err }(), engine.CodeInvalidName)
	if _, err := os.Stat(again); err != nil {
		t.Error(err)
	}
}
