package events

import (
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// TypeUndone cancels an earlier event. Undoing an undo restores its target.
// Nothing is ever deleted from the log, so history shows both the action and
// its undo.
const TypeUndone Type = "ActionUndone"

// Undone is the payload of TypeUndone.
type Undone struct {
	Target uint64 `json:"target"`
}

// Active returns the events still in effect, in log order: every event not
// cancelled by an active undo. Undo events themselves are left out.
func Active(log []Event) []Event {
	cancelled := cancelledSet(log)
	out := make([]Event, 0, len(log))
	for _, e := range log {
		if e.Type != TypeUndone && !cancelled[e.Seq] {
			out = append(out, e)
		}
	}
	return out
}

// IsActive reports whether the event with the given seq is in effect.
func IsActive(log []Event, seq uint64) bool {
	return !cancelledSet(log)[seq]
}

// LastUndoable returns the latest action still in effect.
func LastUndoable(log []Event) (Event, bool) {
	cancelled := cancelledSet(log)
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		if e.Type != TypeUndone && !cancelled[e.Seq] {
			return e, true
		}
	}
	return Event{}, false
}

// UndoLast returns the draft that cancels the latest action in effect.
func UndoLast(log []Event, at time.Time, operator string) (Draft, error) {
	last, ok := LastUndoable(log)
	if !ok {
		return Draft{}, engine.NewError(engine.CodeNothingToUndo)
	}
	return NewDraft(TypeUndone, Undone{Target: last.Seq}, at, operator)
}

// cancelledSet walks the log backwards: an undo only targets earlier events,
// so by the time an undo is reached, whether it is itself cancelled is known.
func cancelledSet(log []Event) map[uint64]bool {
	cancelled := map[uint64]bool{}
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		if e.Type != TypeUndone || cancelled[e.Seq] {
			continue
		}
		var u Undone
		if err := e.Decode(&u); err == nil && u.Target < e.Seq {
			cancelled[u.Target] = true
		}
	}
	return cancelled
}
