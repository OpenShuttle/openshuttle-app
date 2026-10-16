package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

type emitted struct {
	name string
	data any
}

type recorder struct{ events []emitted }

func (r *recorder) emit(name string, data any) { r.events = append(r.events, emitted{name, data}) }

func (r *recorder) last() emitted {
	if len(r.events) == 0 {
		return emitted{}
	}
	return r.events[len(r.events)-1]
}

type fakeDialogs struct {
	save, open, csv, folder string
	suggested, dir          string
}

func (f *fakeDialogs) SaveTournament(dir, name string) (string, error) {
	f.suggested, f.dir = name, dir
	return f.save, nil
}

func (f *fakeDialogs) OpenTournament(dir string) (string, error) {
	f.dir = dir
	return f.open, nil
}

func (f *fakeDialogs) ChooseFolder(dir string) (string, error) {
	f.dir = dir
	return f.folder, nil
}

func (f *fakeDialogs) SaveCSV(dir, name string) (string, error) {
	f.suggested, f.dir = name, dir
	return f.csv, nil
}

type fixture struct {
	ctx      context.Context
	dir      string
	app      *store.AppDB
	rec      *recorder
	dialogs  *fakeDialogs
	clock    time.Time
	settings *SettingsService
	svc      *TournamentService
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	app, err := store.OpenAppDB(ctx, filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		ctx: ctx, dir: dir, app: app, rec: &recorder{}, dialogs: &fakeDialogs{},
		clock: time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC),
	}
	now := func() time.Time { f.clock = f.clock.Add(time.Second); return f.clock }
	f.settings = NewSettingsService(app, f.rec.emit, f.dialogs)
	f.svc = NewTournamentService(app, f.rec.emit, now, filepath.Join(dir, "backups"), f.dialogs)
	t.Cleanup(func() {
		f.svc.ServiceShutdown()
		app.Close()
	})
	return f
}

func wantCode(t *testing.T, err error, code engine.Code) {
	t.Helper()
	var e *engine.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestSettings(t *testing.T) {
	f := newFixture(t)
	s, err := f.settings.GetSettings(f.ctx)
	if err != nil || s.Language != "" || s.RotationSeconds != 8 || s.DefaultCourts != 4 ||
		s.DefaultMinRest != 10 || s.DefaultPoolWin != "sets" || s.TournamentsDir == "" {
		t.Fatalf("default settings = %+v, %v", s, err)
	}
	if err := f.settings.SetLanguage(f.ctx, "fr"); err != nil {
		t.Fatal(err)
	}
	if got := f.rec.last(); got.name != EventSettingsChanged || got.data.(Settings).Language != "fr" {
		t.Errorf("emitted %+v", got)
	}
	wantCode(t, f.settings.SetLanguage(f.ctx, "de"), engine.CodeInvalidSetting)

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.settings.SetOperator(f.ctx, "  Julie   M. "))
	must(f.settings.SetMonitor(f.ctx, "HDMI-1"))
	must(f.settings.SetRotationSeconds(f.ctx, 12))
	must(f.settings.SetDefaultCourts(f.ctx, 6))
	must(f.settings.SetDefaultMinRest(f.ctx, 0))
	must(f.settings.SetDefaultPoolWin(f.ctx, "timed"))
	wantCode(t, f.settings.SetRotationSeconds(f.ctx, 3), engine.CodeInvalidSetting)
	wantCode(t, f.settings.SetDefaultCourts(f.ctx, 0), engine.CodeInvalidSetting)
	wantCode(t, f.settings.SetDefaultMinRest(f.ctx, 61), engine.CodeInvalidSetting)
	wantCode(t, f.settings.SetDefaultPoolWin(f.ctx, "golf"), engine.CodeInvalidSetting)

	// Folder: cancelled, then chosen; dialogs open there.
	if dir, err := f.settings.ChooseTournamentsDir(f.ctx); err != nil || dir != "" {
		t.Fatalf("cancelled folder = %q, %v", dir, err)
	}
	f.dialogs.folder = f.dir
	if dir, err := f.settings.ChooseTournamentsDir(f.ctx); err != nil || dir != f.dir {
		t.Fatalf("folder = %q, %v", dir, err)
	}
	s, _ = f.settings.GetSettings(f.ctx)
	want := Settings{Language: "fr", Operator: "Julie M.", Monitor: "HDMI-1", RotationSeconds: 12,
		DefaultCourts: 6, DefaultMinRest: 0, DefaultPoolWin: "timed", TournamentsDir: f.dir}
	if s != want {
		t.Errorf("settings = %+v, want %+v", s, want)
	}

	// New tournaments take the defaults, and the save dialog opens in the folder.
	f.dialogs.save = filepath.Join(f.dir, "defaults")
	if _, err := f.svc.CreateTournament(f.ctx, "Plume"); err != nil {
		t.Fatal(err)
	}
	if f.dialogs.dir != f.dir {
		t.Errorf("dialog opened in %q, want %q", f.dialogs.dir, f.dir)
	}
	if v := f.svc.GetTournamentView(); v.Courts != 6 || v.MinRest != 0 {
		t.Errorf("tournament = %d courts, %d min rest", v.Courts, v.MinRest)
	}
	must(f.svc.AddCategory(f.ctx, engine.MensSingles))
	if fc := f.svc.GetFormatView().Categories[0]; fc.Stage.Win.Kind != engine.WinByTime {
		t.Errorf("default pool win = %+v", fc.Stage.Win)
	}
}

func TestTournamentLifecycle(t *testing.T) {
	f := newFixture(t)
	if err := f.settings.SetOperator(f.ctx, "Julie"); err != nil {
		t.Fatal(err)
	}

	// Cancelled dialog: nothing happens.
	path, err := f.svc.CreateTournament(f.ctx, "Plume d'automne")
	if err != nil || path != "" || f.svc.GetTournamentView().Open {
		t.Fatalf("cancelled create = %q, %v", path, err)
	}
	if f.dialogs.suggested != "Plume d'automne.openshuttle" {
		t.Errorf("suggested file name = %q", f.dialogs.suggested)
	}

	// The extension is added when missing.
	f.dialogs.save = filepath.Join(f.dir, "plume")
	path, err = f.svc.CreateTournament(f.ctx, "Plume d'automne")
	if err != nil {
		t.Fatal(err)
	}
	if path != f.dialogs.save {
		t.Errorf("path = %q", path)
	}
	full := f.dialogs.save + ".openshuttle"
	v := f.svc.GetTournamentView()
	if !v.Open || v.Path != full || v.Name != "Plume d'automne" || v.Version != 1 || v.CanUndo {
		t.Fatalf("view after create = %+v", v)
	}
	if v.LastAction.Type != "TournamentCreated" || v.LastAction.Operator != "Julie" {
		t.Errorf("last action = %+v", v.LastAction)
	}
	if got := f.rec.last(); got.name != EventTournamentUpdated || got.data.(uint64) != 1 {
		t.Errorf("emitted %+v", got)
	}

	// Rename, unchanged rename, then undo.
	if err := f.svc.RenameTournament(f.ctx, "Nuit du volant"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RenameTournament(f.ctx, " Nuit du volant "); err != nil {
		t.Fatal(err)
	}
	v = f.svc.GetTournamentView()
	if v.Name != "Nuit du volant" || v.Version != 2 || !v.CanUndo {
		t.Fatalf("view after rename = %+v", v)
	}
	wantCode(t, f.svc.RenameTournament(f.ctx, ""), engine.CodeInvalidName)

	if err := f.svc.Undo(f.ctx); err != nil {
		t.Fatal(err)
	}
	v = f.svc.GetTournamentView()
	if v.Name != "Plume d'automne" || v.Version != 3 || v.CanUndo {
		t.Fatalf("view after undo = %+v", v)
	}
	if a := v.LastAction; a.Type != "ActionUndone" || a.Params["action"] != "TournamentRenamed" || a.Params["name"] != "Nuit du volant" {
		t.Errorf("undo action = %+v", a)
	}
	wantCode(t, f.svc.Undo(f.ctx), engine.CodeNothingToUndo)

	// The recent list follows the name.
	recent, err := f.svc.ListRecentTournaments(f.ctx)
	if err != nil || len(recent) != 1 || recent[0].Path != full || recent[0].Name != "Plume d'automne" || !recent[0].Exists {
		t.Fatalf("recent = %+v, %v", recent, err)
	}

	// Close, reopen: state comes back from the file.
	f.svc.CloseTournament()
	if f.svc.GetTournamentView().Open {
		t.Fatal("still open after close")
	}
	wantCode(t, f.svc.Undo(f.ctx), engine.CodeNoTournament)

	f.dialogs.open = full
	if _, err := f.svc.OpenTournamentDialog(f.ctx); err != nil {
		t.Fatal(err)
	}
	v = f.svc.GetTournamentView()
	if v.Name != "Plume d'automne" || v.Version != 3 {
		t.Errorf("reopened view = %+v", v)
	}

	// Opening backs the file up.
	backups, _ := os.ReadDir(filepath.Join(f.dir, "backups"))
	if len(backups) == 0 {
		t.Error("no backup written")
	}
}

func TestCreateRefusesExistingFile(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.dir, "a.openshuttle")
	if err := f.svc.CreateTournamentAt(f.ctx, path, "A"); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.svc.CreateTournamentAt(f.ctx, path, "B"), engine.CodeFileExists)
	if f.svc.GetTournamentView().Name != "A" {
		t.Error("the open tournament changed")
	}
}

func TestRecentList(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	var list []RecentTournament
	for i := range 12 {
		list = touchRecent(list, filepath.Join("t", string(rune('a'+i))), "T", t0)
	}
	if len(list) != maxRecentTournaments || list[0].Path != filepath.Join("t", "l") {
		t.Fatalf("list = %+v", list)
	}
	list = touchRecent(list, filepath.Join("t", "e"), "E", t0)
	if list[0].Name != "E" || len(list) != maxRecentTournaments {
		t.Errorf("touch existing = %+v", list[0])
	}
	list = renameRecent(list, filepath.Join("t", "e"), "E2")
	if list[0].Name != "E2" {
		t.Errorf("rename = %+v", list[0])
	}
}

func TestFileNameFor(t *testing.T) {
	tests := map[string]string{
		"Plume d'automne": "Plume d'automne.openshuttle",
		`A/B: "C"?`:       "A-B- -C.openshuttle",
		"...":             "tournament.openshuttle",
	}
	for in, want := range tests {
		if got := fileNameFor(in); got != want {
			t.Errorf("fileNameFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarshalError(t *testing.T) {
	var cause errorCause
	if err := json.Unmarshal(marshalError(&engine.Error{Code: engine.CodeInvalidSetScore, Set: 2}), &cause); err != nil {
		t.Fatal(err)
	}
	if cause.Code != engine.CodeInvalidSetScore || cause.Set != 2 {
		t.Errorf("cause = %+v", cause)
	}
	_ = json.Unmarshal(marshalError(errors.New("disk on fire")), &cause)
	if cause.Code != engine.CodeInternal {
		t.Errorf("unexpected error cause = %+v", cause)
	}
}
