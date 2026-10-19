package main

import (
	"context"
	"os"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// PlayerView is a player as shown on the entrants and check-in screens.
type PlayerView struct {
	ID        tournament.PlayerID `json:"id"`
	Name      string              `json:"name"`
	Club      string              `json:"club"`
	CheckedIn bool                `json:"checkedIn"`
	// CheckedInAt is when the player was checked in.
	CheckedInAt *time.Time `json:"checkedInAt"`
	// Duplicate warns that another player has the same full name.
	Duplicate bool `json:"duplicate"`
}

// EntrantView is a row of the entrants screen.
type EntrantView struct {
	ID       engine.EntrantID `json:"id"`
	Bib      int              `json:"bib"`
	Complete bool             `json:"complete"`
	Players  []PlayerView     `json:"players"`
}

// CategoryEntrants lists the entrants of one category, with check-in totals.
type CategoryEntrants struct {
	CategoryRef
	Doubles          bool          `json:"doubles"`
	Entrants         []EntrantView `json:"entrants"`
	Players          int           `json:"players"`
	PlayersPresent   int           `json:"playersPresent"`
	EntrantsComplete int           `json:"entrantsComplete"`
}

// EntrantsView is the entrants screen: one section per category.
type EntrantsView struct {
	Categories []CategoryEntrants `json:"categories"`
}

// CheckInRow is a row of the check-in screen: one per player.
type CheckInRow struct {
	PlayerView
	EntrantID engine.EntrantID `json:"entrantId"`
	CategoryRef
	Bib int `json:"bib"`
	// Partner is the other player of a pair, nil in singles.
	Partner *PlayerView `json:"partner"`
}

// CheckInView is the check-in screen: every player, sorted by last name,
// with totals per category.
type CheckInView struct {
	Rows       []CheckInRow       `json:"rows"`
	Categories []CategoryEntrants `json:"categories"`
	Players    int                `json:"players"`
	Present    int                `json:"present"`
}

// AddCategory adds a category (MS, WS, MD, WD, XD) to the tournament.
func (s *TournamentService) AddCategory(ctx context.Context, c engine.Category) error {
	stages := s.defaultFormat(ctx)
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.AddCategory(c, stages...) })
}

// RemoveCategory removes a category that has no entrant.
func (s *TournamentService) RemoveCategory(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.RemoveCategory(c) })
}

// RegisterEntrant enters a player (singles) or a pair (doubles).
func (s *TournamentService) RegisterEntrant(ctx context.Context, c engine.Category, players []tournament.PlayerInput) error {
	if err := s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.RegisterEntrant(c, players) }); err != nil {
		return err
	}
	s.rememberClubs(ctx, players)
	return nil
}

// UpdateEntrant changes the names and clubs of an entrant's players.
func (s *TournamentService) UpdateEntrant(ctx context.Context, id engine.EntrantID, players []tournament.PlayerInput) error {
	if err := s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.UpdateEntrant(id, players) }); err != nil {
		return err
	}
	s.rememberClubs(ctx, players)
	return nil
}

// rememberClubs adds the clubs typed for players to the club list. The
// list is a convenience: failing to update it never fails the entry.
func (s *TournamentService) rememberClubs(ctx context.Context, players []tournament.PlayerInput) {
	var clubs []string
	for _, p := range players {
		clubs = append(clubs, p.Club)
	}
	_ = updatePresets(ctx, s.app, s.emit, func(p *Presets) { p.Clubs = addClubs(p.Clubs, clubs...) })
}

// RemoveEntrant withdraws an entrant.
func (s *TournamentService) RemoveEntrant(ctx context.Context, id engine.EntrantID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.RemoveEntrant(id) })
}

// CheckInPlayer marks a player present, assigning the bib on the pair's first check-in.
func (s *TournamentService) CheckInPlayer(ctx context.Context, id tournament.PlayerID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.CheckInPlayer(id) })
}

// CancelCheckIn marks a player absent again.
func (s *TournamentService) CancelCheckIn(ctx context.Context, id tournament.PlayerID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.CancelCheckIn(id) })
}

// GetEntrantsView returns the entrants screen.
func (s *TournamentService) GetEntrantsView() EntrantsView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return EntrantsView{Categories: categoryEntrants(s.state)}
}

// GetCheckInView returns the check-in screen.
func (s *TournamentService) GetCheckInView() CheckInView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return checkInView(s.state)
}

func categoryRefs(st tournament.State) []CategoryRef {
	refs := make([]CategoryRef, len(st.Categories))
	for i, c := range st.Categories {
		refs[i] = CategoryRef{Category: c, Slot: i + 1}
	}
	return refs
}

func categoryEntrants(st tournament.State) []CategoryEntrants {
	dup := st.DuplicatePlayers()
	out := make([]CategoryEntrants, 0, len(st.Categories))
	for _, ref := range categoryRefs(st) {
		ce := CategoryEntrants{CategoryRef: ref, Doubles: ref.Category.IsDoubles(), Entrants: []EntrantView{}}
		for _, e := range st.Entrants {
			if e.Category != ref.Category {
				continue
			}
			ev := EntrantView{ID: e.ID, Bib: e.Bib, Complete: e.Complete()}
			for _, p := range e.Players {
				ev.Players = append(ev.Players, playerView(p, dup[p.ID]))
				ce.Players++
				if p.CheckedIn {
					ce.PlayersPresent++
				}
			}
			if ev.Complete {
				ce.EntrantsComplete++
			}
			ce.Entrants = append(ce.Entrants, ev)
		}
		out = append(out, ce)
	}
	return out
}

func checkInView(st tournament.State) CheckInView {
	dup := st.DuplicatePlayers()
	slots := map[engine.Category]int{}
	for _, ref := range categoryRefs(st) {
		slots[ref.Category] = ref.Slot
	}
	v := CheckInView{Rows: []CheckInRow{}, Categories: categoryEntrants(st)}
	for _, e := range st.Entrants {
		for i, p := range e.Players {
			row := CheckInRow{
				PlayerView:  playerView(p, dup[p.ID]),
				EntrantID:   e.ID,
				CategoryRef: CategoryRef{Category: e.Category, Slot: slots[e.Category]},
				Bib:         e.Bib,
			}
			if len(e.Players) == 2 {
				partner := playerView(e.Players[1-i], dup[e.Players[1-i].ID])
				row.Partner = &partner
			}
			v.Rows = append(v.Rows, row)
			v.Players++
			if p.CheckedIn {
				v.Present++
			}
		}
	}
	sortRows(v.Rows)
	return v
}

func playerView(p tournament.Player, duplicate bool) PlayerView {
	v := PlayerView{
		ID: p.ID, Name: p.Name, Club: p.Club,
		CheckedIn: p.CheckedIn, Duplicate: duplicate,
	}
	if p.CheckedIn && !p.CheckedInAt.IsZero() {
		at := p.CheckedInAt
		v.CheckedInAt = &at
	}
	return v
}

// defaultFormat is the format a new category starts with: the default
// format preset of the settings.
func (s *TournamentService) defaultFormat(ctx context.Context) []tournament.StageConfig {
	settings, err := loadSettings(ctx, s.app)
	if err != nil {
		return nil
	}
	presets, err := readPresets(ctx, s.app)
	if err != nil {
		return nil
	}
	for _, f := range presets.Formats {
		if f.ID == settings.DefaultFormat {
			return f.Stages
		}
	}
	return nil
}

// ImportEntrants asks for a player list (CSV, the layout of the entrants
// export) and enters its rows, in one undoable action. Rows without a
// category go to fallback. It reports what was added and skipped; a
// cancelled dialog reports nothing.
func (s *TournamentService) ImportEntrants(ctx context.Context, fallback engine.Category) (tournament.ImportReport, error) {
	report := tournament.ImportReport{Skipped: []tournament.ImportSkip{}}
	path, err := s.dialogs.OpenCSV(s.tournamentsDir(ctx))
	if err != nil || path == "" {
		return report, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return report, engine.NewError(engine.CodeInvalidFile)
	}
	rows, err := tournament.ParseEntrantsCSV(data)
	if err != nil {
		return report, err
	}
	err = s.execute(ctx, func(st tournament.State) (tournament.Change, error) {
		ch, r, err := st.ImportEntrants(rows, fallback)
		report = r
		return ch, err
	})
	if err != nil {
		return report, err
	}
	var players []tournament.PlayerInput
	for _, r := range rows {
		players = append(players, r.Players...)
	}
	s.rememberClubs(ctx, players)
	return report, nil
}
