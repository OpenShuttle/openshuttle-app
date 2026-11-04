//go:build windows

package store

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// openLockFile opens the lock file without FILE_SHARE_DELETE: it cannot be
// deleted while another instance holds a handle on it. The file is hidden,
// like the folder's other housekeeping files would be.
func openLockFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_HIDDEN, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

// tryLock takes an exclusive lock on the first byte without waiting. The lock
// belongs to the handle, so two opens in one process exclude each other too.
func tryLock(f *os.File) error {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		return errLocked
	}
	return err
}

// releaseLock closes the lock file, which releases the lock, then deletes it.
// Deleting fails while another instance has it open: it then stays, harmless.
func releaseLock(f *os.File, path string) error {
	err := f.Close()
	_ = os.Remove(path)
	return err
}
