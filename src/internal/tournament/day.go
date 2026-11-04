package tournament

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	// TypeDaySet records the date and hours of the tournament.
	TypeDaySet events.Type = "TournamentDaySet"
	// TypeBreakAdded records a pause of every court, at fixed hours.
	TypeBreakAdded events.Type = "BreakAdded"
	// TypeBreakRemoved removes a pause.
	TypeBreakRemoved events.Type = "BreakRemoved"
)

// Formats of the day's date and hours, as stored in events.
const (
	DateLayout  = "2006-01-02"
	ClockLayout = "15:04"
)

// DaySet is the payload of TypeDaySet: the tournament's date ("2026-11-14")
// and hours ("09:00", "19:00"), each "" when not set. Hours are local time.
type DaySet struct {
	Date  string `json:"date,omitempty"`
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// Break is a pause of every court, from From to To ("12:00", "13:30") on the
// tournament's day: no match is planned across it.
type Break struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
}

// BreakAdded is the payload of TypeBreakAdded. The break's ID comes from the
// event seq ("k12").
type BreakAdded struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// BreakRemoved is the payload of TypeBreakRemoved.
type BreakRemoved struct {
	ID string `json:"id"`
}

// Day returns the tournament's date and hours.
func (s State) Day() DaySet { return s.day }

// Breaks returns the pauses, earliest first.
func (s State) Breaks() []Break { return slices.Clone(s.breaks) }

// SetDay sets the tournament's date and hours. Each may be empty; the end
// must come after the start.
func (s State) SetDay(d DaySet) (Change, error) {
	if d.Date != "" {
		if _, err := time.Parse(DateLayout, d.Date); err != nil {
			return Change{}, engine.NewError(engine.CodeInvalidTime)
		}
	}
	start, okStart := clock(d.Start)
	end, okEnd := clock(d.End)
	if !okStart || !okEnd || (d.Start != "" && d.End != "" && end <= start) {
		return Change{}, engine.NewError(engine.CodeInvalidTime)
	}
	if d == s.day {
		return Change{}, nil
	}
	return Change{TypeDaySet, d}, nil
}

// AddBreak adds a pause of every court from one hour to a later one.
func (s State) AddBreak(from, to string) (Change, error) {
	f, okFrom := clock(from)
	t, okTo := clock(to)
	if from == "" || to == "" || !okFrom || !okTo || t <= f {
		return Change{}, engine.NewError(engine.CodeInvalidTime)
	}
	return Change{TypeBreakAdded, BreakAdded{From: from, To: to}}, nil
}

// RemoveBreak removes a pause.
func (s State) RemoveBreak(id string) (Change, error) {
	if !slices.ContainsFunc(s.breaks, func(b Break) bool { return b.ID == id }) {
		return Change{}, engine.NewError(engine.CodeInvalidSetting)
	}
	return Change{TypeBreakRemoved, BreakRemoved{ID: id}}, nil
}

func (s *State) applyBreakAdded(p BreakAdded, seq uint64) {
	s.breaks = append(s.breaks, Break{ID: fmt.Sprintf("k%d", seq), From: p.From, To: p.To})
	slices.SortStableFunc(s.breaks, func(a, b Break) int { return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To)) })
}

func (s *State) applyBreakRemoved(p BreakRemoved) {
	s.breaks = slices.DeleteFunc(s.breaks, func(b Break) bool { return b.ID == p.ID })
}

// clock reads "HH:MM" as minutes from midnight; "" is valid (not set).
func clock(v string) (int, bool) {
	if v == "" {
		return 0, true
	}
	t, err := time.Parse(ClockLayout, v)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// DayDate is the tournament's day at midnight, in now's time zone: its date,
// or now's date when none is set.
func (s State) DayDate(now time.Time) time.Time {
	y, m, d := now.Date()
	if t, err := time.Parse(DateLayout, s.day.Date); err == nil {
		y, m, d = t.Date()
	}
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

// at is an hour of the tournament's day; zero when not set.
func (s State) at(hour string, now time.Time) time.Time {
	mins, ok := clock(hour)
	if hour == "" || !ok {
		return time.Time{}
	}
	return s.DayDate(now).Add(time.Duration(mins) * time.Minute)
}

// DayStart is when the tournament starts: its start hour on its day; zero
// when no start hour is set.
func (s State) DayStart(now time.Time) time.Time { return s.at(s.day.Start, now) }

// DayEnd is when the tournament should be over; zero when no end hour is set.
func (s State) DayEnd(now time.Time) time.Time { return s.at(s.day.End, now) }

// BreakTimes are the breaks on the tournament's day.
func (s State) BreakTimes(now time.Time) []TimedBreak {
	out := []TimedBreak{}
	for _, b := range s.breaks {
		out = append(out, TimedBreak{ID: b.ID, From: s.at(b.From, now), To: s.at(b.To, now)})
	}
	return out
}

// TimedBreak is a break placed on the tournament's day.
type TimedBreak struct {
	ID       string
	From, To time.Time
}

// ForecastStart is when the forecast starts: the tournament's start while
// no match has been called (now once it is past), then now: real times take
// over.
func (s State) ForecastStart(now time.Time) time.Time {
	if slices.ContainsFunc(s.matches, func(m Match) bool { return m.Called() }) {
		return now
	}
	return later(now, s.DayStart(now))
}
