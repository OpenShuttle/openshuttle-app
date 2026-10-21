package main

import (
	"context"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// SetScheduleMode chooses how two categories share the courts: "alternate"
// or "reserved".
func (s *TournamentService) SetScheduleMode(ctx context.Context, mode tournament.ScheduleMode) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetScheduleMode(mode) })
}

// SetScheduleFirst chooses the category opening each round in alternate mode.
func (s *TournamentService) SetScheduleFirst(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetScheduleFirst(c) })
}

// SetCourtCategory reserves a court for a category ("" for the default share).
func (s *TournamentService) SetCourtCategory(ctx context.Context, court int, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetCourtCategory(court, c) })
}

// ValidateSchedule fixes the order of the waiting matches from the rules
// (again: recomputes it, dropping the moves made by hand).
func (s *TournamentService) ValidateSchedule(ctx context.Context) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ValidateSchedule() })
}

// MoveScheduledMatch moves a match just before another one ("" at the end).
func (s *TournamentService) MoveScheduledMatch(ctx context.Context, id, before tournament.MatchID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.MoveScheduledMatch(id, before) })
}

// MoveScheduledBlock moves a block (pool or bracket round) just before
// another one ("" at the end), before the schedule is validated.
func (s *TournamentService) MoveScheduledBlock(ctx context.Context, key, before string) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.MoveScheduledBlock(key, before) })
}

// UnvalidateSchedule goes back to moving blocks, dropping the matches moved
// by hand.
func (s *TournamentService) UnvalidateSchedule(ctx context.Context) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.UnvalidateSchedule() })
}

// GetScheduleView returns the schedule screen.
func (s *TournamentService) GetScheduleView() ScheduleView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scheduleView(s.state, s.now())
}

// PreviewScheduledBlockMove returns the schedule screen as it would be with
// a block moved just before another one: what the desk sees while dragging.
// Nothing is saved.
func (s *TournamentService) PreviewScheduledBlockMove(key, before string) ScheduleView {
	return s.previewSchedule(func(st tournament.State) (tournament.Change, error) { return st.MoveScheduledBlock(key, before) })
}

// PreviewScheduledMatchMove returns the schedule screen as it would be with a
// match moved just before another one. Nothing is saved.
func (s *TournamentService) PreviewScheduledMatchMove(id, before tournament.MatchID) ScheduleView {
	return s.previewSchedule(func(st tournament.State) (tournament.Change, error) { return st.MoveScheduledMatch(id, before) })
}

// previewSchedule projects the log with the command's event added, in memory
// only. A refused or empty move previews the schedule as it is.
func (s *TournamentService) previewSchedule(cmd func(tournament.State) (tournament.Change, error)) ScheduleView {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	st := s.state
	if s.file == nil {
		return s.scheduleView(st, now)
	}
	if ch, err := cmd(st); err == nil && !ch.None() {
		if d, err := events.NewDraft(ch.Type, ch.Data, now, ""); err == nil {
			log := append(slices.Clone(s.log), events.Event{Seq: st.Version + 1, Type: d.Type, Data: d.Data, At: d.At})
			if p, err := tournament.Project(log); err == nil {
				st = p
			}
		}
	}
	return s.scheduleView(st, now)
}
