package main

import (
	"log/slog"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

// WriteSnapshot writes the readable state of the open tournament beside its
// file ("Tournament.html"): pools, brackets, courts and matches left, for
// the desk to go on by hand if the app or the computer fails. The frontend
// builds it after each action (it holds the translations); version is the
// tournament version it was built from, and an outdated one is skipped. A
// failure is logged: the tournament file stays the reference.
func (s *TournamentService) WriteSnapshot(version uint64, html string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil || version != s.state.Version {
		return nil
	}
	if err := store.WriteSnapshot(s.file.Path(), []byte(html)); err != nil {
		slog.Error("write snapshot", "err", err)
		return engine.NewError(engine.CodeWriteFailed)
	}
	return nil
}
