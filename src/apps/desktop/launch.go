package main

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/instances"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

// EventTournamentLaunched is emitted when a file given to the app at launch
// (or by macOS to the running app) has been opened or refused: the main
// window takes the result with TakeLaunchResult.
const EventTournamentLaunched = "tournament.launched"

// LaunchResult is a tournament file the app was asked to open from outside
// (a double-click on it) and how opening it went.
type LaunchResult struct {
	Path string `json:"path"`
	// Error is nil when the file is open.
	Error *engine.Error `json:"error"`
}

// launchPath is the tournament file among the command-line arguments, as
// Windows and Linux pass it on a double-click; "" when there is none.
func launchPath(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && strings.EqualFold(filepath.Ext(a), store.TournamentExtension) {
			return a
		}
	}
	return ""
}

// openLaunchFile opens a file given to the app from outside, as Open from
// home does (recent list, trash, checks, lock). Its result waits for the
// main window, which takes it with TakeLaunchResult. A file open in another
// instance brings that one to the front (OpenTournament) and is reported
// here as FILE_IN_USE.
func (s *TournamentService) openLaunchFile(ctx context.Context, path string) {
	res := &LaunchResult{Path: path}
	s.mu.Lock()
	open := s.file != nil && sameFile(s.file.Path(), path)
	s.mu.Unlock()
	if !open {
		if err := s.OpenTournament(ctx, path); err != nil {
			res.Error = asEngineError(err)
		}
	}
	s.mu.Lock()
	s.launched = res
	s.mu.Unlock()
	s.emit(EventTournamentLaunched, path)
}

// TakeLaunchResult returns the latest file opened from outside, once; nil
// when there is none.
func (s *TournamentService) TakeLaunchResult() *LaunchResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := s.launched
	s.launched = nil
	return res
}

// inUse tells whether err refuses a file open in another instance.
func inUse(err error) bool {
	var e *engine.Error
	return errors.As(err, &e) && e.Code == engine.CodeFileInUse
}

// asEngineError keeps an error's code; any other error is logged and
// becomes INTERNAL, as marshalError does for a call.
func asEngineError(err error) *engine.Error {
	var e *engine.Error
	if errors.As(err, &e) {
		return e
	}
	slog.Error("open launch file", "err", err)
	return engine.NewError(engine.CodeInternal)
}

// fileHolder lets the other instances of the app find the one holding a
// tournament file, to bring it to the front.
type fileHolder interface {
	// Hold answers for path until the returned func is called.
	Hold(path string) (release func())
	// Raise asks the instance holding path to come to the front; false when
	// none answers.
	Raise(path string) bool
}

// noHolder is the holder of a service built without one (tests).
type noHolder struct{}

func (noHolder) Hold(string) func() { return func() {} }
func (noHolder) Raise(string) bool  { return false }

// socketHolder answers through a local socket per held file
// (internal/instances); raise brings this instance's main window forward.
type socketHolder struct {
	rendezvous instances.Rendezvous
	raise      func()
}

func (h socketHolder) Hold(path string) func() {
	stop, err := h.rendezvous.Hold(path, h.raise)
	if err != nil {
		// Only the bringing to the front is lost: the lock still refuses
		// the file to the other instances.
		slog.Warn("answer for tournament file", "path", path, "err", err)
		return func() {}
	}
	return stop
}

func (h socketHolder) Raise(path string) bool {
	return h.rendezvous.Raise(path)
}
