package main

import (
	"context"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// StandingView is a row of a pool ranking.
type StandingView struct {
	Rank       int      `json:"rank"`
	Entrant    SideView `json:"entrant"`
	Played     int      `json:"played"`
	Wins       int      `json:"wins"`
	Losses     int      `json:"losses"`
	SetsWon    int      `json:"setsWon"`
	SetsLost   int      `json:"setsLost"`
	PointsWon  int      `json:"pointsWon"`
	PointsLost int      `json:"pointsLost"`
	Withdrawn  bool     `json:"withdrawn"`
	DrawNeeded bool     `json:"drawNeeded"`
}

// PoolPhase is one pool of the phases screen.
type PoolPhase struct {
	Index      int            `json:"index"`
	Name       string         `json:"name"`
	Complete   bool           `json:"complete"`
	DrawNeeded bool           `json:"drawNeeded"`
	Standings  []StandingView `json:"standings"`
	Matches    []MatchView    `json:"matches"`
}

// PhaseCategory is one category of the phases screen.
type PhaseCategory struct {
	CategoryRef
	Stage tournament.PoolStage `json:"stage"`
	// Started is false until the draw is confirmed.
	Started  bool           `json:"started"`
	Pools    []PoolPhase    `json:"pools"`
	Brackets []BracketPhase `json:"brackets"`
}

// PhasesView is the phases screen.
type PhasesView struct {
	Categories []PhaseCategory `json:"categories"`
}

// HistoryEntry is a line of the history panel.
type HistoryEntry struct {
	ActionView
	// Undone is set when a later undo cancelled this entry.
	Undone bool `json:"undone"`
}

// HistoryView lists every action, newest first.
type HistoryView struct {
	Entries []HistoryEntry `json:"entries"`
}

// ClearScore removes a recorded score.
func (s *TournamentService) ClearScore(ctx context.Context, id tournament.MatchID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ClearScore(id) })
}

// ReinstateEntrant cancels a forfeit.
func (s *TournamentService) ReinstateEntrant(ctx context.Context, id engine.EntrantID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ReinstateEntrant(id) })
}

// SwapMatches swaps two waiting matches in the call order.
func (s *TournamentService) SwapMatches(ctx context.Context, a, b tournament.MatchID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SwapMatches(a, b) })
}

// DrawLots breaks the remaining ties of a complete pool by lot.
func (s *TournamentService) DrawLots(ctx context.Context, c engine.Category, pool int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.DrawLots(c, pool, s.rng()) })
}

// GetPhasesView returns the phases screen.
func (s *TournamentService) GetPhasesView() PhasesView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	mv := s.matchViewer(st)
	side := sideViewer(st)
	complete := map[tournament.PoolRef]bool{}
	for _, ref := range st.CompletePools() {
		complete[ref] = true
	}

	v := PhasesView{Categories: []PhaseCategory{}}
	for _, ref := range categoryRefs(st) {
		pc := PhaseCategory{CategoryRef: ref, Stage: st.PoolStage(ref.Category), Pools: []PoolPhase{}}
		d, ok := st.Draw(ref.Category)
		pc.Started = ok && d.Confirmed
		if pc.Started {
			for i := range d.Pools {
				pool := PoolPhase{
					Index: i, Name: engine.PoolName(i), Complete: complete[tournament.PoolRef{Category: ref.Category, Pool: i}],
					Standings: []StandingView{}, Matches: []MatchView{},
				}
				for _, row := range st.PoolStandings(ref.Category, i).Rows {
					pool.Standings = append(pool.Standings, StandingView{
						Rank: row.Rank, Entrant: side(row.Entrant), Played: row.Played, Wins: row.Wins, Losses: row.Losses,
						SetsWon: row.SetsWon, SetsLost: row.SetsLost, PointsWon: row.PointsWon, PointsLost: row.PointsLost,
						Withdrawn: row.Withdrawn, DrawNeeded: row.DrawNeeded,
					})
					pool.DrawNeeded = pool.DrawNeeded || row.DrawNeeded
				}
				for _, m := range st.Matches() {
					if m.Category == ref.Category && m.Bracket == "" && m.Pool == i {
						pool.Matches = append(pool.Matches, mv(m))
					}
				}
				pc.Pools = append(pc.Pools, pool)
			}
		}
		pc.Brackets = s.bracketPhases(st, ref.Category)
		v.Categories = append(v.Categories, pc)
	}
	return v
}

// GetHistoryView returns every action, newest first.
func (s *TournamentService) GetHistoryView() HistoryView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := HistoryView{Entries: []HistoryEntry{}}
	for i := len(s.log) - 1; i >= 0; i-- {
		e := s.log[i]
		v.Entries = append(v.Entries, HistoryEntry{
			ActionView: *s.describe(e),
			Undone:     e.Type != events.TypeUndone && !events.IsActive(s.log, e.Seq),
		})
	}
	return v
}
