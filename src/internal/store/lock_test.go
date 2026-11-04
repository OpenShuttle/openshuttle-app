package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func exists(t *testing.T, path string) bool {
	t.Helper()
	ok, err := fileExists(path)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestTournamentOpenOnce(t *testing.T) {
	ctx := context.Background()
	f, path := newTournament(t)

	if !exists(t, path+LockSuffix) {
		t.Fatal("no lock file beside the open tournament")
	}
	_, err := OpenTournament(ctx, path)
	wantCode(t, err, engine.CodeFileInUse)
	if !TournamentInUse(path) {
		t.Fatal("TournamentInUse = false for the open file")
	}
	// The same file through a symbolic link shares the lock.
	link := filepath.Join(t.TempDir(), "link"+TournamentExtension)
	if err := os.Symlink(path, link); err == nil {
		_, err = OpenTournament(ctx, link)
		wantCode(t, err, engine.CodeFileInUse)
	}
	wantCode(t, RemoveTournament(path), engine.CodeFileInUse)
	if !exists(t, path) {
		t.Fatal("RemoveTournament deleted an open file")
	}

	// A backup and a read go on while it is open.
	dest := filepath.Join(t.TempDir(), "copy"+TournamentExtension)
	if err := f.Backup(ctx, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Append(ctx, draft(t, "ScoreRecorded", map[string]int{"a": 1}, t0)); err != nil {
		t.Fatal(err)
	}
	peek, err := PeekTournament(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if log, err := peek.Events(ctx); err != nil || len(log) != 1 {
		t.Fatalf("peek: %d events, %v", len(log), err)
	}
	if _, err := peek.Append(ctx, draft(t, "ScoreRecorded", map[string]int{"a": 2}, t0)); err == nil {
		t.Fatal("a peeked file accepted a write")
	}
	peek.Close()
	if _, err := f.Append(ctx, draft(t, "ScoreRecorded", map[string]int{"a": 3}, t0)); err != nil {
		t.Fatalf("closing a peek broke the open file: %v", err)
	}

	// The copy is a file of its own.
	c, err := OpenTournament(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if exists(t, path+LockSuffix) {
		t.Fatal("lock file left behind after Close")
	}
	if TournamentInUse(path) {
		t.Fatal("TournamentInUse = true once closed")
	}
	g, err := OpenTournament(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	g.Close()
	if err := RemoveTournament(path); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, path + "-wal", path + "-shm", path + LockSuffix} {
		if exists(t, p) {
			t.Fatalf("%s left behind", filepath.Base(p))
		}
	}
}

func TestTournamentOpenFailureReleasesLock(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "missing"+TournamentExtension)
	_, err := OpenTournament(ctx, path)
	wantCode(t, err, engine.CodeFileNotFound)
	if exists(t, path+LockSuffix) {
		t.Fatal("lock file left behind by a failed open")
	}
}

func TestStaleLockFile(t *testing.T) {
	// A crash leaves the lock file, never the lock.
	f, path := newTournament(t)
	f.Close()
	if err := os.WriteFile(path+LockSuffix, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := OpenTournament(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	g.Close()
}

// The helper process holds a tournament open until its stdin closes.
func TestMain(m *testing.M) {
	if path := os.Getenv("OPENSHUTTLE_HOLD"); path != "" {
		f, err := OpenTournament(context.Background(), path)
		if err != nil {
			fmt.Println("error", err)
			os.Exit(1)
		}
		fmt.Println("ready")
		bufio.NewReader(os.Stdin).ReadString('\n')
		f.Close()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// holdInOtherProcess opens path in another process, as another instance of
// the app would.
func holdInOtherProcess(t *testing.T, path string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "OPENSHUTTLE_HOLD="+path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stdin.Close(); cmd.Process.Kill(); cmd.Wait() })
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(out).ReadString('\n')
	if line != "ready\n" {
		t.Fatalf("helper: %q", line)
	}
	return cmd
}

func TestTournamentOpenInAnotherProcess(t *testing.T) {
	ctx := context.Background()
	f, path := newTournament(t)
	f.Close()
	o, other := newTournament(t)
	o.Close()

	cmd := holdInOtherProcess(t, path)
	_, err := OpenTournament(ctx, path)
	wantCode(t, err, engine.CodeFileInUse)
	wantCode(t, RemoveTournament(path), engine.CodeFileInUse)
	// Another tournament opens alongside.
	o, err = OpenTournament(ctx, other)
	if err != nil {
		t.Fatalf("another file: %v", err)
	}
	o.Close()

	// A killed process releases its lock.
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	g, err := OpenTournament(ctx, path)
	if err != nil {
		t.Fatalf("after the other process died: %v", err)
	}
	g.Close()
}

func TestAppDBUpdateFromTwoInstances(t *testing.T) {
	// Two handles on one app.db, like two instances of the app, each adding
	// to one list: nothing is lost.
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "app.db")
	var dbs [2]*AppDB
	for i := range dbs {
		db, err := OpenAppDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		dbs[i] = db
	}
	const each = 50
	var wg sync.WaitGroup
	for i, db := range dbs {
		wg.Go(func() {
			for n := range each {
				err := db.Update(ctx, func(tx Settings) error {
					raw, _, err := tx.Setting(ctx, "list")
					if err != nil {
						return err
					}
					var list []string
					if raw != "" {
						if err := json.Unmarshal([]byte(raw), &list); err != nil {
							return err
						}
					}
					b, _ := json.Marshal(append(list, fmt.Sprintf("%d-%d", i, n)))
					return tx.SetSetting(ctx, "list", string(b))
				})
				if err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	raw, _, err := dbs[0].Setting(ctx, "list")
	if err != nil {
		t.Fatal(err)
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatal(err)
	}
	slices.Sort(list)
	if len(slices.Compact(list)) != 2*each {
		t.Fatalf("%d entries, want %d", len(list), 2*each)
	}
}

func TestAppDBUpdateRollsBack(t *testing.T) {
	ctx := context.Background()
	db, err := OpenAppDB(ctx, filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = db.Update(ctx, func(tx Settings) error {
		if err := tx.SetSetting(ctx, "k", "v"); err != nil {
			return err
		}
		return engine.NewError(engine.CodeInternal)
	})
	wantCode(t, err, engine.CodeInternal)
	if _, ok, _ := db.Setting(ctx, "k"); ok {
		t.Fatal("a failed update was written")
	}
}
