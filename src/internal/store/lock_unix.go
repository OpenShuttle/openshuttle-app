//go:build unix

package store

import (
	"errors"
	"os"
	"syscall"
)

func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
}

// tryLock takes an exclusive flock without waiting. flock belongs to the open
// file, so two opens in one process exclude each other too.
func tryLock(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EWOULDBLOCK):
			return errLocked
		default:
			return err
		}
	}
}

// releaseLock deletes the lock file while still holding the lock, so whoever
// opened it meanwhile sees it is gone (lockTournament retries), then closes
// it, which releases the lock.
func releaseLock(f *os.File, path string) error {
	_ = os.Remove(path)
	return f.Close()
}
