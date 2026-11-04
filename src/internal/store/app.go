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

// Settings reads and writes settings: the AppDB itself, or a transaction of
// it (Update).
type Settings interface {
	Setting(ctx context.Context, key string) (string, bool, error)
	SetSetting(ctx context.Context, key, value string) error
}

// querier is what settings need from a database or a transaction.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Setting returns the value of a setting, and false when it is not set.
func (a *AppDB) Setting(ctx context.Context, key string) (string, bool, error) {
	return setting(ctx, a.db, key)
}

// SetSetting stores the value of a setting.
func (a *AppDB) SetSetting(ctx context.Context, key, value string) error {
	return setSetting(ctx, a.db, key, value)
}

// Update runs fn in a write transaction: app.db is shared by every instance of
// the app, so a setting read, changed and written back (a list) must not lose
// what another instance wrote meanwhile. The transaction starts with the
// write lock (BEGIN IMMEDIATE); another instance waits for it (busy_timeout).
// fn must only use tx: the AppDB has a single connection.
func (a *AppDB) Update(ctx context.Context, fn func(tx Settings) error) error {
	t, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer t.Rollback()
	if err := fn(settingsTx{t}); err != nil {
		return err
	}
	return t.Commit()
}

type settingsTx struct{ tx *sql.Tx }

func (t settingsTx) Setting(ctx context.Context, key string) (string, bool, error) {
	return setting(ctx, t.tx, key)
}

func (t settingsTx) SetSetting(ctx context.Context, key, value string) error {
	return setSetting(ctx, t.tx, key, value)
}

func setting(ctx context.Context, q querier, key string) (string, bool, error) {
	var v string
	err := q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func setSetting(ctx context.Context, q querier, key, value string) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
