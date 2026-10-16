// Package tournament holds the tournament aggregate: its event types, the
// projection of the log into state, and command validation.
//
// It is pure: commands take the current state and return the change to
// append, or an error code. The desktop app appends and broadcasts.
package tournament

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// Event types. Once released, a type never changes meaning.
const (
	TypeCreated events.Type = "TournamentCreated"
	TypeRenamed events.Type = "TournamentRenamed"
)

// MaxNameLength is the longest tournament name, in characters.
const MaxNameLength = 100

// Created is the payload of TypeCreated, always the first event of a file.
type Created struct {
	Name string `json:"name"`
}

// Renamed is the payload of TypeRenamed.
type Renamed struct {
	Name string `json:"name"`
}

// Change is the event a command produces. A zero Change means the command
// changes nothing and nothing is appended.
type Change struct {
	Type events.Type
	Data any
}

// None reports whether the change is empty.
func (c Change) None() bool { return c.Type == "" }

// State is the tournament as projected from its log.
type State struct {
	Name string
	// Version is the seq of the latest event, undo events included.
	Version uint64
	// Categories in the order they were added: the first is slot 1.
	Categories []engine.Category
	// Entrants in registration order.
	Entrants []Entrant
	// lastBib is the highest bib ever assigned, removed entrants included,
	// so a number is never handed out twice.
	lastBib int
}

// nextSeq is the seq the next appended event will get. The desktop app is
// the only writer, so IDs derived from it are unique and replay identically.
func (s State) nextSeq() uint64 { return s.Version + 1 }

// Project replays the events in effect.
func Project(log []events.Event) (State, error) {
	var s State
	if len(log) > 0 {
		s.Version = log[len(log)-1].Seq
	}
	for _, e := range events.Active(log) {
		if err := s.apply(e); err != nil {
			return State{}, err
		}
	}
	return s, nil
}

func (s *State) apply(e events.Event) error {
	switch e.Type {
	case TypeCreated:
		return decodeInto(e, func(p Created) { s.Name = p.Name })
	case TypeRenamed:
		return decodeInto(e, func(p Renamed) { s.Name = p.Name })
	case TypeCategoryAdded:
		return decodeInto(e, s.applyCategoryAdded)
	case TypeCategoryRemoved:
		return decodeInto(e, s.applyCategoryRemoved)
	case TypeEntrantRegistered:
		return decodeInto(e, s.applyEntrantRegistered)
	case TypeEntrantUpdated:
		return decodeInto(e, s.applyEntrantUpdated)
	case TypeEntrantRemoved:
		return decodeInto(e, s.applyEntrantRemoved)
	case TypePlayerCheckedIn:
		return decodeInto(e, s.applyPlayerCheckedIn)
	case TypeCheckInCancelled:
		return decodeInto(e, s.applyCheckInCancelled)
	}
	return nil
}

func decodeInto[T any](e events.Event, apply func(T)) error {
	var p T
	if err := json.Unmarshal(e.Data, &p); err != nil {
		return err
	}
	apply(p)
	return nil
}

// Undoable reports whether the desk may undo an event of this type. The
// creation of the tournament cannot be undone.
func Undoable(t events.Type) bool {
	return t != TypeCreated
}

// NormalizeName trims a tournament name and checks its length.
func NormalizeName(name string) (string, error) {
	name = collapseSpaces(name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
		return "", engine.NewError(engine.CodeInvalidName)
	}
	return name, nil
}

// Rename validates a rename.
func (s State) Rename(name string) (Change, error) {
	name, err := NormalizeName(name)
	if err != nil || name == s.Name {
		return Change{}, err
	}
	return Change{TypeRenamed, Renamed{Name: name}}, nil
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
