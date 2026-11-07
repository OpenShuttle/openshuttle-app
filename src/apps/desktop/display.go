package main

import (
	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// DisplayView is the display screen: the stages being played, shown in
// turn, and the latest results.
type DisplayView struct {
	Name       string        `json:"name"`
	Categories []CategoryRef `json:"categories"`
	// Round is the timed round in progress, for the shared countdown.
	Round *TimedRoundView `json:"round"`
	// Stages are the stages the screen rotates through: the ones being
	// played, among those the desk shows (else the next ones, else the last
	// ones over).
	Stages []DisplayStage `json:"stages"`
	// Results are the latest scores entered, newest first.
	Results []MatchView `json:"results"`
}

// DisplayStage is a stage on the display screen: its pools, or its bracket.
type DisplayStage struct {
	CategoryRef
	StagePhase
}

// displayResults is how many latest results the display screen lists.
const displayResults = 12

// GetDisplayView returns the display screen.
func (s *TournamentService) GetDisplayView() DisplayView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	mv := s.matchViewer(st)
	v := DisplayView{Name: st.Name, Categories: categoryRefs(st), Stages: []DisplayStage{}, Results: []MatchView{}}
	v.Round = s.roundsView(st).Current
	slots := map[engine.Category]CategoryRef{}
	for _, ref := range v.Categories {
		slots[ref.Category] = ref
	}
	stages := map[engine.Category][]tournament.Stage{}
	for _, ref := range st.DisplayStages() {
		if stages[ref.Category] == nil {
			stages[ref.Category] = st.Stages(ref.Category)
		}
		for i, stage := range stages[ref.Category] {
			if stage.ID == ref.Stage {
				v.Stages = append(v.Stages, DisplayStage{slots[ref.Category], s.stagePhase(st, ref.Category, stage, i == 0)})
			}
		}
	}
	for _, m := range st.LatestScores(displayResults) {
		v.Results = append(v.Results, mv(m))
	}
	return v
}
