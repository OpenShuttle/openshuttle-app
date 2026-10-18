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

// RoundView is the first round of a sub-bracket: the entrants playing for
// places From..To.
type RoundView struct {
	From    int         `json:"from"`
	To      int         `json:"to"`
	Size    int         `json:"size"`
	Matches []MatchView `json:"matches"`
}

// PlaceView is a final position of a bracket.
type PlaceView struct {
	From    int      `json:"from"`
	To      int      `json:"to"`
	Entrant SideView `json:"entrant"`
}

// StagePhase is a stage on the phases screen: its pools, or its bracket.
type StagePhase struct {
	tournament.StageConfig
	Phase int  `json:"phase"`
	Entry bool `json:"entry"`
	// Confirmed: the stage has matches. Started: a match was called or scored.
	Confirmed bool `json:"confirmed"`
	Started   bool `json:"started"`
	Complete  bool `json:"complete"`
	// Routed is the number of entrants routed here so far (later stages).
	Routed int `json:"routed"`
	// CanFill is set for a later stage whose feeding stages are over.
	CanFill bool `json:"canFill"`
	// Pools stages.
	Pools []PoolPhase `json:"pools"`
	// Bracket stages: first-round slots (byes have an empty entrant), rounds
	// and the places known so far.
	Slots  []SideView  `json:"slots"`
	Rounds []RoundView `json:"rounds"`
	Places []PlaceView `json:"places"`
}

// PhaseCategory is one category of the phases screen.
type PhaseCategory struct {
	CategoryRef
	// Started is false until the draw is confirmed.
	Started bool         `json:"started"`
	Stages  []StagePhase `json:"stages"`
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
func (s *TournamentService) DrawLots(ctx context.Context, c engine.Category, stage string, pool int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.DrawLots(c, stage, pool, s.rng()) })
}

// GetPhasesView returns the phases screen.
func (s *TournamentService) GetPhasesView() PhasesView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	v := PhasesView{Categories: []PhaseCategory{}}
	for _, ref := range categoryRefs(st) {
		pc := PhaseCategory{CategoryRef: ref, Stages: []StagePhase{}}
		d, ok := st.Draw(ref.Category)
		pc.Started = ok && d.Confirmed
		for i, stage := range st.Stages(ref.Category) {
			pc.Stages = append(pc.Stages, s.stagePhase(st, ref.Category, stage, i == 0))
		}
		v.Categories = append(v.Categories, pc)
	}
	return v
}

func (s *TournamentService) stagePhase(st tournament.State, c engine.Category, stage tournament.Stage, entry bool) StagePhase {
	mv := s.matchViewer(st)
	side := sideViewer(st)
	sp := StagePhase{
		StageConfig: stage.StageConfig, Phase: st.Phase(c, stage.ID), Entry: entry,
		Confirmed: stage.Confirmed, Started: stageStarted(st, c, stage.ID), Complete: st.StageComplete(c, stage.ID),
		Pools: []PoolPhase{}, Slots: []SideView{}, Rounds: []RoundView{}, Places: []PlaceView{},
	}
	if sp.Routing == nil {
		sp.Routing = []tournament.RouteRule{}
	}
	if sp.Tiebreaks == nil {
		sp.Tiebreaks = []engine.Tiebreak{}
	}
	if !entry {
		sp.Routed = len(st.Routed(c, stage.ID))
		sp.CanFill = !sp.Started && st.CanFill(c, stage.ID)
	}
	if !stage.Confirmed {
		return sp
	}
	complete := map[tournament.PoolRef]bool{}
	for _, ref := range st.CompletePools() {
		complete[ref] = true
	}
	matches := st.Matches()
	if stage.Kind == tournament.StagePools {
		for i := range stage.Groups {
			pool := PoolPhase{
				Index: i, Name: engine.PoolName(i), Complete: complete[tournament.PoolRef{Category: c, Stage: stage.ID, Pool: i}],
				Standings: []StandingView{}, Matches: []MatchView{},
			}
			for _, row := range st.PoolStandings(c, stage.ID, i).Rows {
				pool.Standings = append(pool.Standings, standingView(row, side))
				pool.DrawNeeded = pool.DrawNeeded || row.DrawNeeded
			}
			for _, m := range matches {
				if m.Category == c && m.Stage == stage.ID && m.Pool == i {
					pool.Matches = append(pool.Matches, mv(m))
				}
			}
			sp.Pools = append(sp.Pools, pool)
		}
		return sp
	}
	for _, id := range stage.Slots() {
		if id == "" {
			sp.Slots = append(sp.Slots, SideView{Names: []string{}, Clubs: []string{}})
		} else {
			sp.Slots = append(sp.Slots, side(id))
		}
	}
	var current *RoundView
	for _, m := range matches {
		if m.Category != c || m.Stage != stage.ID {
			continue
		}
		if current == nil || current.From != m.Start || current.Size != m.Size {
			sp.Rounds = append(sp.Rounds, RoundView{From: m.Start, To: m.Start + m.Size - 1, Size: m.Size, Matches: []MatchView{}})
			current = &sp.Rounds[len(sp.Rounds)-1]
		}
		current.Matches = append(current.Matches, mv(m))
	}
	for _, p := range st.Places(c, stage.ID) {
		sp.Places = append(sp.Places, PlaceView{From: p.From, To: p.To, Entrant: side(p.Entrant)})
	}
	return sp
}

func standingView(row engine.Standing, side func(engine.EntrantID) SideView) StandingView {
	return StandingView{
		Rank: row.Rank, Entrant: side(row.Entrant), Played: row.Played, Wins: row.Wins, Losses: row.Losses,
		SetsWon: row.SetsWon, SetsLost: row.SetsLost, PointsWon: row.PointsWon, PointsLost: row.PointsLost,
		Withdrawn: row.Withdrawn, DrawNeeded: row.DrawNeeded,
	}
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
