package main

import (
	"context"

	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// readClubBook reads the club list and the club display setting of app.db.
// A failed read knows no club: clubs show as typed.
func readClubBook(ctx context.Context, db *store.AppDB) tournament.ClubBook {
	presets, err := readUserPresets(ctx, db)
	if err != nil {
		return tournament.ClubBook{}
	}
	display, _, _ := db.Setting(ctx, settingClubDisplay)
	return tournament.NewClubBook(presets.Clubs, clubDisplay(display))
}

// clubBook is the club book the open tournament resolves clubs with.
func (s *TournamentService) clubBook() tournament.ClubBook {
	if b := s.clubs.Load(); b != nil {
		return *b
	}
	b := readClubBook(context.Background(), s.app)
	s.clubs.Store(&b)
	return b
}

// project projects a log, resolving clubs with the club list of this PC.
func (s *TournamentService) project(log []events.Event) (tournament.State, error) {
	st, err := tournament.Project(log)
	return st.WithClubs(s.clubBook()), err
}

// refreshClubs reads the club list and display setting again, and has both
// windows refetch the open tournament's views.
func (s *TournamentService) refreshClubs(ctx context.Context) {
	b := readClubBook(ctx, s.app)
	s.clubs.Store(&b)
	s.mu.Lock()
	open := s.file != nil
	s.state = s.state.WithClubs(b)
	version := s.state.Version
	s.mu.Unlock()
	if open {
		s.emit(EventTournamentUpdated, version)
	}
}

// followClubs wraps the emit of the settings and preset services: a change
// of the club list or of the settings refreshes how the open tournament
// shows its clubs.
func (s *TournamentService) followClubs(emit func(string, any)) func(string, any) {
	return func(name string, data any) {
		emit(name, data)
		if name == EventPresetsChanged || name == EventSettingsChanged {
			s.refreshClubs(context.Background())
		}
	}
}
