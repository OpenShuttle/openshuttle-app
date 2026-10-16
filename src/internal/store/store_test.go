package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

var t0 = time.Date(2026, 10, 17, 9, 30, 0, 123456789, time.UTC)

func wantCode(t *testing.T, err error, code engine.Code) {
	t.Helper()
	var e *engine.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func newTournament(t *testing.T) (*TournamentFile, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plume-automne-2026"+TournamentExtension)
	f, err := CreateTournament(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f, path
}

func draft(t *testing.T, typ events.Type, data any, at time.Time) events.Draft {
	t.Helper()
	d, err := events.NewDraft(typ, data, at, "Julie")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestTournamentAppendAndReopen(t *testing.T) {
	ctx := context.Background()
	f, path := newTournament(t)

	for i := range 3 {
		e, err := f.Append(ctx, draft(t, "ScoreRecorded", map[string]int{"i": i}, t0.Add(time.Duration(i)*time.Minute)))
		if err != nil {
			t.Fatal(err)
		}
		if e.Seq != uint64(i+1) {
			t.Fatalf("seq = %d, want %d", e.Seq, i+1)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := OpenTournament(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	log, err := f.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 3 {
		t.Fatalf("got %d events, want 3", len(log))
	}
	last := log[2]
	if last.Seq != 3 || last.Type != "ScoreRecorded" || string(last.Data) != `{"i":2}` ||
		last.Operator != "Julie" || !last.At.Equal(t0.Add(2*time.Minute)) {
		t.Errorf("last event = %+v", last)
	}

	since, err := f.EventsSince(ctx, 1)
	if err != nil || len(since) != 2 || since[0].Seq != 2 {
		t.Errorf("EventsSince(1) = %v, %v", since, err)
	}
	if v, err := f.Version(ctx); err != nil || v != 3 {
		t.Errorf("Version() = %d, %v, want 3", v, err)
	}
}

func TestTournamentUndoRoundTrip(t *testing.T) {
	ctx := context.Background()
	f, _ := newTournament(t)
	if _, err := f.Append(ctx, draft(t, "ScoreRecorded", map[string]int{}, t0)); err != nil {
		t.Fatal(err)
	}
	log, _ := f.Events(ctx)
	undo, err := events.UndoLast(log, nil, t0.Add(time.Minute), "Julie")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Append(ctx, undo); err != nil {
		t.Fatal(err)
	}
	log, _ = f.Events(ctx)
	if len(log) != 2 || len(events.Active(log)) != 0 {
		t.Errorf("log %d events, %d active; want 2 and 0", len(log), len(events.Active(log)))
	}
}

func TestTournamentOddFileName(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "Tournoi #2 été 100%"+TournamentExtension)
	f, err := CreateTournament(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created at the exact path: %v", err)
	}
	f, err = OpenTournament(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestTournamentPragmas(t *testing.T) {
	f, _ := newTournament(t)
	var mode string
	var sync int
	if err := f.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q, %v", mode, err)
	}
	if err := f.db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil || sync != 2 {
		t.Errorf("synchronous = %d, %v, want 2 (FULL)", sync, err)
	}
}

func TestTournamentAppendInvalid(t *testing.T) {
	f, _ := newTournament(t)
	_, err := f.Append(context.Background(), events.Draft{Type: "", Data: []byte(`{}`)})
	wantCode(t, err, engine.CodeInvalidEvent)
}

func TestTournamentOpenErrors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	t.Run("create over an existing file", func(t *testing.T) {
		_, path := newTournament(t)
		_, err := CreateTournament(ctx, path)
		wantCode(t, err, engine.CodeFileExists)
	})

	t.Run("open a missing file", func(t *testing.T) {
		_, err := OpenTournament(ctx, filepath.Join(dir, "missing.openshuttle"))
		wantCode(t, err, engine.CodeFileNotFound)
	})

	t.Run("open a text file", func(t *testing.T) {
		path := filepath.Join(dir, "notes.openshuttle")
		if err := os.WriteFile(path, []byte("not a database, just some text that is long enough"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := OpenTournament(ctx, path)
		wantCode(t, err, engine.CodeNotATournamentFile)
	})

	t.Run("open app.db as a tournament", func(t *testing.T) {
		path := filepath.Join(dir, "app.db")
		a, err := OpenAppDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		a.Close()
		_, err = OpenTournament(ctx, path)
		wantCode(t, err, engine.CodeNotATournamentFile)
	})

	t.Run("open a file from a newer version", func(t *testing.T) {
		f, path := newTournament(t)
		if _, err := f.db.Exec("PRAGMA user_version = 99"); err != nil {
			t.Fatal(err)
		}
		f.Close()
		_, err := OpenTournament(ctx, path)
		wantCode(t, err, engine.CodeFileTooNew)
	})
}

func TestBackup(t *testing.T) {
	ctx := context.Background()
	f, _ := newTournament(t)
	if _, err := f.Append(ctx, draft(t, "ScoreRecorded", map[string]int{"a": 21}, t0)); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "copy.openshuttle")
	if err := f.Backup(ctx, dest); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.Backup(ctx, dest), engine.CodeFileExists)

	copy, err := OpenTournament(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	log, err := copy.Events(ctx)
	if err != nil || len(log) != 1 || string(log[0].Data) != `{"a":21}` {
		t.Errorf("backup log = %v, %v", log, err)
	}
}

func TestBackupRotating(t *testing.T) {
	ctx := context.Background()
	f, _ := newTournament(t)
	dir := filepath.Join(t.TempDir(), "backups")

	var paths []string
	for i := range 5 {
		p, err := f.BackupRotating(ctx, dir, t0.Add(time.Duration(i)*time.Minute), 3)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	// Same second: skipped, no error.
	if _, err := f.BackupRotating(ctx, dir, t0.Add(4*time.Minute), 3); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{
		"plume-automne-2026.backup-20261017-093200.openshuttle",
		"plume-automne-2026.backup-20261017-093300.openshuttle",
		"plume-automne-2026.backup-20261017-093400.openshuttle",
	}
	if len(names) != len(want) {
		t.Fatalf("backups = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("backup %d = %s, want %s", i, names[i], want[i])
		}
	}
	if filepath.Base(paths[4]) != want[2] {
		t.Errorf("latest path = %s", paths[4])
	}
}

func TestAppDBSettings(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "config", "app.db")

	a, err := OpenAppDB(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := a.Setting(ctx, "language"); err != nil || ok {
		t.Fatalf("unset setting = %v, %v", ok, err)
	}
	for _, v := range []string{"en", "fr"} {
		if err := a.SetSetting(ctx, "language", v); err != nil {
			t.Fatal(err)
		}
	}
	a.Close()

	a, err = OpenAppDB(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if v, ok, err := a.Setting(ctx, "language"); err != nil || !ok || v != "fr" {
		t.Errorf("language = %q, %v, %v; want fr", v, ok, err)
	}
}
