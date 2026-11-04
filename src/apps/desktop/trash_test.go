package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestTrash(t *testing.T) {
	f := newFixture(t)
	a := filepath.Join(f.dir, "a.openshuttle")
	b := filepath.Join(f.dir, "b.openshuttle")
	for _, p := range []string{a, b} {
		if err := f.svc.CreateTournamentAt(f.ctx, p, filepath.Base(p)); err != nil {
			t.Fatal(err)
		}
	}

	paths := func(list []RecentTournament, err error) []string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, r := range list {
			out = append(out, r.Path)
		}
		return out
	}
	check := func(what string, recent, trash []string) {
		t.Helper()
		if got := paths(f.svc.ListRecentTournaments(f.ctx)); !equal(got, recent) {
			t.Errorf("%s: recent = %v, want %v", what, got, recent)
		}
		if got := paths(f.svc.ListTrashedTournaments(f.ctx)); !equal(got, trash) {
			t.Errorf("%s: trash = %v, want %v", what, got, trash)
		}
	}

	// Trashing the open tournament closes it; its file stays.
	if err := f.svc.TrashTournament(f.ctx, b); err != nil {
		t.Fatal(err)
	}
	if f.svc.GetTournamentView().Open {
		t.Error("the trashed tournament is still open")
	}
	if got := f.rec.last(); got.name != EventTournamentUpdated || got.data.(uint64) != 0 {
		t.Errorf("emitted %+v", got)
	}
	if _, err := os.Stat(b); err != nil {
		t.Errorf("trashed file: %v", err)
	}
	check("trash b", []string{a}, []string{b})
	trash, _ := f.svc.ListTrashedTournaments(f.ctx)
	if tr := trash[0]; tr.TrashedAt == nil || tr.Summary == nil || !tr.Exists || tr.Name != "b.openshuttle" {
		t.Errorf("trashed entry = %+v", tr)
	}
	// Trashing what is not listed any more changes nothing.
	if err := f.svc.TrashTournament(f.ctx, b); err != nil {
		t.Fatal(err)
	}
	check("trash b twice", []string{a}, []string{b})

	// Restore puts it back on top.
	if err := f.svc.RestoreTournament(f.ctx, b); err != nil {
		t.Fatal(err)
	}
	check("restore b", []string{b, a}, []string{})
	if recent, _ := f.svc.ListRecentTournaments(f.ctx); recent[0].TrashedAt != nil {
		t.Errorf("restored entry = %+v", recent[0])
	}
	wantCode(t, f.svc.RestoreTournament(f.ctx, b), engine.CodeNotInTrash)

	// Opening a trashed file takes it out of the trash.
	if err := f.svc.TrashTournament(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.OpenTournament(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	check("reopen a", []string{a, b}, []string{})

	// Delete keeps the file unless asked.
	if err := f.svc.TrashTournament(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TrashTournament(f.ctx, b); err != nil {
		t.Fatal(err)
	}
	check("trash both", []string{}, []string{b, a})
	wantCode(t, f.svc.DeleteTournament(f.ctx, filepath.Join(f.dir, "c.openshuttle"), true), engine.CodeNotInTrash)
	if err := f.svc.DeleteTournament(f.ctx, a, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a); err != nil {
		t.Errorf("kept file: %v", err)
	}
	check("delete a", []string{}, []string{b})

	// Delete with the file removes it and its SQLite siblings, not the backups.
	for _, p := range []string{b + "-wal", b + "-shm"} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	backups, _ := os.ReadDir(filepath.Join(f.dir, "backups"))
	if err := f.svc.DeleteTournament(f.ctx, b, true); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{b, b + "-wal", b + "-shm"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still there: %v", p, err)
		}
	}
	if after, _ := os.ReadDir(filepath.Join(f.dir, "backups")); len(after) != len(backups) || len(after) == 0 {
		t.Errorf("backups = %d, want %d", len(after), len(backups))
	}
	check("delete b", []string{}, []string{})

	// A file already gone is deleted without complaint.
	if err := f.svc.OpenTournament(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TrashTournament(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	if trash, _ := f.svc.ListTrashedTournaments(f.ctx); trash[0].Exists {
		t.Error("missing file shown as existing")
	}
	if err := f.svc.DeleteTournament(f.ctx, a, true); err != nil {
		t.Fatal(err)
	}
	check("delete missing", []string{}, []string{})
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
