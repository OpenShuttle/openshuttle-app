package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestResultsExportAndArchive(t *testing.T) {
	f := newFixture(t)
	ctx := f.ctx
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(f.dir, "plume.openshuttle")
	must(f.svc.CreateTournamentAt(ctx, source, "Plume"))
	must(f.svc.AddCategory(ctx, engine.MensSingles))
	for i := range 3 {
		must(f.svc.RegisterEntrant(ctx, engine.MensSingles, []tournament.PlayerInput{
			{FirstName: "P", LastName: fmt.Sprint("N", i), Club: "C"},
		}))
	}
	if r := f.svc.GetResultsView(); r.Categories[0].Started {
		t.Fatal("started before the draw")
	}
	for _, row := range f.svc.GetCheckInView().Rows {
		must(f.svc.CheckInPlayer(ctx, row.ID))
	}
	must(f.editEntry(engine.MensSingles, func(stage *tournament.StageConfig) {
		stage.Pools = 1
	}))
	must(f.svc.DrawEntry(ctx, engine.MensSingles))
	must(f.svc.ConfirmDraw(ctx, engine.MensSingles))
	for _, m := range f.svc.GetPhasesView().Categories[0].Stages[0].Pools[0].Matches {
		score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
		if m.A.Entrant > m.B.Entrant {
			score = engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
		}
		must(f.svc.RecordScore(ctx, m.ID, score))
	}
	r := f.svc.GetResultsView().Categories[0]
	if !r.Complete || len(r.Rows) != 3 || r.Rows[0].Rank != 1 || r.Rows[2].Rank != 3 || r.Rows[0].StageRank != 1 || r.Rows[0].Stage != "S1" {
		t.Fatalf("results = %+v", r)
	}

	// CSV: cancelled, then written with a BOM and the .csv extension.
	if path, err := f.svc.ExportCSV("plume.csv", "a;b\n"); err != nil || path != "" {
		t.Fatalf("cancelled export = %q, %v", path, err)
	}
	f.dialogs.csv = filepath.Join(f.dir, "classement")
	path, err := f.svc.ExportCSV("plume.csv", "Rang;Nom\n1;Zola\n")
	must(err)
	data, _ := os.ReadFile(path)
	if !strings.HasSuffix(path, ".csv") || string(data) != utf8BOM+"Rang;Nom\n1;Zola\n" {
		t.Errorf("csv %s = %q", path, data)
	}

	// Archive: a copy that opens as a tournament, the file closed, the
	// recent entry flagged.
	f.dialogs.save = source
	wantCode(t, func() error { _, err := f.svc.ArchiveTournament(ctx); return err }(), engine.CodeFileExists)
	f.dialogs.save = filepath.Join(f.dir, "archives", "plume-final")
	must(os.MkdirAll(filepath.Join(f.dir, "archives"), 0o755))
	dest, err := f.svc.ArchiveTournament(ctx)
	must(err)
	if f.svc.GetTournamentView().Open {
		t.Error("still open after archive")
	}
	copyFile, err := store.OpenTournament(ctx, dest)
	must(err)
	copyFile.Close()
	recent, _ := f.svc.ListRecentTournaments(ctx)
	if len(recent) != 1 || !recent[0].Archived {
		t.Errorf("recent = %+v", recent)
	}
	// Reopening the original shows it archived, and it stays archived in the list.
	must(f.svc.OpenTournament(ctx, source))
	if !f.svc.GetTournamentView().Archived || !f.svc.GetResultsView().Archived {
		t.Error("archive flag lost")
	}
	recent, _ = f.svc.ListRecentTournaments(ctx)
	if !recent[0].Archived {
		t.Error("recent archive flag lost on reopen")
	}
}
