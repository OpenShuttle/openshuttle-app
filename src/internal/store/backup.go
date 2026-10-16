package store

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

const backupTimeLayout = "20060102-150405"

// Backup writes a consistent copy of the tournament to dest. The copy is a
// regular tournament file that opens directly. dest must not exist.
func (f *TournamentFile) Backup(ctx context.Context, dest string) error {
	exists, err := fileExists(dest)
	if err != nil {
		return err
	}
	if exists {
		return engine.NewError(engine.CodeFileExists)
	}
	_, err = f.db.ExecContext(ctx, "VACUUM INTO ?", dest)
	return err
}

// BackupRotating writes a timestamped backup into dir, then deletes the
// oldest backups of this tournament so that at most keep remain. Backups are
// named "<name>.backup-YYYYMMDD-HHMMSS.openshuttle". It returns the path of
// the new backup. A second backup within the same second is skipped.
func (f *TournamentFile) BackupRotating(ctx context.Context, dir string, now time.Time, keep int) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	prefix := backupPrefix(f.path)
	dest := filepath.Join(dir, prefix+now.UTC().Format(backupTimeLayout)+TournamentExtension)

	exists, err := fileExists(dest)
	if err != nil {
		return "", err
	}
	if !exists {
		if err := f.Backup(ctx, dest); err != nil {
			return "", err
		}
	}
	return dest, pruneBackups(dir, prefix, keep)
}

func backupPrefix(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), TournamentExtension)
	return name + ".backup-"
}

// pruneBackups keeps the newest keep backups. Names sort by time.
func pruneBackups(dir, prefix string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var backups []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, prefix) && strings.HasSuffix(n, TournamentExtension) {
			backups = append(backups, n)
		}
	}
	slices.Sort(backups)
	for len(backups) > max(keep, 1) {
		if err := os.Remove(filepath.Join(dir, backups[0])); err != nil {
			return err
		}
		backups = backups[1:]
	}
	return nil
}
