// Package events defines the tournament event log: the event envelope,
// undo, and the set of events that are still in effect.
//
// The package is pure: no I/O and no clock reads. Storage lives in
// internal/store.
package events

import (
	"encoding/json"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// Type names an event. Types are stored in tournament files: once released,
// a type never changes meaning.
type Type string

// Event is one entry of a tournament log.
type Event struct {
	// Seq is the 1-based position in the log. The latest Seq is the
	// tournament version.
	Seq  uint64
	Type Type
	Data json.RawMessage
	At   time.Time
	// Operator is the name set in Settings by whoever was at the desk.
	Operator string
}

// Draft is an event not yet appended. The store assigns its Seq.
type Draft struct {
	Type     Type
	Data     json.RawMessage
	At       time.Time
	Operator string
}

// NewDraft encodes data as the event payload.
func NewDraft(t Type, data any, at time.Time, operator string) (Draft, error) {
	if t == "" {
		return Draft{}, engine.NewError(engine.CodeInvalidEvent)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return Draft{}, err
	}
	return Draft{Type: t, Data: raw, At: at.UTC(), Operator: operator}, nil
}

// Decode unmarshals the event payload into v.
func (e Event) Decode(v any) error {
	return json.Unmarshal(e.Data, v)
}
