package main

import (
	"context"
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
	// Stage of the match: its name ("" shows the kind's label), its kind,
	// and whether it is the entry stage (its name is then left out).
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	// Bracket matches: Pool is empty, the round plays for places
	// PlaceFrom..PlaceTo with RoundSize entrants (2 for a final).
	PlaceFrom int  `json:"placeFrom"`
	PlaceTo   int  `json:"placeTo"`
	RoundSize int  `json:"roundSize"`
	ByeA      bool `json:"byeA"`
	ByeB      bool `json:"byeB"`
}

// CourtView is a court card: its match and the next match proposed.
type CourtView struct {
	Number   int                   `json:"number"`
	State    tournament.CourtState `json:"state"`
	Match    *MatchView            `json:"match"`
	Proposal *MatchView            `json:"proposal"`
	// ProposalUnrested warns that the proposal's players have not had their
	// minimum rest: no rested match fitted the schedule rules. The desk calls
	// it anyway or changes the schedule.
	ProposalUnrested bool `json:"proposalUnrested"`
}

// QueueView is a match waiting to be called, with why it is not ready.
type QueueView struct {
	Match        MatchView           `json:"match"`
	Reason       tournament.NotReady `json:"reason"`
	Court        int                 `json:"court"`
	RestingSince *time.Time          `json:"restingSince"`
}

// PoolRefView names a pool of a stage.
type PoolRefView struct {
	CategoryRef
	Stage      string `json:"stage"`
	StageName  string `json:"stageName"`
	EntryStage bool   `json:"entryStage"`
	Pool       string `json:"pool"`
}

// StageRefView names a stage.
type StageRefView struct {
	CategoryRef
	Stage string               `json:"stage"`
	Name  string               `json:"name"`
	Kind  tournament.StageKind `json:"kind"`
}

// ControlDeskView is the control desk: courts, pending scores, queue, alerts.
type ControlDeskView struct {
	Now      time.Time     `json:"now"`
	Courts   []CourtView   `json:"courts"`
	Pending  []MatchView   `json:"pending"`
	Queue    []QueueView   `json:"queue"`
	Complete []PoolRefView `json:"complete"`
	// StagesToFill lists the later stages ready to be filled.
	StagesToFill []StageRefView `json:"stagesToFill"`
	MinRest      int            `json:"minRest"`
	Categories   []CategoryRef  `json:"categories"`
	// Started is false until a draw is confirmed: there is nothing to call yet.
	Started bool       `json:"started"`
	Rounds  RoundsView `json:"rounds"`
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

// GetControlDeskView returns the control desk.
func (s *TournamentService) GetControlDeskView() ControlDeskView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	now := s.now()
	mv := s.matchViewer(st)

	v := ControlDeskView{
		Now: now, Courts: []CourtView{}, Pending: []MatchView{}, Queue: []QueueView{}, Complete: []PoolRefView{},
		MinRest: st.MinRest(), Categories: categoryRefs(st),
		Started: len(st.Matches()) > 0,
	}
	v.Rounds = s.roundsView(st)
	proposals := st.Proposals(now)
	for _, c := range st.CourtStates() {
		cv := CourtView{Number: c.Number, State: c.State}
		if c.Match != nil {
			m := mv(*c.Match)
			cv.Match = &m
		}
		if p, ok := proposals[c.Number]; ok {
			if pm, ok := st.Match(p.Match); ok {
				m := mv(pm)
				cv.Proposal, cv.ProposalUnrested = &m, p.Unrested
			}
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
	v.StagesToFill = []StageRefView{}
	for _, ref := range v.Categories {
		for i, stage := range st.Stages(ref.Category) {
			if i > 0 && !stage.Filled() && st.CanFill(ref.Category, stage.ID) {
				v.StagesToFill = append(v.StagesToFill, StageRefView{ref, stage.ID, stage.Name, stage.Kind})
			}
		}
	}
	for _, p := range st.CompletePools() {
		// Pools feeding a later stage lead to the "fill" alert instead.
		stage, _ := st.Stage(p.Category, p.Stage)
		if len(stage.Routing) == 0 {
			v.Complete = append(v.Complete, PoolRefView{
				CategoryRef: CategoryRef{p.Category, slots[p.Category]}, Stage: p.Stage, StageName: stage.Name,
				EntryStage: st.Stages(p.Category)[0].ID == p.Stage, Pool: engine.PoolName(p.Pool),
			})
		}
	}
	return v
}

// sideViewer returns a function describing an entrant as a match side.
func sideViewer(st tournament.State) func(engine.EntrantID) SideView {
	return func(id engine.EntrantID) SideView {
		e, _ := st.Entrant(id)
		sv := SideView{Entrant: id, Bib: e.Bib, Names: []string{}, Clubs: e.Clubs(), Withdrawn: st.Withdrawn(id)}
		for _, p := range e.Players {
			sv.Names = append(sv.Names, p.Name)
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
		stage, _ := st.Stage(m.Category, m.Stage)
		v.Stage, v.StageName, v.StageKind = m.Stage, stage.Name, stage.Kind
		v.EntryStage = st.Stages(m.Category)[0].ID == m.Stage
		if m.IsBracket() {
			v.Pool, v.Round = "", 0
			v.PlaceFrom, v.PlaceTo, v.RoundSize = m.Start, m.Start+m.Size-1, m.Size
			v.ByeA, v.ByeB = m.ByeA, m.ByeB
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

// matchLabel is "#3 Léa Zola / Tom Ecuyer – #7 …" for history entries.
func matchLabel(st tournament.State, id tournament.MatchID) (a, b string) {
	m, ok := st.Match(id)
	if !ok {
		return string(id), ""
	}
	label := func(eid engine.EntrantID) string {
		e, _ := st.Entrant(eid)
		names := make([]string, len(e.Players))
		for i, p := range e.Players {
			names[i] = p.Name
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
