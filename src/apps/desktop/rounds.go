package main

import (
	"context"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// TimedRoundView is the round in progress: the shared countdown runs from
// StartedAt to EndsAt. The frontend only renders it.
type TimedRoundView struct {
	Number    int         `json:"number"`
	Minutes   int         `json:"minutes"`
	StartedAt time.Time   `json:"startedAt"`
	EndsAt    time.Time   `json:"endsAt"`
	Matches   []MatchView `json:"matches"`
}

// RoundsView is the round banner of the control desk.
type RoundsView struct {
	// Active is set when timed matches remain: the desk plays by rounds.
	Active  bool            `json:"active"`
	Current *TimedRoundView `json:"current"`
	Next    []MatchView     `json:"next"` // what "start the round" would call
	Played  int             `json:"played"`
}

// StartRound calls the next round on every free court.
func (s *TournamentService) StartRound(ctx context.Context) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.StartRound() })
}

// EndRound whistles the end of the round in progress.
func (s *TournamentService) EndRound(ctx context.Context) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.EndRound() })
}

func (s *TournamentService) roundsView(st tournament.State) RoundsView {
	mv := s.matchViewer(st)
	v := RoundsView{Next: []MatchView{}, Played: len(st.Rounds())}
	for _, m := range st.Matches() {
		if st.Timed(m) && !st.Decided(m) && !(m.ByeA || m.ByeB) {
			v.Active = true
			break
		}
	}
	if r, ok := st.CurrentRound(); ok {
		v.Active = true
		v.Played = r.Number - 1
		rv := TimedRoundView{
			Number: r.Number, Minutes: r.Minutes, StartedAt: r.StartedAt,
			EndsAt: r.StartedAt.Add(time.Duration(r.Minutes) * time.Minute), Matches: []MatchView{},
		}
		for _, call := range r.Calls {
			if m, ok := st.Match(call.Match); ok {
				rv.Matches = append(rv.Matches, mv(m))
			}
		}
		v.Current = &rv
		return v
	}
	for _, call := range st.NextRound() {
		if m, ok := st.Match(call.Match); ok {
			view := mv(m)
			view.Court = call.Court
			v.Next = append(v.Next, view)
		}
	}
	return v
}
