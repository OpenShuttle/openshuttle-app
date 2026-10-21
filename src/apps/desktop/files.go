package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// stateOf projects a tournament file: the open one from memory, any other
// read from disk.
func (s *TournamentService) stateOf(ctx context.Context, path string) (tournament.State, error) {
	s.mu.Lock()
	if s.file != nil && s.file.Path() == path {
		st := s.state
		s.mu.Unlock()
		return st, nil
	}
	s.mu.Unlock()
	f, err := store.OpenTournament(ctx, path)
	if err != nil {
		return tournament.State{}, err
	}
	defer f.Close()
	log, err := f.Events(ctx)
	if err != nil {
		return tournament.State{}, err
	}
	return tournament.Project(log)
}

func (s *TournamentService) closeLocked() {
	if s.stopBackups != nil {
		close(s.stopBackups)
		s.stopBackups = nil
	}
	if s.file != nil {
		ctx := context.Background()
		s.backupLocked(ctx)
		if list, err := readRecent(ctx, s.app); err == nil {
			_ = writeRecent(ctx, s.app, setSummary(list, s.file.Path(), summaryOf(s.state, lastEventAt(s.log))))
		}
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

// createdWithDefaults is the creation event carrying the app's defaults for
// new tournaments.
func (s *TournamentService) createdWithDefaults(ctx context.Context, name string) tournament.Created {
	c := tournament.Created{Name: name}
	settings, err := loadSettings(ctx, s.app)
	if err != nil {
		return c
	}
	rest := settings.DefaultMinRest
	c.Courts, c.MinRest = settings.DefaultCourts, &rest
	return c
}

// tournamentsDir is where file dialogs open.
func (s *TournamentService) tournamentsDir(ctx context.Context) string {
	if settings, err := loadSettings(ctx, s.app); err == nil {
		return settings.TournamentsDir
	}
	return ""
}
