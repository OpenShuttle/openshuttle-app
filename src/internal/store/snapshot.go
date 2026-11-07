package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SnapshotExtension ends the readable state written beside a tournament
// file: "Tournament.openshuttle" has "Tournament.html".
const SnapshotExtension = ".html"

// SnapshotPath is where the readable state of a tournament file goes.
func SnapshotPath(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + SnapshotExtension
}

// WriteSnapshot replaces the readable state of a tournament file. It writes
// a temporary file first, then renames it: a crash while writing leaves the
// previous state whole.
func WriteSnapshot(path string, content []byte) error {
	dest := SnapshotPath(path)
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".*.tmp")
	if err != nil {
		return err
	}
	_, err = tmp.Write(content)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), dest)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}

// removeSnapshot deletes the readable state of a tournament file, if any.
func removeSnapshot(path string) error {
	if err := os.Remove(SnapshotPath(path)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
