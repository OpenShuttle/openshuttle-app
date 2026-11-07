package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

func TestWriteSnapshot(t *testing.T) {
	f := newFixture(t)
	// No tournament open: nothing to write.
	if err := f.svc.WriteSnapshot(0, "<p>none</p>"); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(f.dir, "Plume.openshuttle")
	if err := f.svc.CreateTournamentAt(f.ctx, path, "Plume"); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(f.dir, "Plume.html")
	read := func() string {
		t.Helper()
		b, err := os.ReadFile(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	v := f.svc.GetTournamentView().Version
	if err := f.svc.WriteSnapshot(v, "<p>created</p>"); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "<p>created</p>" {
		t.Fatalf("snapshot = %q", got)
	}

	// A snapshot built from an older version is skipped.
	if err := f.svc.RenameTournament(f.ctx, "Plume 2"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.WriteSnapshot(v, "<p>outdated</p>"); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "<p>created</p>" {
		t.Fatalf("outdated snapshot written: %q", got)
	}
	if err := f.svc.WriteSnapshot(f.svc.GetTournamentView().Version, "<p>renamed</p>"); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "<p>renamed</p>" {
		t.Fatalf("snapshot = %q", got)
	}

	// Deleting the file from the trash takes the snapshot with it.
	if err := f.svc.TrashTournament(f.ctx, path); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteTournament(f.ctx, path, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.SnapshotPath(path)); !os.IsNotExist(err) {
		t.Fatalf("snapshot left behind: %v", err)
	}
}
