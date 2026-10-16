package main

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// SideView is one side of a match.
type SideView struct {
	Entrant   engine.EntrantID `json:"entrant"`
	Bib       int              `json:"bib"`
	Names     []string         `json:"names"` // "First Last" per player
	Clubs     []string         `json:"clubs"`
	Withdrawn bool             `json:"withdrawn"`
}

// MatchView is a match as every desk screen shows it.
type MatchView struct {
	ID tournament.MatchID `json:"id"`
	CategoryRef
	Pool     string              `json:"pool"`
	Round    int                 `json:"round"` // 1-based
	A        SideView            `json:"a"`
	B        SideView            `json:"b"`
	Court    int                 `json:"court"`
	CalledAt *time.Time          `json:"calledAt"`
	EndedAt  *time.Time          `json:"endedAt"`
	Score    *engine.Score       `json:"score"`
	Walkover bool                `json:"walkover"`
	Win      engine.WinCondition `json:"win"`
}

// CourtView is a court card: its match and the next match proposed.
type CourtView struct {
	Number   int                   `json:"number"`
	State    tournament.CourtState `json:"state"`
	Match    *MatchView            `json:"match"`
	Proposal *MatchView            `json:"proposal"`
}

// QueueView is a match waiting to be called, with why it is not ready.
type QueueView struct {
	Match        MatchView           `json:"match"`
	Reason       tournament.NotReady `json:"reason"`
	Court        int                 `json:"court"`
	RestingSince *time.Time          `json:"restingSince"`
}

// PoolRefView names a pool.
type PoolRefView struct {
	CategoryRef
	Pool string `json:"pool"`
}

// ControlDeskView is the control desk: courts, pending scores, queue, alerts.
type ControlDeskView struct {
	Now        time.Time     `json:"now"`
	Courts     []CourtView   `json:"courts"`
	Pending    []MatchView   `json:"pending"`
	Queue      []QueueView   `json:"queue"`
	Complete   []PoolRefView `json:"complete"`
	Priority   string        `json:"priority"`
	MinRest    int           `json:"minRest"`
	Categories []CategoryRef `json:"categories"`
	// Started is false until a draw is confirmed: there is nothing to call yet.
	Started bool `json:"started"`
}

// DisplayCourt is a court on the display screen.
type DisplayCourt struct {
	Number int        `json:"number"`
	Match  *MatchView `json:"match"` // nil when free
}

// DisplayView is the display screen: latest calls in big, then every court.
type DisplayView struct {
	Calls  []MatchView    `json:"calls"`
	Courts []DisplayCourt `json:"courts"`
}

// CallMatch calls a match on a court.
func (s *TournamentService) CallMatch(ctx context.Context, id tournament.MatchID, court int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.CallMatch(id, court) })
}

// EndMatch ends a match in progress; its score can be entered later.
func (s *TournamentService) EndMatch(ctx context.Context, id tournament.MatchID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.EndMatch(id) })
}

// RecordScore records or corrects a score.
func (s *TournamentService) RecordScore(ctx context.Context, id tournament.MatchID, score engine.Score) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.RecordScore(id, score) })
}

// WithdrawEntrant declares a forfeit: all the entrant's pool matches become walkovers.
func (s *TournamentService) WithdrawEntrant(ctx context.Context, id engine.EntrantID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.WithdrawEntrant(id) })
}

// SetCourts sets the number of courts.
func (s *TournamentService) SetCourts(ctx context.Context, n int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetCourts(n) })
}

// SetMinRest sets the minimum rest between two matches, in minutes.
func (s *TournamentService) SetMinRest(ctx context.Context, minutes int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetMinRest(minutes) })
}

// SetCourtPriority sets "balanced" or the category played first.
func (s *TournamentService) SetCourtPriority(ctx context.Context, priority string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetCourtPriority(priority) })
}

// GetControlDeskView returns the control desk.
func (s *TournamentService) GetControlDeskView() ControlDeskView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	now := s.now()
	mv := s.matchViewer(st)

	v := ControlDeskView{
		Now: now, Courts: []CourtView{}, Pending: []MatchView{}, Queue: []QueueView{}, Complete: []PoolRefView{},
		Priority: st.CourtPriority(), MinRest: st.MinRest(), Categories: categoryRefs(st),
		Started: len(st.Matches()) > 0,
	}
	proposals := st.Proposals(now)
	for _, c := range st.CourtStates() {
		cv := CourtView{Number: c.Number, State: c.State}
		if c.Match != nil {
			m := mv(*c.Match)
			cv.Match = &m
		}
		if p, ok := proposals[c.Number]; ok {
			m := mv(p)
			cv.Proposal = &m
		}
		v.Courts = append(v.Courts, cv)
	}
	for _, m := range st.PendingScores() {
		v.Pending = append(v.Pending, mv(m))
	}
	for _, q := range st.Queue(now) {
		qv := QueueView{Match: mv(q.Match), Reason: q.Reason, Court: q.Court}
		if !q.RestingSince.IsZero() {
			t := q.RestingSince
			qv.RestingSince = &t
		}
		v.Queue = append(v.Queue, qv)
	}
	slots := map[engine.Category]int{}
	for _, ref := range v.Categories {
		slots[ref.Category] = ref.Slot
	}
	for _, p := range st.CompletePools() {
		v.Complete = append(v.Complete, PoolRefView{CategoryRef{p.Category, slots[p.Category]}, engine.PoolName(p.Pool)})
	}
	return v
}

// GetDisplayView returns the display screen.
func (s *TournamentService) GetDisplayView() DisplayView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	mv := s.matchViewer(st)
	v := DisplayView{Calls: []MatchView{}, Courts: []DisplayCourt{}}
	var playing []tournament.Match
	for _, c := range st.CourtStates() {
		dc := DisplayCourt{Number: c.Number}
		if c.State == tournament.CourtPlaying {
			m := mv(*c.Match)
			dc.Match = &m
			playing = append(playing, *c.Match)
		}
		v.Courts = append(v.Courts, dc)
	}
	// Latest calls first: the hall reads the top of the screen.
	slices.SortStableFunc(playing, func(a, b tournament.Match) int { return b.CalledAt.Compare(a.CalledAt) })
	for _, m := range playing[:min(len(playing), 3)] {
		v.Calls = append(v.Calls, mv(m))
	}
	return v
}

// sideViewer returns a function describing an entrant as a match side.
func sideViewer(st tournament.State) func(engine.EntrantID) SideView {
	return func(id engine.EntrantID) SideView {
		e, _ := st.Entrant(id)
		sv := SideView{Entrant: id, Bib: e.Bib, Names: []string{}, Clubs: e.Clubs(), Withdrawn: st.Withdrawn(id)}
		for _, p := range e.Players {
			sv.Names = append(sv.Names, p.FirstName+" "+p.LastName)
		}
		if sv.Clubs == nil {
			sv.Clubs = []string{}
		}
		return sv
	}
}

// matchViewer returns a function turning matches into views.
func (s *TournamentService) matchViewer(st tournament.State) func(tournament.Match) MatchView {
	slots := map[engine.Category]int{}
	for _, ref := range categoryRefs(st) {
		slots[ref.Category] = ref.Slot
	}
	side := sideViewer(st)
	return func(m tournament.Match) MatchView {
		v := MatchView{
			ID: m.ID, CategoryRef: CategoryRef{m.Category, slots[m.Category]},
			Pool: engine.PoolName(m.Pool), Round: m.Round + 1,
			A: side(m.A), B: side(m.B), Court: m.Court, Score: m.Score,
			Walkover: st.Walkover(m), Win: st.WinCondition(m),
		}
		if m.Called() {
			t := m.CalledAt
			v.CalledAt = &t
		}
		if m.Ended() {
			t := m.EndedAt
			v.EndedAt = &t
		}
		return v
	}
}

// matchLabel is "#3 Zola / Ecuyer – #7 Petit / Martin" for history entries.
func matchLabel(st tournament.State, id tournament.MatchID) (a, b string) {
	m, ok := st.Match(id)
	if !ok {
		return string(id), ""
	}
	label := func(eid engine.EntrantID) string {
		e, _ := st.Entrant(eid)
		names := make([]string, len(e.Players))
		for i, p := range e.Players {
			names[i] = p.LastName
		}
		prefix := ""
		if e.Bib > 0 {
			prefix = "#" + itoa(e.Bib) + " "
		}
		return prefix + strings.Join(names, " / ")
	}
	return label(m.A), label(m.B)
}

// scoreLabel is "21-10 21-12".
func scoreLabel(sc engine.Score) string {
	parts := make([]string, len(sc.Sets))
	for i, set := range sc.Sets {
		parts[i] = itoa(set.A) + "-" + itoa(set.B)
	}
	return strings.Join(parts, " ")
}
