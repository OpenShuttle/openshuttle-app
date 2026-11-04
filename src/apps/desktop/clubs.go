package main

import (
	"context"
	"slices"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

const (
	maxClubName    = 80
	maxClubAcronym = 20
)

// SaveClub adds a club to the list offered when entering a player (old ""),
// or changes the club named old. The name is required, the acronym
// optional; case, accents and punctuation ignored. A listed club the new
// one fully covers ("AUVR" alone, when saving Rillieux, AUVR) merges into
// it; any other club already holding the name or acronym refuses it
// (CLUB_EXISTS).
func (s *PresetService) SaveClub(ctx context.Context, old string, club tournament.Club) error {
	club, err := normalizeClub(club)
	if err != nil {
		return err
	}
	found, taken := old == "", false
	err = s.update(ctx, func(p *Presets) {
		list := slices.Clone(p.Clubs)
		if old != "" {
			i := slices.IndexFunc(list, func(c tournament.Club) bool { return c.Name == old })
			if found = i >= 0; !found {
				return
			}
			list = slices.Delete(list, i, i+1)
		}
		keys := []string{tournament.ClubKey(club.Name), tournament.ClubKey(club.Acronym)}
		covered := func(c tournament.Club) bool {
			return slices.Contains(keys, tournament.ClubKey(c.Name)) &&
				(c.Acronym == "" || slices.Contains(keys, tournament.ClubKey(c.Acronym)))
		}
		list = slices.DeleteFunc(list, covered)
		book := tournament.NewClubBook(list, tournament.ClubDisplayName)
		for _, v := range []string{club.Name, club.Acronym} {
			if _, ok := book.Find(v); ok && v != "" {
				taken = true
				return
			}
		}
		p.Clubs = sortClubs(append(list, club))
	})
	switch {
	case err != nil:
		return err
	case !found:
		return engine.NewError(engine.CodeClubNotFound)
	case taken:
		return engine.NewError(engine.CodeClubExists)
	}
	return nil
}

// DeleteClub removes a club from the list. Players keep their club.
func (s *PresetService) DeleteClub(ctx context.Context, name string) error {
	found := false
	err := s.update(ctx, func(p *Presets) {
		n := len(p.Clubs)
		p.Clubs = slices.DeleteFunc(p.Clubs, func(c tournament.Club) bool { return c.Name == name })
		found = len(p.Clubs) != n
	})
	if err == nil && !found {
		return engine.NewError(engine.CodeClubNotFound)
	}
	return err
}

// normalizeClub trims a club and checks it. A club given by its acronym
// alone takes it as its name.
func normalizeClub(c tournament.Club) (tournament.Club, error) {
	name, err := presetName(c.Name, maxClubName, false)
	if err != nil {
		return c, engine.NewError(engine.CodeInvalidClub)
	}
	acronym, err := presetName(c.Acronym, maxClubAcronym, false)
	if err != nil {
		return c, engine.NewError(engine.CodeInvalidClub)
	}
	if name == "" {
		name, acronym = acronym, ""
	}
	if name == "" {
		return c, engine.NewError(engine.CodeInvalidClub)
	}
	if tournament.ClubKey(acronym) == tournament.ClubKey(name) {
		acronym = ""
	}
	return tournament.Club{Name: name, Acronym: acronym}, nil
}

// addClubs adds the clubs whose name and acronym name no listed club yet,
// and keeps the list sorted.
func addClubs(list []tournament.Club, clubs ...tournament.Club) []tournament.Club {
	for _, c := range clubs {
		c, err := normalizeClub(c)
		if err != nil {
			continue
		}
		book := tournament.NewClubBook(list, tournament.ClubDisplayName)
		if _, ok := book.Find(c.Name); ok {
			continue
		}
		if _, ok := book.Find(c.Acronym); ok && c.Acronym != "" {
			continue
		}
		list = append(list, c)
	}
	return sortClubs(list)
}

// sortClubs sorts clubs by name, case and accents ignored.
func sortClubs(list []tournament.Club) []tournament.Club {
	c := collate.New(language.French, collate.IgnoreCase, collate.IgnoreDiacritics)
	slices.SortStableFunc(list, func(a, b tournament.Club) int { return c.CompareString(a.Name, b.Name) })
	return list
}
