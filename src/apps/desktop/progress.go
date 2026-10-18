package main

import (
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// Tournament stages shown on the home screen.
const (
	StagePreparation = "preparation" // no draw confirmed yet
	StageDayOf       = "dayOf"       // a draw is confirmed
	StageArchived    = "archived"
)

// ProgressView sums up a tournament for the side navigation and the top bar.
type ProgressView struct {
	Entrants int `json:"entrants"`
	// Doubles is true when every category is played in pairs.
	Doubles        bool `json:"doubles"`
	Players        int  `json:"players"`
	PlayersPresent int  `json:"playersPresent"`
	// DrawsConfirmed counts the categories whose draw is confirmed.
	DrawsConfirmed int `json:"drawsConfirmed"`
	// DrawConfirmedAt is when the latest draw was confirmed, once all are.
	DrawConfirmedAt *time.Time `json:"drawConfirmedAt"`
	// Started is set once a match has been called.
	Started bool `json:"started"`
	// Matches counts the known matches (byes excluded); Played the decided ones.
	Matches int `json:"matches"`
	Played  int `json:"played"`
	// Final is set once every category has its full standings.
	Final bool   `json:"final"`
	Stage string `json:"stage"`
}

func progressOf(st tournament.State) ProgressView {
	p := ProgressView{Entrants: len(st.Entrants), Doubles: len(st.Categories) > 0}
	for _, c := range st.Categories {
		p.Doubles = p.Doubles && c.IsDoubles()
	}
	for _, e := range st.Entrants {
		for _, pl := range e.Players {
			p.Players++
			if pl.CheckedIn {
				p.PlayersPresent++
			}
		}
	}
	var latest time.Time
	for _, c := range st.Categories {
		if d, ok := st.Draw(c); ok && d.Confirmed {
			p.DrawsConfirmed++
			if d.ConfirmedAt.After(latest) {
				latest = d.ConfirmedAt
			}
		}
	}
	if p.DrawsConfirmed > 0 && p.DrawsConfirmed == len(st.Categories) {
		p.DrawConfirmedAt = &latest
	}
	for _, m := range st.Matches() {
		if m.ByeA || m.ByeB {
			continue
		}
		p.Matches++
		if st.Decided(m) {
			p.Played++
		}
		p.Started = p.Started || m.Called()
	}
	p.Final = len(st.Categories) > 0
	for _, c := range st.Categories {
		if _, complete := st.FinalStandings(c); !complete {
			p.Final = false
		}
	}
	switch {
	case st.Archived:
		p.Stage = StageArchived
	case p.DrawsConfirmed > 0:
		p.Stage = StageDayOf
	default:
		p.Stage = StagePreparation
	}
	return p
}

// TournamentSummary describes a tournament in the recent list.
type TournamentSummary struct {
	Categories []engine.Category `json:"categories"`
	Entrants   int               `json:"entrants"`
	Doubles    bool              `json:"doubles"`
	Courts     int               `json:"courts"`
	Stage      string            `json:"stage"`
	// UpdatedAt is the time of the latest event.
	UpdatedAt time.Time `json:"updatedAt"`
}

func summaryOf(st tournament.State, updatedAt time.Time) *TournamentSummary {
	p := progressOf(st)
	return &TournamentSummary{
		Categories: append([]engine.Category{}, st.Categories...),
		Entrants:   p.Entrants,
		Doubles:    p.Doubles,
		Courts:     st.Courts(),
		Stage:      p.Stage,
		UpdatedAt:  updatedAt.UTC(),
	}
}
