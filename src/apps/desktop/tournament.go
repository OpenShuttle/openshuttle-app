package main

import (
	"context"
	"log/slog"
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
}

// ActionView describes a log entry for display. The frontend translates
// Type ("history.<Type>") with Params.
type ActionView struct {
	Seq      uint64            `json:"seq"`
	Type     string            `json:"type"`
	At       time.Time         `json:"at"`
	Operator string            `json:"operator"`
	Params   map[string]string `json:"params"`
}

// TournamentService opens tournament files and runs commands against them.
// Every command appends one event, then emits tournament.updated.
type TournamentService struct {
	app       *store.AppDB
	emit      func(name string, data any)
	now       func() time.Time
	backupDir string
	dialogs   fileDialogs

	mu          sync.Mutex
	file        *store.TournamentFile
	log         []events.Event
	state       tournament.State
	backedUp    uint64 // version of the latest backup
	stopBackups chan struct{}
}

// fileDialogs shows native file dialogs. An empty path means cancelled.
type fileDialogs interface {
	SaveTournament(suggestedName string) (string, error)
	OpenTournament() (string, error)
}

func NewTournamentService(app *store.AppDB, emit func(string, any), now func() time.Time, backupDir string, dialogs fileDialogs) *TournamentService {
	return &TournamentService{app: app, emit: emit, now: now, backupDir: backupDir, dialogs: dialogs}
}

// CreateTournament asks where to save a new tournament, creates it and opens
// it. It returns the path, or "" when the user cancelled the dialog.
func (s *TournamentService) CreateTournament(ctx context.Context, name string) (string, error) {
	name, err := tournament.NormalizeName(name)
	if err != nil {
		return "", err
	}
	path, err := s.dialogs.SaveTournament(fileNameFor(name))
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
	d, err := events.NewDraft(tournament.TypeCreated, tournament.Created{Name: name}, s.now(), s.operator(ctx))
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

// OpenTournamentDialog asks for a tournament file and opens it. It returns
// the path, or "" when the user cancelled.
func (s *TournamentService) OpenTournamentDialog(ctx context.Context) (string, error) {
	path, err := s.dialogs.OpenTournament()
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
		_ = writeRecent(ctx, s.app, touchRecent(list, path, state.Name, s.now()))
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return engine.NewError(engine.CodeNoTournament)
	}
	renamed, changed, err := tournament.Rename(s.state, name)
	if err != nil || !changed {
		return err
	}
	return s.appendLocked(ctx, tournament.TypeRenamed, renamed)
}

// Undo cancels the latest action still in effect.
func (s *TournamentService) Undo(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return engine.NewError(engine.CodeNoTournament)
	}
	last, ok := events.LastUndoable(s.log, tournament.Undoable)
	if !ok {
		return engine.NewError(engine.CodeNothingToUndo)
	}
	return s.appendLocked(ctx, events.TypeUndone, events.Undone{Target: last.Seq})
}

// GetTournamentView returns the current tournament, or Open=false.
func (s *TournamentService) GetTournamentView() TournamentView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return TournamentView{}
	}
	v := TournamentView{
		Open:    true,
		Path:    s.file.Path(),
		Name:    s.state.Name,
		Version: s.state.Version,
	}
	if len(s.log) > 0 {
		v.LastAction = s.describe(s.log[len(s.log)-1])
	}
	_, v.CanUndo = events.LastUndoable(s.log, tournament.Undoable)
	return v
}

// ListRecentTournaments returns the recently opened tournaments.
func (s *TournamentService) ListRecentTournaments(ctx context.Context) ([]RecentTournament, error) {
	return readRecent(ctx, s.app)
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

// describe turns an event into an ActionView. An undo describes its target.
func (s *TournamentService) describe(e events.Event) *ActionView {
	v := &ActionView{Seq: e.Seq, Type: string(e.Type), At: e.At, Operator: e.Operator, Params: map[string]string{}}
	switch e.Type {
	case tournament.TypeCreated, tournament.TypeRenamed:
		var p struct {
			Name string `json:"name"`
		}
		if e.Decode(&p) == nil {
			v.Params["name"] = p.Name
		}
	case events.TypeUndone:
		var u events.Undone
		if e.Decode(&u) == nil && u.Target >= 1 && int(u.Target) <= len(s.log) {
			target := s.describe(s.log[u.Target-1])
			v.Params = target.Params
			v.Params["action"] = target.Type
		}
	}
	return v
}

func (s *TournamentService) operator(ctx context.Context) string {
	op, _, err := s.app.Setting(ctx, settingOperator)
	if err != nil {
		return ""
	}
	return op
}

func (s *TournamentService) closeLocked() {
	if s.stopBackups != nil {
		close(s.stopBackups)
		s.stopBackups = nil
	}
	if s.file != nil {
		s.backupLocked(context.Background())
		if err := s.file.Close(); err != nil {
			slog.Error("close tournament", "err", err)
		}
	}
	s.file, s.log, s.state, s.backedUp = nil, nil, tournament.State{}, 0
}

// backupLocked writes a rotating backup if the tournament changed since the
// latest one. A failed backup is logged, never shown as a blocking error.
func (s *TournamentService) backupLocked(ctx context.Context) {
	if s.file == nil || s.backupDir == "" || (s.backedUp != 0 && s.backedUp == s.state.Version) {
		return
	}
	if _, err := s.file.BackupRotating(ctx, s.backupDir, s.now(), backupsToKeep); err != nil {
		slog.Error("backup tournament", "err", err)
		return
	}
	s.backedUp = s.state.Version
}

func (s *TournamentService) startBackupsLocked() {
	stop := make(chan struct{})
	s.stopBackups = stop
	go func() {
		ticker := time.NewTicker(backupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.mu.Lock()
				// The file may have been closed while waiting for the lock.
				if s.stopBackups == stop {
					s.backupLocked(context.Background())
				}
				s.mu.Unlock()
			}
		}
	}()
}

// fileNameFor suggests a file name for a tournament name.
func fileNameFor(name string) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '-'
		}
		return r
	}, name)
	clean = strings.Trim(clean, " .-")
	if clean == "" {
		clean = "tournament"
	}
	return clean + store.TournamentExtension
}
