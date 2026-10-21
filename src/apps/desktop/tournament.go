package main

import (
	"context"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

const (
	backupInterval = 5 * time.Minute
	backupsToKeep  = 20
)

// TournamentView is what the top bar and the home screen need.
type TournamentView struct {
	Open       bool        `json:"open"`
	Path       string      `json:"path"`
	Name       string      `json:"name"`
	Version    uint64      `json:"version"`
	LastAction *ActionView `json:"lastAction"`
	CanUndo    bool        `json:"canUndo"`
	// Categories in slot order: the first gets --color-category-1.
	Categories []CategoryRef `json:"categories"`
	Archived   bool          `json:"archived"`
	// AvailableCategories are the categories that can still be added.
	AvailableCategories []engine.Category `json:"availableCategories"`
	Courts              int               `json:"courts"`
	MinRest             int               `json:"minRest"`
	Progress            ProgressView      `json:"progress"`
}

// CategoryRef is a category with its slot (1 or 2), which sets its color.
type CategoryRef struct {
	Category engine.Category `json:"category"`
	Slot     int             `json:"slot"`
}

// TournamentService opens tournament files and runs commands against them.
// Every command appends one event, then emits tournament.updated.
type TournamentService struct {
	app       *store.AppDB
	emit      func(name string, data any)
	now       func() time.Time
	backupDir string
	dialogs   fileDialogs
	rng       func() *rand.Rand

	mu          sync.Mutex
	file        *store.TournamentFile
	log         []events.Event
	state       tournament.State
	backedUp    uint64 // version of the latest backup
	stopBackups chan struct{}
}

// fileDialogs shows native file dialogs. An empty path means cancelled.
type fileDialogs interface {
	SaveTournament(dir, suggestedName string) (string, error)
	OpenTournament(dir string) (string, error)
	SaveCSV(dir, suggestedName string) (string, error)
	OpenCSV(dir string) (string, error)
	ChooseFolder(dir string) (string, error)
}

func NewTournamentService(app *store.AppDB, emit func(string, any), now func() time.Time, backupDir string, dialogs fileDialogs) *TournamentService {
	return &TournamentService{app: app, emit: emit, now: now, backupDir: backupDir, dialogs: dialogs, rng: cryptoRand}
}

// CreateTournament asks where to save a new tournament, creates it and opens
// it. It returns the path, or "" when the user cancelled the dialog.
func (s *TournamentService) CreateTournament(ctx context.Context, name string) (string, error) {
	name, err := tournament.NormalizeName(name)
	if err != nil {
		return "", err
	}
	path, err := s.dialogs.SaveTournament(s.tournamentsDir(ctx), fileNameFor(name))
	if err != nil || path == "" {
		return "", err
	}
	return path, s.CreateTournamentAt(ctx, path, name)
}

// CreateTournamentAt creates a tournament file at path and opens it.
func (s *TournamentService) CreateTournamentAt(ctx context.Context, path, name string) error {
	name, err := tournament.NormalizeName(name)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(path), store.TournamentExtension) {
		path += store.TournamentExtension
	}
	f, err := store.CreateTournament(ctx, path)
	if err != nil {
		return err
	}
	d, err := events.NewDraft(tournament.TypeCreated, s.createdWithDefaults(ctx, name), s.now(), s.operator(ctx))
	if err == nil {
		_, err = f.Append(ctx, d)
	}
	if err != nil {
		f.Close()
		return err
	}
	f.Close()
	return s.OpenTournament(ctx, path)
}

// DuplicateTournament asks where to save a copy of a tournament's setup
// (courts, categories, format), without its entrants, and adds it to the
// recent list without opening it. It returns the path, or "" when the user
// cancelled.
func (s *TournamentService) DuplicateTournament(ctx context.Context, source, name string) (string, error) {
	src, err := s.stateOf(ctx, source)
	if err != nil {
		return "", err
	}
	changes, err := tournament.Duplicate(src, name)
	if err != nil {
		return "", err
	}
	path, err := s.dialogs.SaveTournament(s.tournamentsDir(ctx), fileNameFor(name))
	if err != nil || path == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(path), store.TournamentExtension) {
		path += store.TournamentExtension
	}
	f, err := store.CreateTournament(ctx, path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	log := make([]events.Event, 0, len(changes))
	for _, ch := range changes {
		d, err := events.NewDraft(ch.Type, ch.Data, s.now(), s.operator(ctx))
		if err != nil {
			return "", err
		}
		e, err := f.Append(ctx, d)
		if err != nil {
			return "", err
		}
		log = append(log, e)
	}
	state, err := tournament.Project(log)
	if err != nil {
		return "", err
	}
	if list, err := readRecent(ctx, s.app); err == nil {
		list = touchRecent(list, path, state.Name, s.now())
		_ = writeRecent(ctx, s.app, setSummary(list, path, summaryOf(state, lastEventAt(log))))
	}
	return path, nil
}

// OpenTournamentDialog asks for a tournament file and opens it. It returns
// the path, or "" when the user cancelled.
func (s *TournamentService) OpenTournamentDialog(ctx context.Context) (string, error) {
	path, err := s.dialogs.OpenTournament(s.tournamentsDir(ctx))
	if err != nil || path == "" {
		return "", err
	}
	return path, s.OpenTournament(ctx, path)
}

// OpenTournament opens a tournament file, closing the current one.
func (s *TournamentService) OpenTournament(ctx context.Context, path string) error {
	f, err := store.OpenTournament(ctx, path)
	if err != nil {
		return err
	}
	log, err := f.Events(ctx)
	if err != nil {
		f.Close()
		return err
	}
	state, err := tournament.Project(log)
	if err != nil {
		f.Close()
		return err
	}

	s.mu.Lock()
	s.closeLocked()
	s.file, s.log, s.state = f, log, state
	s.backupLocked(ctx)
	s.startBackupsLocked()
	s.mu.Unlock()

	if list, err := readRecent(ctx, s.app); err == nil {
		list = touchRecent(list, path, state.Name, s.now())
		_ = writeRecent(ctx, s.app, setSummary(list, path, summaryOf(state, lastEventAt(log))))
	}
	s.emit(EventTournamentUpdated, state.Version)
	return nil
}

// CloseTournament closes the current tournament, if any.
func (s *TournamentService) CloseTournament() {
	s.mu.Lock()
	s.closeLocked()
	s.mu.Unlock()
	s.emit(EventTournamentUpdated, uint64(0))
}

// RenameTournament renames the current tournament.
func (s *TournamentService) RenameTournament(ctx context.Context, name string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.Rename(name) })
}

// Undo cancels the latest action still in effect.
func (s *TournamentService) Undo(ctx context.Context) error {
	return s.execute(ctx, func(tournament.State) (tournament.Change, error) {
		last, ok := events.LastUndoable(s.log, tournament.Undoable)
		if !ok {
			return tournament.Change{}, engine.NewError(engine.CodeNothingToUndo)
		}
		return tournament.Change{Type: events.TypeUndone, Data: events.Undone{Target: last.Seq}}, nil
	})
}

// execute runs a command against the current state and appends its change.
func (s *TournamentService) execute(ctx context.Context, cmd func(tournament.State) (tournament.Change, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return engine.NewError(engine.CodeNoTournament)
	}
	ch, err := cmd(s.state)
	if err != nil || ch.None() {
		return err
	}
	return s.appendLocked(ctx, ch.Type, ch.Data)
}

// GetTournamentView returns the current tournament, or Open=false.
func (s *TournamentService) GetTournamentView() TournamentView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return TournamentView{Categories: []CategoryRef{}, AvailableCategories: []engine.Category{}}
	}
	v := TournamentView{
		Open:                true,
		Path:                s.file.Path(),
		Name:                s.state.Name,
		Version:             s.state.Version,
		Categories:          categoryRefs(s.state),
		Archived:            s.state.Archived,
		AvailableCategories: s.state.AvailableCategories(),
		Courts:              s.state.Courts(),
		MinRest:             s.state.MinRest(),
		Progress:            progressOf(s.state),
	}
	if len(s.log) > 0 {
		v.LastAction = s.describe(s.log[len(s.log)-1])
	}
	_, v.CanUndo = events.LastUndoable(s.log, tournament.Undoable)
	return v
}

// ListRecentTournaments returns the recently opened tournaments. The open
// one is summed up from its live state.
func (s *TournamentService) ListRecentTournaments(ctx context.Context) ([]RecentTournament, error) {
	list, err := readRecent(ctx, s.app)
	if err != nil {
		return list, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		list = setSummary(list, s.file.Path(), summaryOf(s.state, lastEventAt(s.log)))
	}
	return list, nil
}

// ServiceShutdown closes the tournament when the app quits.
func (s *TournamentService) ServiceShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
	return nil
}

func (s *TournamentService) appendLocked(ctx context.Context, typ events.Type, data any) error {
	d, err := events.NewDraft(typ, data, s.now(), s.operator(ctx))
	if err != nil {
		return err
	}
	e, err := s.file.Append(ctx, d)
	if err != nil {
		return err
	}
	log := append(s.log, e)
	state, err := tournament.Project(log)
	if err != nil {
		return err
	}
	nameChanged := state.Name != s.state.Name
	s.log, s.state = log, state

	if nameChanged {
		if list, err := readRecent(ctx, s.app); err == nil {
			_ = writeRecent(ctx, s.app, renameRecent(list, s.file.Path(), state.Name))
		}
	}
	s.emit(EventTournamentUpdated, state.Version)
	return nil
}

func (s *TournamentService) operator(ctx context.Context) string {
	op, _, err := s.app.Setting(ctx, settingOperator)
	if err != nil {
		return ""
	}
	return op
}
