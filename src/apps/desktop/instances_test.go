package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

// otherInstance is a second instance of the app: its own service on the same
// app.db.
func (f *fixture) otherInstance(t *testing.T) *TournamentService {
	t.Helper()
	app, err := store.OpenAppDB(f.ctx, filepath.Join(f.dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewTournamentService(app, func(string, any) {}, func() time.Time { return f.clock }, filepath.Join(f.dir, "backups"), &fakeDialogs{})
	t.Cleanup(func() {
		svc.ServiceShutdown()
		app.Close()
	})
	return svc
}

func TestTournamentOpenInOneInstance(t *testing.T) {
	f := newFixture(t)
	a := filepath.Join(f.dir, "a.openshuttle")
	b := filepath.Join(f.dir, "b.openshuttle")
	for _, p := range []string{b, a} {
		if err := f.svc.CreateTournamentAt(f.ctx, p, filepath.Base(p)); err != nil {
			t.Fatal(err)
		}
	}
	other := f.otherInstance(t)

	// a is open here: the other instance cannot open it, but opens b.
	wantCode(t, other.OpenTournament(f.ctx, a), engine.CodeFileInUse)
	if other.GetTournamentView().Open {
		t.Fatal("a refused file was opened")
	}
	if err := other.OpenTournament(f.ctx, b); err != nil {
		t.Fatal(err)
	}
	// Opening the open file again in its own instance reopens it.
	if err := f.svc.OpenTournament(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	if v := f.svc.GetTournamentView(); !v.Open || v.Path != a {
		t.Fatalf("view = %+v", v)
	}
	// A file open elsewhere can still be duplicated: it is only read.
	f.dialogs.save = filepath.Join(f.dir, "copy.openshuttle")
	if _, err := f.svc.DuplicateTournament(f.ctx, b, "Copy"); err != nil {
		t.Fatal(err)
	}

	// Its file is not deleted from the trash while open elsewhere.
	if err := f.svc.OpenTournament(f.ctx, f.dialogs.save); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TrashTournament(f.ctx, b); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.svc.DeleteTournament(f.ctx, b, true), engine.CodeFileInUse)
	if _, err := os.Stat(b); err != nil {
		t.Fatalf("deleted a file open elsewhere: %v", err)
	}
	if trash, _ := f.svc.ListTrashedTournaments(f.ctx); len(trash) != 1 {
		t.Fatalf("trash = %+v", trash)
	}
	// Once closed there, it goes.
	other.CloseTournament()
	if err := f.svc.DeleteTournament(f.ctx, b, true); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{b, b + "-wal", b + "-shm", b + store.LockSuffix} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s left behind", filepath.Base(p))
		}
	}
}
