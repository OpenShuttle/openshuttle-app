// Package store handles SQLite access: the app database (app.db) and
// tournament files (.openshuttle), schema migrations and backups.
//
// Every database runs in WAL mode with synchronous=FULL: once a write
// returns, it survives a crash or a power cut.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// SQLite application_id values, stored in the file header, so a file can be
// recognized whatever its name.
const (
	appIDTournament int32 = 0x4F534854 // "OSHT"
	appIDApp        int32 = 0x4F534150 // "OSAP"
)

// migration upgrades a schema by one version, inside a transaction.
type migration func(ctx context.Context, tx *sql.Tx) error

// openDB opens a SQLite file with the pragmas every OpenShuttle database uses.
func openDB(path string) (*sql.DB, error) {
	q := strings.Join([]string{
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(FULL)",
		"_pragma=foreign_keys(1)",
		"_pragma=busy_timeout(5000)",
		"_txlock=immediate",
	}, "&")
	// No "file:" prefix: the path is then passed as is, not parsed as a URI,
	// so Windows paths and names with '#' or '%' work.
	db, err := sql.Open("sqlite", path+"?"+q)
	if err != nil {
		return nil, err
	}
	// One desk, one writer: a single connection keeps writes ordered.
	db.SetMaxOpenConns(1)
	return db, nil
}

// prepare checks the file type, then migrates it to the latest schema. A new,
// empty database is stamped with appID.
func prepare(ctx context.Context, db *sql.DB, appID int32, migrations []migration) error {
	var id int32
	if err := db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&id); err != nil {
		// Not a SQLite file at all.
		return engine.NewError(engine.CodeNotATournamentFile)
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}

	switch {
	case id == 0 && version == 0:
		if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", appID)); err != nil {
			return err
		}
	case id != appID:
		return engine.NewError(engine.CodeNotATournamentFile)
	}
	if version > len(migrations) {
		return engine.NewError(engine.CodeFileTooNew)
	}
	if version == len(migrations) {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i := version; i < len(migrations); i++ {
		if err := migrations[i](ctx, tx); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", len(migrations))); err != nil {
		return err
	}
	return tx.Commit()
}

func execAll(statements ...string) migration {
	return func(ctx context.Context, tx *sql.Tx) error {
		for _, s := range statements {
			if _, err := tx.ExecContext(ctx, s); err != nil {
				return err
			}
		}
		return nil
	}
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}
