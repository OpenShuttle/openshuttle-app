package main

import (
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// DisplayCourt is a court on the display screen.
type DisplayCourt struct {
	Number int                   `json:"number"`
	State  tournament.CourtState `json:"state"`
	Match  *MatchView            `json:"match"` // nil when free
	// Proposal is the match coming next on a free or finished court.
	Proposal *MatchView `json:"proposal"`
}

// DisplayView is the display screen: latest calls in big, then every court.
type DisplayView struct {
	Name       string        `json:"name"`
	Categories []CategoryRef `json:"categories"`
	// Calls are the latest calls, newest first: the first one in big.
	Calls  []MatchView    `json:"calls"`
	Courts []DisplayCourt `json:"courts"`
	// Round is the timed round in progress, for the shared countdown.
	Round *TimedRoundView `json:"round"`
	// Boards are the standings and brackets the screen rotates through.
	Boards []DisplayBoard `json:"boards"`
}

// DisplayBoard is one standings view of the display screen rotation: a pool
// ranking, or the matches of a bracket.
type DisplayBoard struct {
	CategoryRef
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	Pool       string               `json:"pool"` // pool boards
	Standings  []StandingView       `json:"standings"`
	Matches    []MatchView          `json:"matches"`
}

// GetDisplayView returns the display screen.
func (s *TournamentService) GetDisplayView() DisplayView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	mv := s.matchViewer(st)
	v := DisplayView{Name: st.Name, Categories: categoryRefs(st), Calls: []MatchView{}, Courts: []DisplayCourt{}}
	proposals := st.Proposals(s.now())
	var playing []tournament.Match
	for _, c := range st.CourtStates() {
		dc := DisplayCourt{Number: c.Number, State: c.State}
		if c.Match != nil && c.State != tournament.CourtFree {
			m := mv(*c.Match)
			dc.Match = &m
		}
		if c.State == tournament.CourtPlaying {
			playing = append(playing, *c.Match)
		} else if p, ok := proposals[c.Number]; ok {
			if pm, ok := st.Match(p.Match); ok {
				m := mv(pm)
				dc.Proposal = &m
			}
		}
		v.Courts = append(v.Courts, dc)
	}
	// Latest calls first: the hall reads the top of the screen.
	slices.SortStableFunc(playing, func(a, b tournament.Match) int { return b.CalledAt.Compare(a.CalledAt) })
	for _, m := range playing[:min(len(playing), 4)] {
		v.Calls = append(v.Calls, mv(m))
	}
	v.Round = s.roundsView(st).Current
	v.Boards = displayBoards(st, mv)
	return v
}

// displayBoards lists, per category and stage, each pool ranking and each
// filled bracket with its matches whose entrants are known.
func displayBoards(st tournament.State, mv func(tournament.Match) MatchView) []DisplayBoard {
	side := sideViewer(st)
	boards := []DisplayBoard{}
	matches := st.Matches()
	for _, ref := range categoryRefs(st) {
		for i, stage := range st.Stages(ref.Category) {
			if !stage.Confirmed {
				continue
			}
			base := DisplayBoard{CategoryRef: ref, Stage: stage.ID, StageName: stage.Name, StageKind: stage.Kind,
				EntryStage: i == 0, Standings: []StandingView{}, Matches: []MatchView{}}
			if stage.Kind == tournament.StagePools {
				for p := range stage.Groups {
					b := base
					b.Pool, b.Standings = engine.PoolName(p), []StandingView{}
					for _, row := range st.PoolStandings(ref.Category, stage.ID, p).Rows {
						b.Standings = append(b.Standings, standingView(row, side))
					}
					boards = append(boards, b)
				}
				continue
			}
			for _, m := range matches {
				if m.Category == ref.Category && m.Stage == stage.ID && st.Playable(m) {
					base.Matches = append(base.Matches, mv(m))
				}
			}
			boards = append(boards, base)
		}
	}
	return boards
}
