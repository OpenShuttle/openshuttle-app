package main

import (
	"context"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// PlayerView is a player as shown on the entrants and check-in screens.
type PlayerView struct {
	ID   tournament.PlayerID `json:"id"`
	Name string              `json:"name"`
	// Club is the club's name in the club list, else as typed; ClubShown
	// the club as the settings show it.
	Club      string `json:"club"`
	ClubShown string `json:"clubShown"`
	CheckedIn bool   `json:"checkedIn"`
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
	// Waiting is a player registered alone in doubles, waiting for a partner.
	Waiting bool `json:"waiting"`
	// Locked: the entrant has matches, so it can no longer be removed or
	// split (a forfeit is declared instead).
	Locked  bool         `json:"locked"`
	Players []PlayerView `json:"players"`
}

// CategoryEntrants lists the entrants of one category, with check-in totals.
type CategoryEntrants struct {
	CategoryRef
	Doubles          bool          `json:"doubles"`
	Entrants         []EntrantView `json:"entrants"`
	Players          int           `json:"players"`
	PlayersPresent   int           `json:"playersPresent"`
	EntrantsComplete int           `json:"entrantsComplete"`
	// Waiting counts the players waiting for a partner.
	Waiting int `json:"waiting"`
}

// EntrantsView is the entrants screen: one section per category.
type EntrantsView struct {
	Categories []CategoryEntrants `json:"categories"`
}

// CheckInTeam is a row of the check-in screen: an entrant with its players,
// a pair shown together.
type CheckInTeam struct {
	EntrantID engine.EntrantID `json:"entrantId"`
	CategoryRef
	Bib      int          `json:"bib"`
	Players  []PlayerView `json:"players"`
	Complete bool         `json:"complete"`
	Waiting  bool         `json:"waiting"`
}

// CheckInView is the check-in screen: every entrant, sorted by the names of
// its players, the ones fully checked in last, with totals per category.
type CheckInView struct {
	Teams      []CheckInTeam      `json:"teams"`
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
	players = s.listedClubs(players)
	if err := s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.RegisterEntrant(c, players) }); err != nil {
		return err
	}
	s.rememberClubs(ctx, players)
	return nil
}

// UpdateEntrant changes the names and clubs of an entrant's players.
func (s *TournamentService) UpdateEntrant(ctx context.Context, id engine.EntrantID, players []tournament.PlayerInput) error {
	players = s.listedClubs(players)
	if err := s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.UpdateEntrant(id, players) }); err != nil {
		return err
	}
	s.rememberClubs(ctx, players)
	return nil
}

// listedClubs enters the clubs typed for players as the clubs of the list
// they name, by name or acronym ("AUVR" is entered as "Rillieux").
func (s *TournamentService) listedClubs(players []tournament.PlayerInput) []tournament.PlayerInput {
	book := s.clubBook()
	out := slices.Clone(players)
	for i := range out {
		out[i].Club = book.Name(strings.Join(strings.Fields(out[i].Club), " "))
	}
	return out
}

// rememberClubs adds the clubs typed for players to the club list. The
// list is a convenience: failing to update it never fails the entry.
func (s *TournamentService) rememberClubs(ctx context.Context, players []tournament.PlayerInput) {
	var clubs []tournament.Club
	for _, p := range players {
		clubs = append(clubs, tournament.Club{Name: p.Club})
	}
	added := false
	err := updatePresets(ctx, s.app, s.emit, func(p *Presets) {
		n := len(p.Clubs)
		p.Clubs = addClubs(p.Clubs, clubs...)
		added = len(p.Clubs) != n
	})
	if err == nil && added {
		s.refreshClubs(ctx)
	}
}

// PairEntrants pairs two players waiting for a partner: the partner's
// player joins the entrant id.
func (s *TournamentService) PairEntrants(ctx context.Context, id, partner engine.EntrantID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.PairEntrants(id, partner) })
}

// AddPartner adds a partner, not registered before, to a player waiting
// for one.
func (s *TournamentService) AddPartner(ctx context.Context, id engine.EntrantID, partner tournament.PlayerInput) error {
	partner = s.listedClubs([]tournament.PlayerInput{partner})[0]
	if err := s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.AddPartner(id, partner) }); err != nil {
		return err
	}
	s.rememberClubs(ctx, []tournament.PlayerInput{partner})
	return nil
}

// SplitEntrant takes a player out of a pair: both wait for a partner again.
func (s *TournamentService) SplitEntrant(ctx context.Context, id engine.EntrantID, player tournament.PlayerID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SplitEntrant(id, player) })
}

// RemoveEntrant withdraws an entrant.
func (s *TournamentService) RemoveEntrant(ctx context.Context, id engine.EntrantID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.RemoveEntrant(id) })
}

// CheckInPlayer marks a player present, assigning the bib on the pair's first check-in.
func (s *TournamentService) CheckInPlayer(ctx context.Context, id tournament.PlayerID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.CheckInPlayer(id) })
}

// CheckInEntrant marks every absent player of an entrant present at once.
func (s *TournamentService) CheckInEntrant(ctx context.Context, id engine.EntrantID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.CheckInEntrant(id) })
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
			ev := EntrantView{ID: e.ID, Bib: e.Bib, Complete: e.Complete(), Waiting: e.Waiting(), Locked: st.InMatches(e.ID)}
			for _, p := range e.Players {
				ev.Players = append(ev.Players, playerView(st, p, dup[p.ID]))
				ce.Players++
				if p.CheckedIn {
					ce.PlayersPresent++
				}
			}
			if ev.Complete {
				ce.EntrantsComplete++
			}
			if ev.Waiting {
				ce.Waiting++
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
	v := CheckInView{Teams: []CheckInTeam{}, Categories: categoryEntrants(st)}
	for _, e := range st.Entrants {
		team := CheckInTeam{
			EntrantID: e.ID, CategoryRef: CategoryRef{Category: e.Category, Slot: slots[e.Category]},
			Bib: e.Bib, Players: []PlayerView{}, Complete: e.Complete(), Waiting: e.Waiting(),
		}
		for _, p := range e.Players {
			team.Players = append(team.Players, playerView(st, p, dup[p.ID]))
			v.Players++
			if p.CheckedIn {
				v.Present++
			}
		}
		v.Teams = append(v.Teams, team)
	}
	sortTeams(v.Teams)
	return v
}

func playerView(st tournament.State, p tournament.Player, duplicate bool) PlayerView {
	clubs := st.ClubBook()
	v := PlayerView{
		ID: p.ID, Name: p.Name, Club: clubs.Name(p.Club), ClubShown: clubs.Label(p.Club),
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

// ImportFile is a player list chosen for import: its path, its lines and
// the mapping guessed from them, for the frontend to show and adjust.
type ImportFile struct {
	Path    string                   `json:"path"`
	Lines   []tournament.ImportLine  `json:"lines"`
	Mapping tournament.ImportMapping `json:"mapping"`
}

// ChooseImportFile asks for a player list (CSV from OpenShuttle, a club
// registration tool such as HelloAsso or a spreadsheet) and reads it. Lines
// without category go to fallback. A cancelled dialog gives an empty path.
func (s *TournamentService) ChooseImportFile(ctx context.Context, fallback engine.Category) (ImportFile, error) {
	path, err := s.dialogs.OpenCSV(s.dialogDir(ctx, dialogImport))
	if err != nil || path == "" {
		return ImportFile{Lines: []tournament.ImportLine{}}, err
	}
	s.rememberDir(ctx, dialogImport, path)
	lines, err := readImportFile(path)
	if err != nil {
		return ImportFile{Lines: []tournament.ImportLine{}}, err
	}
	return ImportFile{Path: path, Lines: lines, Mapping: tournament.GuessImportMapping(lines, fallback)}, nil
}

func readImportFile(path string) ([]tournament.ImportLine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, engine.NewError(engine.CodeInvalidFile)
	}
	return tournament.ReadImportTable(data)
}

// PreviewImport is what importing the file with this mapping would do,
// nothing saved.
func (s *TournamentService) PreviewImport(path string, m tournament.ImportMapping) (tournament.ImportPreview, error) {
	lines, err := readImportFile(path)
	if err != nil {
		return tournament.ImportPreview{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return tournament.ImportPreview{}, engine.NewError(engine.CodeNoTournament)
	}
	return s.state.PreviewImport(lines, m), nil
}

// ImportEntrants enters the lines of a player list read with a mapping, in
// one undoable action. It first adds the clubs chosen for club values
// matching no listed club (add) to the club list, then enters each club as
// the listed club it names (m.Clubs for the others, else as typed). It
// reports what was added, skipped and ignored.
func (s *TournamentService) ImportEntrants(ctx context.Context, path string, m tournament.ImportMapping, add []tournament.Club) (tournament.ImportReport, error) {
	report := tournament.ImportReport{Skipped: []tournament.ImportSkip{}}
	lines, err := readImportFile(path)
	if err != nil {
		return report, err
	}
	if len(add) > 0 {
		if err := updatePresets(ctx, s.app, s.emit, func(p *Presets) { p.Clubs = addClubs(p.Clubs, add...) }); err != nil {
			return report, err
		}
		s.refreshClubs(ctx)
	}
	err = s.execute(ctx, func(st tournament.State) (tournament.Change, error) {
		ch, r, err := st.ImportEntrants(lines, m)
		report = r
		return ch, err
	})
	return report, err
}
