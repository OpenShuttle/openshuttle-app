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
	for _, m := range s.orderedWaiting(now) {
		q := QueueItem{Match: m}
		for _, id := range []engine.EntrantID{m.A, m.B} {
			e, _ := s.Entrant(id)
			switch {
			case playingOn[id] > 0:
				q.Reason, q.Court = ReasonPlaying, playingOn[id]
			case !e.Complete() && q.Reason == ReasonNone:
				q.Reason = ReasonAbsent
			case now.Sub(lastEnd[id]) < rest && q.Reason == ReasonNone && !s.Timed(m):
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

// orderedWaiting returns the matches not called nor decided, in schedule
// order. Matches the schedule leaves out (none in practice) come last.
func (s State) orderedWaiting(now time.Time) []Match {
	return s.waitingIn(s.Schedule(now))
}

// waitingIn orders the waiting matches as a schedule plays them.
func (s State) waitingIn(sched Schedule) []Match {
	var out []Match
	seen := map[MatchID]bool{}
	for _, slot := range sched.Slots {
		if m, ok := s.Match(slot.Match); ok && slot.Match != "" && !seen[m.ID] && s.Playable(m) {
			out = append(out, m)
			seen[m.ID] = true
		}
	}
	for _, m := range s.matches {
		if !seen[m.ID] && !m.ByeA && !m.ByeB && !m.Called() && !s.Decided(m) && s.Playable(m) {
			out = append(out, m)
		}
	}
	return out
}

// PoolRef names a pool of a stage.
type PoolRef struct {
	Category engine.Category
	Stage    string
	Pool     int
}

// CompletePools returns the pools whose every match is decided.
func (s State) CompletePools() []PoolRef {
	open := map[PoolRef]bool{}
	var order []PoolRef
	for _, m := range s.matches {
		if m.IsBracket() {
			continue
		}
		ref := PoolRef{m.Category, m.Stage, m.Pool}
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

// PoolStandings ranks a pool of a stage with the stage's tiebreaks.
func (s State) PoolStandings(c engine.Category, stage string, pool int) engine.PoolStandings {
	st := s.stage(c, stage)
	if st == nil || st.Kind != StagePools || pool < 0 || pool >= len(st.Groups) {
		return engine.PoolStandings{}
	}
	return engine.RankPool(s.poolInput(c, stage, pool))
}

// poolInput gathers what ranks a pool: entrants, matches, forfeits, lots.
func (s State) poolInput(c engine.Category, stage string, pool int) engine.Pool {
	st := s.stage(c, stage)
	si := s.stageIndex(c, stage)
	p := engine.Pool{
		Entrants:  st.Groups[pool],
		Withdrawn: map[engine.EntrantID]bool{},
		Tiebreaks: st.Tiebreaks,
		DrawLots:  s.lots[PoolRef{c, stage, pool}],
	}
	for _, id := range st.Groups[pool] {
		p.Withdrawn[id] = s.withdrawnBy(id, c, si)
	}
	for _, m := range s.matches {
		if m.Category == c && m.Stage == stage && m.Pool == pool {
			p.Matches = append(p.Matches, engine.PoolMatch{A: m.A, B: m.B, Score: m.Score})
		}
	}
	return p
}
