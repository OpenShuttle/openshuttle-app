package tournament

import (
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// CourtState is the state of a court.
type CourtState string

const (
	CourtFree     CourtState = "free"
	CourtPlaying  CourtState = "playing"
	CourtFinished CourtState = "finished" // ended by the desk, score not entered yet
)

// Court is a court and the match it shows: in progress, or finished and
// waiting for its score.
type Court struct {
	Number int
	State  CourtState
	Match  *Match
}

// CourtStates returns every court, from 1. A court shows the latest match
// called on it until that match is decided or another match is called.
func (s State) CourtStates() []Court {
	latest := map[int]Match{}
	for _, m := range s.matches {
		if m.Called() && m.callSeq >= latest[m.Court].callSeq {
			latest[m.Court] = m
		}
	}
	courts := make([]Court, s.Courts())
	for i := range courts {
		c := Court{Number: i + 1, State: CourtFree}
		if m, ok := latest[c.Number]; ok && !s.Decided(m) {
			match := m
			c.Match = &match
			c.State = CourtFinished
			if !m.Ended() {
				c.State = CourtPlaying
			}
		}
		courts[i] = c
	}
	return courts
}

// PendingScores returns the matches ended without a score, oldest first.
func (s State) PendingScores() []Match {
	var out []Match
	for _, m := range s.matches {
		if m.Ended() && !s.Decided(m) {
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(a, b Match) int { return a.EndedAt.Compare(b.EndedAt) })
	return out
}

// NotReady says why a queued match cannot be called yet.
type NotReady string

const (
	ReasonNone    NotReady = ""
	ReasonPlaying NotReady = "playing" // an entrant is on a court
	ReasonResting NotReady = "resting" // an entrant ended a match less than MinRest ago
	ReasonAbsent  NotReady = "absent"  // a player is no longer checked in
)

// QueueItem is a match waiting to be called.
type QueueItem struct {
	Match  Match
	Reason NotReady
	// Court is where a busy entrant plays (ReasonPlaying).
	Court int
	// RestingSince is when the resting entrant ended (ReasonResting).
	RestingSince time.Time
}

// Ready reports whether the match can be called now.
func (q QueueItem) Ready() bool { return q.Reason == ReasonNone }

// Queue returns the matches to call, in the order the courts take them:
// round order within a category, categories merged by court priority.
func (s State) Queue(now time.Time) []QueueItem {
	playingOn := map[engine.EntrantID]int{}
	lastEnd := map[engine.EntrantID]time.Time{}
	for _, m := range s.matches {
		if s.Playing(m) {
			playingOn[m.A], playingOn[m.B] = m.Court, m.Court
		}
		if m.Ended() {
			for _, id := range []engine.EntrantID{m.A, m.B} {
				if m.EndedAt.After(lastEnd[id]) {
					lastEnd[id] = m.EndedAt
				}
			}
		}
	}
	rest := time.Duration(s.MinRest()) * time.Minute

	var out []QueueItem
	for _, m := range s.orderedWaiting() {
		q := QueueItem{Match: m}
		for _, id := range []engine.EntrantID{m.A, m.B} {
			e, _ := s.Entrant(id)
			switch {
			case playingOn[id] > 0:
				q.Reason, q.Court = ReasonPlaying, playingOn[id]
			case !e.Complete() && q.Reason == ReasonNone:
				q.Reason = ReasonAbsent
			case now.Sub(lastEnd[id]) < rest && q.Reason == ReasonNone:
				q.Reason, q.RestingSince = ReasonResting, lastEnd[id]
			}
			if q.Reason == ReasonPlaying {
				break
			}
		}
		out = append(out, q)
	}
	return out
}

// Proposals assigns the next ready match to every free or finished court,
// in court order. Two proposals never share an entrant.
func (s State) Proposals(now time.Time) map[int]Match {
	taken := map[engine.EntrantID]bool{}
	queue := s.Queue(now)
	out := map[int]Match{}
	for _, c := range s.CourtStates() {
		if c.State == CourtPlaying {
			continue
		}
		for _, q := range queue {
			m := q.Match
			if q.Ready() && !taken[m.A] && !taken[m.B] {
				out[c.Number] = m
				taken[m.A], taken[m.B] = true, true
				break
			}
		}
	}
	return out
}

// orderedWaiting returns the matches not called nor decided, merged across
// categories by court priority.
func (s State) orderedWaiting() []Match {
	waiting := map[engine.Category][]Match{}
	total := map[engine.Category]int{}
	done := map[engine.Category]int{}
	for _, m := range s.matches {
		total[m.Category]++
		if m.Called() || s.Decided(m) {
			done[m.Category]++
			continue
		}
		waiting[m.Category] = append(waiting[m.Category], m)
	}

	priority := s.CourtPriority()
	var out []Match
	if priority != PriorityBalanced {
		first := engine.Category(priority)
		out = append(out, waiting[first]...)
		for _, c := range s.Categories {
			if c != first {
				out = append(out, waiting[c]...)
			}
		}
		return out
	}
	// Balanced: the category with the smallest share of matches started goes
	// next. Shares are compared as fractions to stay exact.
	for {
		best := engine.Category("")
		for _, c := range s.Categories {
			if len(waiting[c]) == 0 {
				continue
			}
			if best == "" || done[c]*total[best] < done[best]*total[c] {
				best = c
			}
		}
		if best == "" {
			return out
		}
		out = append(out, waiting[best][0])
		waiting[best] = waiting[best][1:]
		done[best]++
	}
}

// PoolRef names a pool of a category.
type PoolRef struct {
	Category engine.Category
	Pool     int
}

// CompletePools returns the pools whose every match is decided.
func (s State) CompletePools() []PoolRef {
	open := map[PoolRef]bool{}
	var order []PoolRef
	for _, m := range s.matches {
		ref := PoolRef{m.Category, m.Pool}
		if _, seen := open[ref]; !seen {
			order = append(order, ref)
			open[ref] = false
		}
		if !s.Decided(m) {
			open[ref] = true
		}
	}
	var out []PoolRef
	for _, ref := range order {
		if !open[ref] {
			out = append(out, ref)
		}
	}
	return out
}

// PoolStandings ranks a pool with the stage's tiebreaks.
func (s State) PoolStandings(c engine.Category, pool int) engine.PoolStandings {
	d, ok := s.Draw(c)
	if !ok || pool < 0 || pool >= len(d.Pools) {
		return engine.PoolStandings{}
	}
	p := engine.Pool{
		Entrants:  d.Pools[pool],
		Withdrawn: map[engine.EntrantID]bool{},
		Tiebreaks: s.PoolStage(c).Tiebreaks,
	}
	for _, id := range d.Pools[pool] {
		p.Withdrawn[id] = s.withdrawn[id]
	}
	for _, m := range s.matches {
		if m.Category == c && m.Pool == pool {
			p.Matches = append(p.Matches, engine.PoolMatch{A: m.A, B: m.B, Score: m.Score})
		}
	}
	return engine.RankPool(p)
}
