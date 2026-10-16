package tournament

import (
	"errors"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// game drives commands through a log, like the desktop service does.
type game struct {
	t   *testing.T
	log []events.Event
	now time.Time // time of the next events
}

func newGame(t *testing.T, categories ...engine.Category) *game {
	g := &game{t: t, now: t0}
	g.append(Change{TypeCreated, Created{Name: "Plume"}})
	for _, c := range categories {
		g.do(func(s State) (Change, error) { return s.AddCategory(c) })
	}
	return g
}

func (g *game) state() State {
	g.t.Helper()
	s, err := Project(g.log)
	if err != nil {
		g.t.Fatal(err)
	}
	return s
}

func (g *game) append(ch Change) {
	g.t.Helper()
	d, err := events.NewDraft(ch.Type, ch.Data, g.now, "Julie")
	if err != nil {
		g.t.Fatal(err)
	}
	g.log = append(g.log, events.Event{Seq: uint64(len(g.log) + 1), Type: d.Type, Data: d.Data, At: d.At})
}

// do runs a command that must succeed and appends its change, if any.
func (g *game) do(cmd func(State) (Change, error)) Change {
	g.t.Helper()
	ch, err := cmd(g.state())
	if err != nil {
		g.t.Fatalf("command failed: %v", err)
	}
	if !ch.None() {
		g.append(ch)
	}
	return ch
}

// fails runs a command that must fail with code.
func (g *game) fails(code engine.Code, cmd func(State) (Change, error)) {
	g.t.Helper()
	_, err := cmd(g.state())
	var e *engine.Error
	if !errors.As(err, &e) || e.Code != code {
		g.t.Fatalf("error = %v, want %s", err, code)
	}
}

func (g *game) undo() {
	g.t.Helper()
	d, err := events.UndoLast(g.log, Undoable, t0, "Julie")
	if err != nil {
		g.t.Fatal(err)
	}
	g.log = append(g.log, events.Event{Seq: uint64(len(g.log) + 1), Type: d.Type, Data: d.Data, At: d.At})
}

// undoSeq undoes a specific event, even an undo.
func (g *game) undoSeq(seq uint64) {
	g.t.Helper()
	g.append(Change{events.TypeUndone, events.Undone{Target: seq}})
}

func (g *game) register(c engine.Category, names ...string) engine.EntrantID {
	g.t.Helper()
	var in []PlayerInput
	for _, n := range names {
		in = append(in, PlayerInput{FirstName: n, LastName: "Martin", Club: "BC Lyon"})
	}
	ch := g.do(func(s State) (Change, error) { return s.RegisterEntrant(c, in) })
	return ch.Data.(EntrantRegistered).ID
}

func (g *game) entrant(id engine.EntrantID) Entrant {
	g.t.Helper()
	e, ok := g.state().Entrant(id)
	if !ok {
		g.t.Fatalf("entrant %s not found", id)
	}
	return e
}
