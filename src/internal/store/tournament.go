package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// TournamentExtension is the extension of tournament files.
const TournamentExtension = ".openshuttle"

// tournamentMigrations is the schema history of tournament files. Append
// only: a released migration never changes.
var tournamentMigrations = []migration{
	execAll(`CREATE TABLE events (
		seq      INTEGER PRIMARY KEY,
		type     TEXT NOT NULL,
		data     TEXT NOT NULL,
		at       TEXT NOT NULL,
		operator TEXT NOT NULL DEFAULT ''
	) STRICT`),
	// Version 2: the flexible format (ADR 4) changed the event types. Files
	// written before it are refused rather than read wrong.
	refuseOldEvents,
}

// refuseOldEvents fails on a file that already holds events: they were
// written by the first format.
func refuseOldEvents(ctx context.Context, tx *sql.Tx) error {
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM events").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return engine.NewError(engine.CodeFileTooOld)
	}
	return nil
}

// TournamentFile is an open tournament file: a self-contained SQLite
// database holding the event log.
type TournamentFile struct {
	db   *sql.DB
	path string
}

// CreateTournament creates a new tournament file. It fails with FILE_EXISTS
// rather than overwrite a file.
func CreateTournament(ctx context.Context, path string) (*TournamentFile, error) {
	exists, err := fileExists(path)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, engine.NewError(engine.CodeFileExists)
	}
	return openTournament(ctx, path)
}

// OpenTournament opens an existing tournament file and migrates it.
func OpenTournament(ctx context.Context, path string) (*TournamentFile, error) {
	exists, err := fileExists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, engine.NewError(engine.CodeFileNotFound)
	}
	return openTournament(ctx, path)
}

func openTournament(ctx context.Context, path string) (*TournamentFile, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	if err := prepare(ctx, db, appIDTournament, tournamentMigrations); err != nil {
		db.Close()
		return nil, err
	}
	return &TournamentFile{db: db, path: path}, nil
}

// Path returns the file path.
func (f *TournamentFile) Path() string { return f.path }

// Close closes the file.
func (f *TournamentFile) Close() error { return f.db.Close() }

// Append writes an event at the end of the log and returns it with its seq.
func (f *TournamentFile) Append(ctx context.Context, d events.Draft) (events.Event, error) {
	if d.Type == "" || len(d.Data) == 0 {
		return events.Event{}, engine.NewError(engine.CodeInvalidEvent)
	}
	at := d.At.UTC()
	var seq uint64
	err := f.db.QueryRowContext(ctx,
		`INSERT INTO events (seq, type, data, at, operator)
		 VALUES ((SELECT COALESCE(MAX(seq), 0) + 1 FROM events), ?, ?, ?, ?)
		 RETURNING seq`,
		string(d.Type), string(d.Data), at.Format(time.RFC3339Nano), d.Operator,
	).Scan(&seq)
	if err != nil {
		return events.Event{}, err
	}
	return events.Event{Seq: seq, Type: d.Type, Data: d.Data, At: at, Operator: d.Operator}, nil
}

// Events returns the whole log, in order.
func (f *TournamentFile) Events(ctx context.Context) ([]events.Event, error) {
	return f.EventsSince(ctx, 0)
}

// EventsSince returns the events after the given seq, in order.
func (f *TournamentFile) EventsSince(ctx context.Context, seq uint64) ([]events.Event, error) {
	rows, err := f.db.QueryContext(ctx,
		`SELECT seq, type, data, at, operator FROM events WHERE seq > ? ORDER BY seq`, seq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []events.Event
	for rows.Next() {
		var (
			e       events.Event
			typ, at string
			data    string
		)
		if err := rows.Scan(&e.Seq, &typ, &data, &at, &e.Operator); err != nil {
			return nil, err
		}
		e.Type = events.Type(typ)
		e.Data = []byte(data)
		if e.At, err = time.Parse(time.RFC3339Nano, at); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Version returns the seq of the latest event, 0 for an empty log.
func (f *TournamentFile) Version(ctx context.Context) (uint64, error) {
	var v uint64
	err := f.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM events`).Scan(&v)
	return v, err
}
