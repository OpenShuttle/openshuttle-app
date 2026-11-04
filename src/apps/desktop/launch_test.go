package main

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/instances"
)

func TestLaunchPath(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{`C:\Users\Club\Tournoi.openshuttle`}, `C:\Users\Club\Tournoi.openshuttle`},
		{[]string{"/home/club/Open de printemps.OPENSHUTTLE"}, "/home/club/Open de printemps.OPENSHUTTLE"},
		{[]string{"-psn_0_12345", "/Users/club/a.openshuttle"}, "/Users/club/a.openshuttle"},
		{[]string{"notes.txt", "b.openshuttle"}, "b.openshuttle"},
		{[]string{"notes.txt"}, ""},
		{[]string{"a.openshuttle.backup-20261104-090000.zip"}, ""},
		{[]string{"-x.openshuttle"}, ""},
	} {
		if got := launchPath(tc.args); got != tc.want {
			t.Errorf("launchPath(%q) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

// fakeHolder records which files the service answers for and whom it asks
// to come to the front.
type fakeHolder struct {
	mu     sync.Mutex
	held   []string
	raised []string
	// answers are the files another instance holds.
	answers []string
}

func (h *fakeHolder) Hold(path string) func() {
	h.mu.Lock()
	h.held = append(h.held, path)
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.held = slices.DeleteFunc(h.held, func(p string) bool { return p == path })
	}
}

func (h *fakeHolder) Raise(path string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.raised = append(h.raised, path)
	return slices.Contains(h.answers, path)
}

func (h *fakeHolder) holding() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.held)
}

func TestOpenFileIsHeldWhileOpen(t *testing.T) {
	f := newFixture(t)
	h := &fakeHolder{}
	f.svc.holder = h
	a := filepath.Join(f.dir, "a.openshuttle")
	b := filepath.Join(f.dir, "b.openshuttle")
	for _, p := range []string{b, a} {
		if err := f.svc.CreateTournamentAt(f.ctx, p, filepath.Base(p)); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.holding(); !slices.Equal(got, []string{a}) {
		t.Fatalf("held = %q, want only a", got)
	}
	if err := f.svc.OpenTournament(f.ctx, b); err != nil {
		t.Fatal(err)
	}
	if got := h.holding(); !slices.Equal(got, []string{b}) {
		t.Fatalf("held = %q, want only b", got)
	}
	f.svc.CloseTournament()
	if got := h.holding(); len(got) != 0 {
		t.Fatalf("held after close = %q", got)
	}
}

func TestOpenLaunchFile(t *testing.T) {
	f := newFixture(t)
	a := filepath.Join(f.dir, "a.openshuttle")
	if err := f.svc.CreateTournamentAt(f.ctx, a, "Spring open"); err != nil {
		t.Fatal(err)
	}
	// In the trash, which closes it.
	if err := f.svc.TrashTournament(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	f.rec.events = nil

	if f.svc.TakeLaunchResult() != nil {
		t.Fatal("a result before any launch")
	}
	f.svc.openLaunchFile(f.ctx, a)
	if v := f.svc.GetTournamentView(); !v.Open || v.Path != a {
		t.Fatalf("view = %+v", v)
	}
	if got := f.rec.last(); got.name != EventTournamentLaunched || got.data != a {
		t.Fatalf("last event = %+v", got)
	}
	if res := f.svc.TakeLaunchResult(); res == nil || res.Path != a || res.Error != nil {
		t.Fatalf("result = %+v", res)
	}
	if f.svc.TakeLaunchResult() != nil {
		t.Fatal("a result taken twice")
	}
	// Opened as from home: out of the trash, back in the recent list.
	if recent, _ := f.svc.ListRecentTournaments(f.ctx); len(recent) != 1 || recent[0].Path != a {
		t.Fatalf("recent = %+v", recent)
	}
	if trash, _ := f.svc.ListTrashedTournaments(f.ctx); len(trash) != 0 {
		t.Fatalf("trash = %+v", trash)
	}

	// The open file again (macOS hands it to the running app): kept as it is.
	version := f.svc.GetTournamentView().Version
	f.svc.openLaunchFile(f.ctx, a)
	if v := f.svc.GetTournamentView(); !v.Open || v.Version != version {
		t.Fatalf("view = %+v", v)
	}
	if res := f.svc.TakeLaunchResult(); res == nil || res.Error != nil {
		t.Fatalf("result = %+v", res)
	}

	// A file that is gone: the error waits for the home screen.
	gone := filepath.Join(f.dir, "gone.openshuttle")
	f.svc.openLaunchFile(f.ctx, gone)
	res := f.svc.TakeLaunchResult()
	if res == nil || res.Path != gone || res.Error == nil || res.Error.Code != engine.CodeFileNotFound {
		t.Fatalf("result = %+v", res)
	}
	if v := f.svc.GetTournamentView(); !v.Open || v.Path != a {
		t.Fatalf("a failed launch closed the tournament: %+v", v)
	}
}

func TestOpenLaunchFileHeldElsewhere(t *testing.T) {
	f := newFixture(t)
	a := filepath.Join(f.dir, "a.openshuttle")
	if err := f.svc.CreateTournamentAt(f.ctx, a, "Spring open"); err != nil {
		t.Fatal(err)
	}
	other := f.otherInstance(t)
	h := &fakeHolder{answers: []string{a}}
	other.holder = h

	other.openLaunchFile(f.ctx, a)
	res := other.TakeLaunchResult()
	if res == nil || res.Error == nil || res.Error.Code != engine.CodeFileInUse {
		t.Fatalf("result = %+v", res)
	}
	if !slices.Equal(h.raised, []string{a}) {
		t.Fatalf("raised = %q, want a", h.raised)
	}
	// Open from home, the same.
	wantCode(t, other.OpenTournament(f.ctx, a), engine.CodeFileInUse)
	if len(h.raised) != 2 {
		t.Fatalf("raised = %q", h.raised)
	}
}

// Two instances through real sockets: the one holding a file comes to the
// front when the other tries to open it, and only while it holds it.
func TestInstanceHoldingAFileComesToTheFront(t *testing.T) {
	sockets, err := os.MkdirTemp("", "osd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockets) })
	rv := instances.Rendezvous{Dir: sockets}

	f := newFixture(t)
	var raised atomic.Int32
	f.svc.holder = socketHolder{rendezvous: rv, raise: func() { raised.Add(1) }}
	other := f.otherInstance(t)
	other.holder = socketHolder{rendezvous: rv, raise: func() { t.Error("the other instance was raised") }}

	a := filepath.Join(f.dir, "a.openshuttle")
	if err := f.svc.CreateTournamentAt(f.ctx, a, "Spring open"); err != nil {
		t.Fatal(err)
	}
	// What a double-click does before starting the app.
	if !rv.Raise(a) || raised.Load() != 1 {
		t.Fatal("the holder did not answer")
	}
	wantCode(t, other.OpenTournament(f.ctx, a), engine.CodeFileInUse)
	if raised.Load() != 2 {
		t.Fatalf("raised %d times, want 2", raised.Load())
	}

	// Closed here, the other instance opens it and answers in turn.
	f.svc.CloseTournament()
	if rv.Raise(a) {
		t.Fatal("a closed file still answers")
	}
	other.holder = socketHolder{rendezvous: rv, raise: func() {}}
	if err := other.OpenTournament(f.ctx, a); err != nil {
		t.Fatal(err)
	}
	if !rv.Raise(a) {
		t.Fatal("the new holder did not answer")
	}
}
