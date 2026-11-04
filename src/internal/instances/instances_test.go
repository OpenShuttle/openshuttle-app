package instances

import (
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// rendezvous is a fresh rendezvous in a short folder: socket paths are
// limited to about 100 bytes, more than t.TempDir gives on macOS.
func rendezvous(t *testing.T) Rendezvous {
	t.Helper()
	dir, err := os.MkdirTemp("", "osi")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return Rendezvous{Dir: filepath.Join(dir, "r")}
}

func TestRaiseReachesTheHolder(t *testing.T) {
	r := rendezvous(t)
	path := filepath.Join(t.TempDir(), "a.openshuttle")
	other := filepath.Join(t.TempDir(), "b.openshuttle")

	if r.Raise(path) {
		t.Fatal("raised with no holder")
	}
	var raised atomic.Int32
	stop, err := r.Hold(path, func() { raised.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	if !r.Raise(path) || raised.Load() != 1 {
		t.Fatalf("raise not answered: %d", raised.Load())
	}
	if r.Raise(other) || raised.Load() != 1 {
		t.Fatal("another file raised the holder")
	}
	stop()
	stop() // twice is harmless
	if r.Raise(path) {
		t.Fatal("raised after stop")
	}
	if _, err := os.Stat(r.Endpoint(path)); !os.IsNotExist(err) {
		t.Fatalf("socket left after stop: %v", err)
	}
}

func TestHoldReplacesASocketLeftByACrash(t *testing.T) {
	r := rendezvous(t)
	path := filepath.Join(t.TempDir(), "a.openshuttle")
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// A crashed holder leaves its socket file, bound by nobody.
	l, err := net.Listen("unix", r.Endpoint(path))
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close()
	if _, err := os.Stat(r.Endpoint(path)); err != nil {
		t.Fatalf("no stale socket to test with: %v", err)
	}
	if r.Raise(path) {
		t.Fatal("a stale socket answered")
	}

	var raised atomic.Int32
	stop, err := r.Hold(path, func() { raised.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if !r.Raise(path) || raised.Load() != 1 {
		t.Fatal("new holder not reached")
	}
}

func TestHoldRefusesALiveHolder(t *testing.T) {
	r := rendezvous(t)
	path := filepath.Join(t.TempDir(), "a.openshuttle")
	stop, err := r.Hold(path, func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if _, err := r.Hold(path, func() {}); err == nil {
		t.Fatal("a live holder was replaced")
	}
	if !r.Raise(path) {
		t.Fatal("first holder lost")
	}
}

func TestEndpointIsOnePerFile(t *testing.T) {
	r := Rendezvous{Dir: t.TempDir()}
	dir := t.TempDir()
	file := filepath.Join(dir, "a.openshuttle")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	linked := os.Symlink(dir, link) == nil

	t.Chdir(dir)
	same := []string{file, "a.openshuttle", filepath.Join(dir, ".", "x", "..", "a.openshuttle")}
	if linked {
		same = append(same, filepath.Join(link, "a.openshuttle"))
	}
	for _, p := range same {
		if r.Endpoint(p) != r.Endpoint(file) {
			t.Errorf("Endpoint(%q) differs from Endpoint(%q)", p, file)
		}
	}
	if r.Endpoint(filepath.Join(dir, "b.openshuttle")) == r.Endpoint(file) {
		t.Error("two files share an endpoint")
	}
}

func TestDefaultIsPerUser(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if got := Default().Dir; filepath.Dir(got) != filepath.Clean("/run/user/1000") {
		t.Errorf("Default().Dir = %q", got)
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	if got := Default().Dir; filepath.Dir(got) != filepath.Clean(os.TempDir()) {
		t.Errorf("Default().Dir = %q", got)
	}
}
