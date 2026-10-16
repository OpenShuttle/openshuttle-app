package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
)

// appMigrations is the schema history of app.db. Append only.
var appMigrations = []migration{
	execAll(`CREATE TABLE settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	) STRICT`),
}

// AppDB is the app database: player and club registry, presets, settings.
type AppDB struct {
	db *sql.DB
}

// OpenAppDB opens app.db, creating it and its folder if needed.
func OpenAppDB(ctx context.Context, path string) (*AppDB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	if err := prepare(ctx, db, appIDApp, appMigrations); err != nil {
		db.Close()
		return nil, err
	}
	return &AppDB{db: db}, nil
}

// Close closes the database.
func (a *AppDB) Close() error { return a.db.Close() }

// Setting returns the value of a setting, and false when it is not set.
func (a *AppDB) Setting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := a.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetSetting stores the value of a setting.
func (a *AppDB) SetSetting(ctx context.Context, key, value string) error {
	_, err := a.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
