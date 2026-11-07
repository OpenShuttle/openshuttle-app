package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotPath(t *testing.T) {
	dir := filepath.Join("some", "folder")
	got := SnapshotPath(filepath.Join(dir, "Open 50%"+TournamentExtension))
	if want := filepath.Join(dir, "Open 50%.html"); got != want {
		t.Fatalf("SnapshotPath = %q, want %q", got, want)
	}
}

func TestWriteSnapshot(t *testing.T) {
	f, path := newTournament(t)
	defer f.Close()
	for _, content := range []string{"<p>first</p>", "<p>second</p>"} {
		if err := WriteSnapshot(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(SnapshotPath(path))
		if err != nil || string(got) != content {
			t.Fatalf("snapshot = %q, %v; want %q", got, err, content)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("temporary file %s left behind", e.Name())
		}
	}
}

func TestRemoveTournamentRemovesSnapshot(t *testing.T) {
	f, path := newTournament(t)
	if err := WriteSnapshot(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := RemoveTournament(path); err != nil {
		t.Fatal(err)
	}
	if exists(t, SnapshotPath(path)) {
		t.Fatal("snapshot left behind")
	}
	// A file already gone takes its snapshot with it too.
	if err := WriteSnapshot(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := RemoveTournament(path); err != nil {
		t.Fatal(err)
	}
	if exists(t, SnapshotPath(path)) {
		t.Fatal("snapshot of a missing file left behind")
	}
}
