package store

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// LockSuffix names the lock file kept beside an open tournament file, like
// SQLite's -wal and -shm: "<name>.openshuttle-lock".
const LockSuffix = "-lock"

// errLocked is returned by the platform lock when another handle holds it.
var errLocked = errors.New("locked")

// fileLock is an exclusive OS lock on the lock file of a tournament: flock on
// macOS and Linux, LockFileEx on Windows. The OS releases it when the handle
// is closed or the process dies, so a crash or a power cut never leaves a
// tournament unopenable; a lock file left behind is only reused.
//
// The lock is taken on a sibling file rather than on the database itself:
// SQLite keeps its own locks on the database, and a second handle on it
// would interfere with them (on Unix, closing any handle of a file drops the
// process's fcntl locks; on macOS, flock and fcntl locks conflict).
type fileLock struct {
	path string
	f    *os.File
}

// lockPath is the lock file of a tournament file. Symbolic links are
// resolved, so two paths to one file share one lock.
func lockPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return path + LockSuffix
}

// lockTournament takes the lock of a tournament file, or fails with
// FILE_IN_USE when another instance of the app (or another open in this one)
// holds it.
func lockTournament(path string) (*fileLock, error) {
	lp := lockPath(path)
	// A holder closing the file deletes the lock file: a handle opened just
	// before may lock a file that is no longer at lp. Check it and retry.
	for range 10 {
		f, err := openLockFile(lp)
		if err != nil {
			return nil, err
		}
		err = tryLock(f)
		if err != nil && !errors.Is(err, errLocked) {
			f.Close()
			return nil, err
		}
		same := sameFileAt(f, lp)
		if err != nil {
			f.Close()
			if same {
				return nil, engine.NewError(engine.CodeFileInUse)
			}
			continue
		}
		if same {
			return &fileLock{path: lp, f: f}, nil
		}
		f.Close()
	}
	return nil, engine.NewError(engine.CodeFileInUse)
}

// sameFileAt tells whether f is still the file at path.
func sameFileAt(f *os.File, path string) bool {
	a, err := f.Stat()
	if err != nil {
		return false
	}
	b, err := os.Stat(path)
	if err != nil {
		return false
	}
	return os.SameFile(a, b)
}

// release deletes the lock file and releases the lock. Deleting is best
// effort: a lock file left behind does no harm.
func (l *fileLock) release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := releaseLock(l.f, l.path)
	l.f = nil
	return err
}

// TournamentInUse tells whether a tournament file is open, in this instance
// or another one.
func TournamentInUse(path string) bool {
	l, err := lockTournament(path)
	if err != nil {
		var e *engine.Error
		return errors.As(err, &e) && e.Code == engine.CodeFileInUse
	}
	l.release()
	return false
}
