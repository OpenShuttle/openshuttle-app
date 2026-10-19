// Package tournament holds the tournament aggregate: its event types, the
// projection of the log into state, and command validation.
//
// It is pure: commands take the current state and return the change to
// append, or an error code. The desktop app appends and broadcasts.
package tournament

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// Event types. Once released, a type never changes meaning.
const (
	TypeCreated  events.Type = "TournamentCreated"
	TypeRenamed  events.Type = "TournamentRenamed"
	TypeArchived events.Type = "TournamentArchived"
)

// Archived is the payload of TypeArchived: the tournament is closed. It
// stays readable and correctable: nothing is ever locked.
type Archived struct{}

// MaxNameLength is the longest tournament name, in characters.
const MaxNameLength = 100

// Created is the payload of TypeCreated, always the first event of a file.
// Courts, MinRest and TimedPools carry the app's defaults for new
// tournaments; zero values mean the built-in defaults.
type Created struct {
	Name       string `json:"name"`
	Courts     int    `json:"courts,omitempty"`
	MinRest    *int   `json:"minRest,omitempty"`
	TimedPools bool   `json:"timedPools,omitempty"`
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
	Name     string
	Archived bool
	// Version is the seq of the latest event, undo events included.
	Version uint64
	// Categories in the order they were added: the first is slot 1.
	Categories []engine.Category
	// Entrants in registration order.
	Entrants []Entrant
	// lastBib is the highest bib ever assigned, removed entrants included,
	// so a number is never handed out twice.
	lastBib int
	// formats holds the stages of each category, entry stage first.
	formats map[engine.Category][]*Stage
	// formatSet is false while a category keeps its default format.
	formatSet    map[engine.Category]bool
	stageNumbers map[engine.Category]int // highest stage number ever used

	courts    int  // 0: DefaultCourts
	minRest   *int // nil: DefaultMinRest
	priority  string
	matches   []Match
	withdrawn map[engine.EntrantID]string // stage of the forfeit
	lots      map[PoolRef]map[engine.EntrantID]int

	places map[string][]Place // "<category>/<stage>"
	rounds []Round
	// schedule is the schedule setup (mode, reserved courts); scheduleOrder
	// the validated order of the waiting matches, since scheduleFrozenAt.
	schedule         ScheduleSet
	scheduleOrder    []MatchID
	scheduleFrozenAt time.Time
	// scheduleBlocks is the block order set before validation.
	scheduleBlocks []string
	// lastAt is the time of the latest event in effect: "now" for orders
	// that need one without a clock.
	lastAt time.Time
	// defaultTimed makes timed pools the default while a single category plays.
	defaultTimed bool
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
		s.lastAt = later(s.lastAt, e.At)
	}
	s.refreshBrackets()
	return s, nil
}

func (s *State) apply(e events.Event) error {
	switch e.Type {
	case TypeCreated:
		return decodeInto(e, func(p Created) {
			s.Name = p.Name
			s.courts = p.Courts
			s.minRest = p.MinRest
			s.defaultTimed = p.TimedPools
		})
	case TypeRenamed:
		return decodeInto(e, func(p Renamed) { s.Name = p.Name })
	case TypeArchived:
		s.Archived = true
	case TypeCategoryAdded:
		return decodeInto(e, s.applyCategoryAdded)
	case TypeCategoryRemoved:
		return decodeInto(e, s.applyCategoryRemoved)
	case TypeEntrantRegistered:
		return decodeInto(e, s.applyEntrantRegistered)
	case TypeEntrantsImported:
		return decodeInto(e, s.applyEntrantsImported)
	case TypeEntrantUpdated:
		return decodeInto(e, s.applyEntrantUpdated)
	case TypeEntrantRemoved:
		return decodeInto(e, s.applyEntrantRemoved)
	case TypePlayerCheckedIn:
		return decodeInto(e, func(p PlayerCheckedIn) { s.applyPlayerCheckedIn(p, e.At) })
	case TypeEntrantCheckedIn:
		return decodeInto(e, func(p EntrantCheckedIn) { s.applyEntrantCheckedIn(p, e.At) })
	case TypeCheckInCancelled:
		return decodeInto(e, s.applyCheckInCancelled)
	case TypeFormatSet:
		return decodeInto(e, s.applyFormatSet)
	case TypeScheduleSet:
		return decodeInto(e, s.applyScheduleSet)
	case TypeScheduleBlocksSet:
		return decodeInto(e, s.applyScheduleBlocksSet)
	case TypeScheduleOrderSet:
		return decodeInto(e, func(p ScheduleOrderSet) { s.applyScheduleOrderSet(p, e.At) })
	case TypeSeedsSet:
		return decodeInto(e, s.applySeedsSet)
	case TypeEntrantSeeded:
		return decodeInto(e, s.applyEntrantSeeded)
	case TypeEntryDrawn:
		return decodeInto(e, s.applyEntryDrawn)
	case TypeEntrantMoved:
		return decodeInto(e, s.applyEntrantMoved)
	case TypeDrawConfirmed:
		return decodeInto(e, func(p DrawConfirmed) { s.setConfirmed(p.Category, true, e.At) })
	case TypeDrawReopened:
		return decodeInto(e, func(p DrawConfirmed) { s.setConfirmed(p.Category, false, time.Time{}) })
	case TypeCourtsSet:
		return decodeInto(e, func(p CourtsSet) { s.courts = p.Courts })
	case TypeMinRestSet:
		return decodeInto(e, func(p MinRestSet) { s.minRest = &p.Minutes })
	case TypeCourtPrioritySet:
		return decodeInto(e, func(p CourtPrioritySet) { s.priority = p.Priority })
	case TypeMatchCalled:
		return decodeInto(e, func(p MatchCalled) { s.applyMatchCalled(p, e.At, e.Seq) })
	case TypeMatchEnded:
		return decodeInto(e, func(p MatchEnded) { s.applyMatchEnded(p, e.At) })
	case TypeScoreRecorded:
		return decodeInto(e, func(p ScoreRecorded) { s.applyScoreRecorded(p, e.At) })
	case TypeEntrantWithdrawn:
		return decodeInto(e, func(p EntrantWithdrawn) { s.applyEntrantWithdrawn(p, e.At) })
	case TypeScoreCleared:
		return decodeInto(e, s.applyScoreCleared)
	case TypeEntrantReinstated:
		return decodeInto(e, s.applyEntrantReinstated)
	case TypeMatchesSwapped:
		return decodeInto(e, s.applyMatchesSwapped)
	case TypeLotsDrawn:
		return decodeInto(e, s.applyLotsDrawn)
	case TypeStageFilled:
		return decodeInto(e, func(p StageFilled) { s.applyStageFilled(p, e.At) })
	case TypeSlotsSwapped:
		return decodeInto(e, s.applySlotsSwapped)
	case TypeRoundStarted:
		return decodeInto(e, func(p RoundStarted) { s.applyRoundStarted(p, e.At, e.Seq) })
	case TypeRoundEnded:
		return decodeInto(e, func(p RoundEnded) { s.applyRoundEnded(p, e.At) })
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

// Archive closes the tournament.
func (s State) Archive() (Change, error) {
	if s.Archived {
		return Change{}, nil
	}
	return Change{TypeArchived, Archived{}}, nil
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
