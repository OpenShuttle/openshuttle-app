package main

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"path/filepath"
	"strconv"
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
	Courts     int           `json:"courts"`
	MinRest    int           `json:"minRest"`
	Priority   string        `json:"priority"`
}

// CategoryRef is a category with its slot (1 or 2), which sets its color.
type CategoryRef struct {
	Category engine.Category `json:"category"`
	Slot     int             `json:"slot"`
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
	SaveTournament(suggestedName string) (string, error)
	OpenTournament() (string, error)
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
		return TournamentView{Categories: []CategoryRef{}}
	}
	v := TournamentView{
		Open:       true,
		Path:       s.file.Path(),
		Name:       s.state.Name,
		Version:    s.state.Version,
		Categories: categoryRefs(s.state),
		Courts:     s.state.Courts(),
		MinRest:    s.state.MinRest(),
		Priority:   s.state.CourtPriority(),
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
	var p struct {
		Match    tournament.MatchID  `json:"match"`
		Court    int                 `json:"court"`
		Courts   int                 `json:"courts"`
		Minutes  int                 `json:"minutes"`
		Priority string              `json:"priority"`
		Score    engine.Score        `json:"score"`
		Pool     int                 `json:"pool"`
		Seed     int                 `json:"seed"`
		Entrant  engine.EntrantID    `json:"entrant"`
		Name     string              `json:"name"`
		Category engine.Category     `json:"category"`
		ID       engine.EntrantID    `json:"id"`
		Player   tournament.PlayerID `json:"player"`
		Bib      int                 `json:"bib"`
		Players  []tournament.Player `json:"players"`
	}
	if e.Type != events.TypeUndone && e.Decode(&p) != nil {
		return v
	}
	switch e.Type {
	case tournament.TypeCreated, tournament.TypeRenamed:
		v.Params["name"] = p.Name
	case tournament.TypeCategoryAdded, tournament.TypeCategoryRemoved, tournament.TypePoolStageConfigured,
		tournament.TypePoolsDrawn, tournament.TypeDrawConfirmed, tournament.TypeDrawReopened:
		v.Params["category"] = string(p.Category)
	case tournament.TypeMatchCalled, tournament.TypeMatchEnded, tournament.TypeScoreRecorded:
		v.Params["a"], v.Params["b"] = matchLabel(s.state, p.Match)
		if m, ok := s.state.Match(p.Match); ok {
			v.Params["court"] = itoa(m.Court)
		}
		if e.Type == tournament.TypeMatchCalled {
			v.Params["court"] = itoa(p.Court)
		}
		if e.Type == tournament.TypeScoreRecorded {
			v.Params["score"] = scoreLabel(p.Score)
		}
	case tournament.TypeEntrantWithdrawn:
		v.Params["names"] = s.entrantNamesAt(p.Entrant, e.Seq)
	case tournament.TypeCourtsSet:
		v.Params["courts"] = itoa(p.Courts)
	case tournament.TypeMinRestSet:
		v.Params["minutes"] = itoa(p.Minutes)
	case tournament.TypeCourtPrioritySet:
		v.Params["priority"] = p.Priority
	case tournament.TypeEntrantSeeded:
		v.Params["names"] = s.entrantNamesAt(p.ID, e.Seq)
		v.Params["seed"] = itoa(p.Seed)
	case tournament.TypeEntrantMoved:
		v.Params["names"] = s.entrantNamesAt(p.Entrant, e.Seq)
		v.Params["pool"] = engine.PoolName(p.Pool)
	case tournament.TypeEntrantRegistered, tournament.TypeEntrantUpdated:
		v.Params["names"] = playerNames(p.Players)
	case tournament.TypeEntrantRemoved:
		v.Params["names"] = s.entrantNamesAt(p.ID, e.Seq)
	case tournament.TypePlayerCheckedIn, tournament.TypeCheckInCancelled:
		v.Params["name"] = s.playerNameAt(p.Player, e.Seq)
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

// registrations walks the log (undone events included, so history can name
// removed entrants) before seq, newest first.
func (s *TournamentService) registrations(before uint64, visit func(engine.EntrantID, []tournament.Player) bool) {
	for i := min(int(before), len(s.log)) - 1; i >= 0; i-- {
		e := s.log[i]
		if e.Type != tournament.TypeEntrantRegistered && e.Type != tournament.TypeEntrantUpdated {
			continue
		}
		var p struct {
			ID      engine.EntrantID    `json:"id"`
			Players []tournament.Player `json:"players"`
		}
		if e.Decode(&p) == nil && !visit(p.ID, p.Players) {
			return
		}
	}
}

func (s *TournamentService) entrantNamesAt(id engine.EntrantID, seq uint64) string {
	names := ""
	s.registrations(seq, func(eid engine.EntrantID, players []tournament.Player) bool {
		if eid == id {
			names = playerNames(players)
			return false
		}
		return true
	})
	return names
}

func (s *TournamentService) playerNameAt(id tournament.PlayerID, seq uint64) string {
	name := ""
	s.registrations(seq, func(_ engine.EntrantID, players []tournament.Player) bool {
		for _, p := range players {
			if p.ID == id {
				name = p.FirstName + " " + p.LastName
				return false
			}
		}
		return true
	})
	return name
}

func playerNames(players []tournament.Player) string {
	names := make([]string, len(players))
	for i, p := range players {
		names[i] = p.FirstName + " " + p.LastName
	}
	return strings.Join(names, " / ")
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

func itoa(n int) string { return strconv.Itoa(n) }
