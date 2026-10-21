package tournament

import (
	"fmt"
	"math/bits"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament/blocks"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament/forecast"
)

// ScheduleSlot is a match of the schedule: on a court, from Start to End.
type ScheduleSlot struct {
	// Key is the block the match belongs to: its round of a pool or of a
	// bracket.
	Key      string
	Category engine.Category
	Stage    string
	Pool     int // -1 in a bracket
	// Level is the round of the pool, or the round of the bracket (0 the
	// first).
	Level int
	// RoundSize is, in a bracket, how many entrants the round starts with
	// (8 for quarter-finals, 2 for a final).
	RoundSize int
	// PlaceFrom is, in a bracket, the first place its part of the bracket
	// plays for: 1 for the main draw, 5 for places 5 to 8 (0 in pools).
	PlaceFrom int
	// Match is "" in a stage not filled yet: who plays is not known.
	Match MatchID
	A, B  engine.EntrantID
	Court int
	Start time.Time
	End   time.Time
	// Unrested: no rested match fitted the rules here. The desk is warned: it
	// calls the match anyway or changes the order.
	Unrested bool
}

// Schedule is the forecast of the matches left: every waiting match, and
// the matches of stages not filled yet, on a court with estimated times.
type Schedule struct {
	Slots []ScheduleSlot
	// End is when the last match is expected to end.
	End time.Time
	// Reserved is the category of each court in reserved mode.
	Reserved map[int]engine.Category
}

// Schedule forecasts the rest of the tournament from now (forecast.Run):
// courts take the matches in schedule order (ScheduleOrder), then the
// stand-ins of stages not filled yet, as they free up. A match on court
// running late ends any minute, never "already".
func (s State) Schedule(now time.Time) Schedule {
	items := s.orderedItems()
	in := forecast.Input{
		Now: now, Courts: s.Courts(), Rest: time.Duration(s.MinRest()) * time.Minute,
		Items: items, Reserved: s.reservedCourts(items),
	}
	for _, m := range s.matches {
		switch {
		case s.Playing(m):
			end := m.CalledAt.Add(time.Duration(s.WinCondition(m).EstimatedMinutes()) * time.Minute)
			if !end.After(now) {
				end = now.Add(time.Minute)
			}
			in.OnCourt = append(in.OnCourt, forecast.OnCourt{
				Court: m.Court, Category: m.Category, Stage: m.Stage, Level: s.bracketLevel(m), A: m.A, B: m.B, End: end,
			})
		case m.Ended():
			in.Played = append(in.Played, forecast.Played{A: m.A, B: m.B, End: m.EndedAt})
		}
	}
	res := forecast.Run(in)
	out := Schedule{End: res.End, Reserved: in.Reserved}
	for _, sl := range res.Slots {
		out.Slots = append(out.Slots, ScheduleSlot{
			Key: sl.Key, Category: sl.Category, Stage: sl.Stage, Pool: sl.Pool, Level: sl.Level,
			RoundSize: sl.RoundSize, PlaceFrom: sl.PlaceFrom, Match: MatchID(sl.Match), A: sl.A, B: sl.B,
			Court: sl.Court, Start: sl.Start, End: sl.End, Unrested: sl.Unrested,
		})
	}
	return out
}

// orderedItems lists the matches to place: the waiting matches in schedule
// order, then the stand-ins of stages not filled yet, in block order.
func (s State) orderedItems() []forecast.Item {
	all := s.scheduleItems()
	byMatch := map[MatchID]forecast.Item{}
	var out, stand []forecast.Item
	for _, it := range all {
		if it.Match != "" {
			byMatch[MatchID(it.Match)] = it
		} else {
			stand = append(stand, it)
		}
	}
	for _, id := range s.ScheduleOrder() {
		if it, ok := byMatch[id]; ok {
			out = append(out, it)
		}
	}
	rank := map[string]int{}
	for i, b := range s.blockOrder() {
		rank[b.Key] = i
	}
	slices.SortStableFunc(stand, func(a, b forecast.Item) int { return rank[a.Key] - rank[b.Key] })
	return append(out, stand...)
}

// scheduleItems lists the matches left: waiting matches of filled stages,
// and stand-in matches for stages not filled yet, played after the stages
// feeding them.
func (s State) scheduleItems() []forecast.Item {
	var items []forecast.Item
	for _, c := range s.Categories {
		stages := s.Stages(c)
		preview := PreviewFormat(s.StageConfigs(c), s.expectedEntrants(c), s.filledSizes(c))
		for i, st := range stages {
			base := forecast.Item{Category: c, Stage: st.ID, Minutes: st.Win.EstimatedMinutes()}
			if st.Confirmed {
				items = append(items, s.waitingItems(st, base)...)
				continue
			}
			if i > 0 {
				base.After = s.Sources(c, st.ID)
				if len(base.After) == 0 {
					continue // nothing feeds it: nobody will play it
				}
			}
			items = append(items, standIns(st, preview.Stages[i], base)...)
		}
	}
	return items
}

// expectedEntrants is how many entrants a category plays with: those present,
// or all of them before the check-in.
func (s State) expectedEntrants(c engine.Category) int {
	if n := len(s.Present(c)); n > 0 {
		return n
	}
	n := 0
	for _, e := range s.Entrants {
		if e.Category == c {
			n++
		}
	}
	return n
}

// filledSizes gives the group sizes of the category's filled stages.
func (s State) filledSizes(c engine.Category) map[string][]int {
	filled := map[string][]int{}
	for _, st := range s.Stages(c) {
		if st.Confirmed {
			filled[st.ID] = groupSizes(st.Groups)
		}
	}
	return filled
}

// waitingItems are the waiting matches of a filled stage.
func (s State) waitingItems(st Stage, base forecast.Item) []forecast.Item {
	var items []forecast.Item
	for _, m := range s.matches {
		if m.Category != base.Category || m.Stage != st.ID || m.ByeA || m.ByeB || m.Called() || s.Decided(m) {
			continue
		}
		it := base
		it.Pool, it.Match, it.A, it.B, it.RoundSize = m.Pool, string(m.ID), m.A, m.B, m.Size
		if m.IsBracket() {
			it.Level, it.PlaceFrom = s.bracketLevel(m), m.Start
			it.Key = blocks.BracketKey(base.Category, st.ID, it.Level, it.PlaceFrom)
		} else {
			it.Level = m.Round
			it.Key = blocks.PoolKey(base.Category, st.ID, m.Pool, m.Round)
		}
		items = append(items, it)
	}
	return items
}

// standIns are the matches a stage not filled yet will hold, from its
// preview: every round-robin pairing of each pool (placeholder entrants, so
// a pool's rounds keep apart), or every bracket match but first-round byes.
func standIns(st Stage, p StagePreview, base forecast.Item) []forecast.Item {
	c := base.Category
	var items []forecast.Item
	if st.Kind == StagePools {
		for pi, size := range p.Sizes {
			for r, round := range engine.RoundRobinSchedule(size) {
				for _, pair := range round {
					it := base
					it.Pool, it.Level = pi, r
					it.Key = blocks.PoolKey(c, st.ID, pi, r)
					it.A = engine.EntrantID(fmt.Sprintf("~%s/%s/%d/%d", c, st.ID, pi, pair[0]))
					it.B = engine.EntrantID(fmt.Sprintf("~%s/%s/%d/%d", c, st.ID, pi, pair[1]))
					items = append(items, it)
				}
			}
		}
		return items
	}
	if p.Entrants < 2 {
		return nil
	}
	size := engine.NextPowerOfTwo(p.Entrants)
	byes := size - p.Entrants
	for _, node := range subBrackets(size, st.FullPlacement) {
		level := bits.Len(uint(size/node.Size)) - 1
		for range node.Size / 2 {
			if level == 0 && byes > 0 {
				byes--
				continue
			}
			it := base
			it.Pool, it.Level, it.RoundSize, it.PlaceFrom = -1, level, node.Size, node.Start
			it.Key = blocks.BracketKey(c, st.ID, level, node.Start)
			items = append(items, it)
		}
	}
	return items
}

// reservedCourts gives each court its category in reserved mode
// (forecast.ReserveCourts), from the matches left.
func (s State) reservedCourts(items []forecast.Item) map[int]engine.Category {
	settings := s.ScheduleSettings()
	if settings.Mode != ScheduleReserved {
		return map[int]engine.Category{}
	}
	left := map[engine.Category]int{}
	for _, it := range items {
		left[it.Category]++
	}
	return forecast.ReserveCourts(s.Courts(), settings.Courts, s.Categories, left)
}

// Proposals gives each free or finished court the match the schedule starts
// there now. Timed matches are called by rounds, never one court at a time.
func (s State) Proposals(now time.Time) map[int]ScheduleSlot {
	out := map[int]ScheduleSlot{}
	playing := map[int]bool{}
	for _, c := range s.CourtStates() {
		playing[c.Number] = c.State == CourtPlaying
	}
	for _, slot := range s.Schedule(now).Slots {
		if !slot.Start.Equal(now) || playing[slot.Court] || slot.Match == "" {
			continue
		}
		if m, ok := s.Match(slot.Match); ok && !s.Timed(m) && s.Playable(m) && !s.onCourt(m.A) && !s.onCourt(m.B) {
			out[slot.Court] = slot
		}
	}
	return out
}

// onCourt reports whether an entrant is playing a match.
func (s State) onCourt(id engine.EntrantID) bool {
	return id != "" && slices.ContainsFunc(s.matches, func(m Match) bool { return m.Has(id) && s.Playing(m) })
}

// MatchesLeft counts, per entrant, the matches of a stage it has not started.
func (s State) MatchesLeft(c engine.Category, stage string) map[engine.EntrantID]int {
	out := map[engine.EntrantID]int{}
	for _, m := range s.matches {
		if m.Category != c || m.Stage != stage || m.ByeA || m.ByeB || m.Called() || s.Decided(m) {
			continue
		}
		for _, id := range []engine.EntrantID{m.A, m.B} {
			if id != "" {
				out[id]++
			}
		}
	}
	return out
}

func groupSizes(groups [][]engine.EntrantID) []int {
	var sizes []int
	for _, g := range groups {
		n := 0
		for _, id := range g {
			if id != "" {
				n++
			}
		}
		sizes = append(sizes, n)
	}
	return sizes
}
