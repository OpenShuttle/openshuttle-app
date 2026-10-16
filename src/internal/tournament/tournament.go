// Package tournament holds the tournament aggregate: its event types, the
// projection of the log into state, and command validation.
//
// It is pure: commands take the current state and return the event to
// append, or an error code. The desktop app appends and broadcasts.
package tournament

import (
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

// State is the tournament as projected from its log.
type State struct {
	Name string
	// Version is the seq of the latest event, undo events included.
	Version uint64
}

// Project replays the events in effect.
func Project(log []events.Event) (State, error) {
	var s State
	if len(log) > 0 {
		s.Version = log[len(log)-1].Seq
	}
	for _, e := range events.Active(log) {
		switch e.Type {
		case TypeCreated:
			var p Created
			if err := e.Decode(&p); err != nil {
				return State{}, err
			}
			s.Name = p.Name
		case TypeRenamed:
			var p Renamed
			if err := e.Decode(&p); err != nil {
				return State{}, err
			}
			s.Name = p.Name
		}
	}
	return s, nil
}

// Undoable reports whether the desk may undo an event of this type. The
// creation of the tournament cannot be undone.
func Undoable(t events.Type) bool {
	return t != TypeCreated
}

// NormalizeName trims a tournament name and checks its length.
func NormalizeName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
		return "", engine.NewError(engine.CodeInvalidName)
	}
	return name, nil
}

// Rename validates a rename. It returns ok=false when the name is unchanged:
// nothing needs to be appended.
func Rename(s State, name string) (Renamed, bool, error) {
	name, err := NormalizeName(name)
	if err != nil {
		return Renamed{}, false, err
	}
	if name == s.Name {
		return Renamed{}, false, nil
	}
	return Renamed{Name: name}, true, nil
}
