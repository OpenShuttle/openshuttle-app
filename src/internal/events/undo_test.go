package events

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

var t0 = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

// buildLog turns a compact description into a log: "x" is a plain action,
// a number is an undo of that seq.
func buildLog(t *testing.T, steps ...any) []Event {
	t.Helper()
	var log []Event
	for i, step := range steps {
		seq := uint64(i + 1)
		e := Event{Seq: seq, Type: "ScoreRecorded", Data: json.RawMessage(`{}`), At: t0}
		if target, ok := step.(int); ok {
			d, err := NewDraft(TypeUndone, Undone{Target: uint64(target)}, t0, "Julie")
			if err != nil {
				t.Fatal(err)
			}
			e.Type, e.Data = d.Type, d.Data
		}
		log = append(log, e)
	}
	return log
}

func seqs(log []Event) []uint64 {
	out := []uint64{}
	for _, e := range log {
		out = append(out, e.Seq)
	}
	return out
}

func TestActive(t *testing.T) {
	tests := []struct {
		name  string
		steps []any
		want  []uint64
	}{
		{"empty", nil, []uint64{}},
		{"no undo", []any{"x", "x", "x"}, []uint64{1, 2, 3}},
		{"undo last", []any{"x", "x", 2}, []uint64{1}},
		{"undo an older action", []any{"x", "x", 1}, []uint64{2}},
		{"undo twice", []any{"x", "x", 2, 1}, []uint64{}},
		{"undo of undo restores", []any{"x", "x", 2, 3}, []uint64{1, 2}},
		{"undo of undo of undo", []any{"x", "x", 2, 3, 4}, []uint64{1}},
		{"action after undo", []any{"x", 1, "x"}, []uint64{3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := seqs(Active(buildLog(t, tt.steps...)))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Active() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUndoLast(t *testing.T) {
	tests := []struct {
		name   string
		steps  []any
		target uint64
	}{
		{"latest action", []any{"x", "x"}, 2},
		{"skips undone actions", []any{"x", "x", 2}, 1},
		{"restored action is undoable again", []any{"x", "x", 2, 3}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := UndoLast(buildLog(t, tt.steps...), nil, t0, "Julie")
			if err != nil {
				t.Fatal(err)
			}
			var u Undone
			if err := json.Unmarshal(d.Data, &u); err != nil {
				t.Fatal(err)
			}
			if d.Type != TypeUndone || u.Target != tt.target {
				t.Errorf("UndoLast() = %s %d, want %s %d", d.Type, u.Target, TypeUndone, tt.target)
			}
			if d.Operator != "Julie" || !d.At.Equal(t0) {
				t.Errorf("UndoLast() operator/time = %q %v", d.Operator, d.At)
			}
		})
	}
}

func TestUndoLastNothing(t *testing.T) {
	for _, steps := range [][]any{nil, {"x", 1}} {
		_, err := UndoLast(buildLog(t, steps...), nil, t0, "")
		var e *engine.Error
		if !errors.As(err, &e) || e.Code != engine.CodeNothingToUndo {
			t.Errorf("UndoLast(%v) error = %v, want %s", steps, err, engine.CodeNothingToUndo)
		}
	}
}

func TestUndoLastSkipsNonUndoable(t *testing.T) {
	log := buildLog(t, "x", "x")
	log[1].Type = "TournamentCreated"
	undoable := func(t Type) bool { return t != "TournamentCreated" }
	if e, ok := LastUndoable(log, undoable); !ok || e.Seq != 1 {
		t.Errorf("LastUndoable() = %d, %v, want 1", e.Seq, ok)
	}
	log[0].Type = "TournamentCreated"
	if _, ok := LastUndoable(log, undoable); ok {
		t.Error("LastUndoable() found an action, want none")
	}
}

func TestIsActive(t *testing.T) {
	log := buildLog(t, "x", "x", 1)
	if IsActive(log, 1) || !IsActive(log, 2) {
		t.Errorf("IsActive: seq 1 should be undone, seq 2 active")
	}
}

func TestNewDraft(t *testing.T) {
	paris := time.FixedZone("Paris", 2*3600)
	d, err := NewDraft("ScoreRecorded", map[string]int{"a": 21}, t0.In(paris), "Julie")
	if err != nil {
		t.Fatal(err)
	}
	if string(d.Data) != `{"a":21}` || d.At.Location() != time.UTC {
		t.Errorf("NewDraft() = %s at %v", d.Data, d.At)
	}
	if _, err := NewDraft("", nil, t0, ""); err == nil {
		t.Error("NewDraft with empty type should fail")
	}
}
