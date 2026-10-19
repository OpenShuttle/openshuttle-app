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
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
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
	openCSV                 string
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

func (f *fakeDialogs) OpenCSV(dir string) (string, error) {
	f.dir = dir
	return f.openCSV, nil
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

// newFixture is a fixture whose new categories start with the engine's
// default format (2 pools, 2 sets of 21), which most tests build on.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := newBareFixture(t)
	presets := NewPresetService(f.app, func(string, any) {}, func() time.Time { return f.clock })
	if err := presets.SaveFormatPreset(f.ctx, "", "Classic", "", []tournament.StageConfig{tournament.DefaultPoolsStage("S1")}); err != nil {
		t.Fatal(err)
	}
	p, err := presets.GetPresets(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.settings.SetDefaultPreset(f.ctx, PresetKindFormat, p.Formats[len(p.Formats)-1].ID); err != nil {
		t.Fatal(err)
	}
	f.rec.events = nil
	return f
}

// newBareFixture is a fixture with the app's own defaults.
func newBareFixture(t *testing.T) *fixture {
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

// editEntry changes the entry stage of a category.
func (f *fixture) editEntry(c engine.Category, edit func(*tournament.StageConfig)) error {
	for _, fc := range f.svc.GetFormatView().Categories {
		if fc.Category != c {
			continue
		}
		stages := make([]tournament.StageConfig, len(fc.Stages))
		for i, st := range fc.Stages {
			stages[i] = st.StageConfig
		}
		edit(&stages[0])
		return f.svc.SetFormat(f.ctx, c, stages)
	}
	return nil
}

func wantCode(t *testing.T, err error, code engine.Code) {
	t.Helper()
	var e *engine.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestSettings(t *testing.T) {
	f := newBareFixture(t)
	s, err := f.settings.GetSettings(f.ctx)
	if err != nil || s.Language != "" || s.RotationSeconds != 8 || s.DefaultCourts != 4 ||
		s.DefaultMinRest != 10 || s.TournamentsDir == "" ||
		s.DefaultFormat != "b-pools" || s.DefaultWin != "b-sets-2x15" || s.DefaultTiebreaks != "b-standard" {
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
	must(f.settings.SetDefaultPreset(f.ctx, PresetKindFormat, "b-pools-main"))
	must(f.settings.SetDefaultPreset(f.ctx, PresetKindWin, "b-timed-10"))
	must(f.settings.SetDefaultPreset(f.ctx, PresetKindTiebreaks, "b-differences-first"))
	wantCode(t, f.settings.SetRotationSeconds(f.ctx, 3), engine.CodeInvalidSetting)
	wantCode(t, f.settings.SetDefaultCourts(f.ctx, 0), engine.CodeInvalidSetting)
	wantCode(t, f.settings.SetDefaultMinRest(f.ctx, 61), engine.CodeInvalidSetting)
	wantCode(t, f.settings.SetDefaultPreset(f.ctx, "golf", "b-pools"), engine.CodeInvalidSetting)
	wantCode(t, f.settings.SetDefaultPreset(f.ctx, PresetKindWin, "b-pools"), engine.CodePresetNotFound)

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
		DefaultCourts: 6, DefaultMinRest: 0, TournamentsDir: f.dir,
		DefaultFormat: "b-pools-main", DefaultWin: "b-timed-10", DefaultTiebreaks: "b-differences-first"}
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
	// A new category starts with the default format.
	must(f.svc.AddCategory(f.ctx, engine.MensSingles))
	if fc := f.svc.GetFormatView().Categories[0]; len(fc.Stages) != 2 || fc.Stages[1].Kind != tournament.StageBracket ||
		fc.Stages[0].Win != (engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 2, PointsPerSet: 15, Cap: 21}) {
		t.Errorf("default format = %+v", fc.Stages)
	}
	// One undo removes the category and its format together.
	must(f.svc.Undo(f.ctx))
	if v := f.svc.GetFormatView(); len(v.Categories) != 0 {
		t.Errorf("categories after undo = %+v", v.Categories)
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

func TestDuplicateAndSummaries(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(f.dir, "src.openshuttle")
	if err := f.svc.CreateTournamentAt(f.ctx, src, "Plume"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AddCategory(f.ctx, engine.MensDoubles); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RegisterEntrant(f.ctx, engine.MensDoubles, []tournament.PlayerInput{
		{Name: "A A"}, {Name: "B B"},
	}); err != nil {
		t.Fatal(err)
	}
	p := f.svc.GetTournamentView().Progress
	if p.Entrants != 1 || !p.Doubles || p.Players != 2 || p.PlayersPresent != 0 || p.Stage != StagePreparation {
		t.Errorf("progress = %+v", p)
	}

	// The open tournament is summed up live.
	recent, _ := f.svc.ListRecentTournaments(f.ctx)
	if s := recent[0].Summary; s == nil || s.Entrants != 1 || len(s.Categories) != 1 || s.Stage != StagePreparation {
		t.Fatalf("live summary = %+v", recent[0].Summary)
	}

	// Cancelled dialog: nothing is written.
	if path, err := f.svc.DuplicateTournament(f.ctx, src, "Plume 2027"); err != nil || path != "" {
		t.Fatalf("cancelled duplicate = %q, %v", path, err)
	}
	wantCode(t, func() error { _, err := f.svc.DuplicateTournament(f.ctx, src, " "); return err }(), engine.CodeInvalidName)

	f.dialogs.save = filepath.Join(f.dir, "copy")
	path, err := f.svc.DuplicateTournament(f.ctx, src, "Plume 2027")
	if err != nil || path != f.dialogs.save+".openshuttle" {
		t.Fatalf("duplicate = %q, %v", path, err)
	}
	if f.svc.GetTournamentView().Path != src {
		t.Error("duplicating changed the open tournament")
	}
	recent, _ = f.svc.ListRecentTournaments(f.ctx)
	if recent[0].Path != path || recent[0].Name != "Plume 2027" || recent[0].Summary.Entrants != 0 ||
		len(recent[0].Summary.Categories) != 1 {
		t.Fatalf("recent after duplicate = %+v", recent[0])
	}

	// Closing keeps the summary of the closed tournament.
	f.svc.CloseTournament()
	recent, _ = f.svc.ListRecentTournaments(f.ctx)
	if recent[1].Path != src || recent[1].Summary == nil || recent[1].Summary.Entrants != 1 {
		t.Fatalf("summary after close = %+v", recent[1])
	}

	// A closed file can be duplicated too, read from disk.
	f.dialogs.save = filepath.Join(f.dir, "copy2.openshuttle")
	if _, err := f.svc.DuplicateTournament(f.ctx, src, "Plume 2028"); err != nil {
		t.Fatal(err)
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
