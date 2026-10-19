package main

import (
	"context"
	"slices"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

const maxClubName = 80

// SaveClub adds a club to the list offered when entering a player (old ""),
// or renames one. A club already listed (case ignored) is not added twice.
func (s *PresetService) SaveClub(ctx context.Context, old, name string) error {
	name, err := presetName(name, maxClubName, true)
	if err != nil {
		return engine.NewError(engine.CodeInvalidClub)
	}
	found := old == ""
	err = s.update(ctx, func(p *Presets) {
		if old != "" {
			i := slices.Index(p.Clubs, old)
			if found = i >= 0; found {
				p.Clubs = slices.Delete(p.Clubs, i, i+1)
			}
		}
		if found {
			p.Clubs = addClubs(p.Clubs, name)
		}
	})
	if err == nil && !found {
		return engine.NewError(engine.CodeClubNotFound)
	}
	return err
}

// DeleteClub removes a club from the list. Players keep their club.
func (s *PresetService) DeleteClub(ctx context.Context, name string) error {
	found := false
	err := s.update(ctx, func(p *Presets) {
		n := len(p.Clubs)
		p.Clubs = slices.DeleteFunc(p.Clubs, func(c string) bool { return c == name })
		found = len(p.Clubs) != n
	})
	if err == nil && !found {
		return engine.NewError(engine.CodeClubNotFound)
	}
	return err
}

// addClubs adds the clubs not listed yet (case and accents ignored) and
// keeps the list sorted.
func addClubs(list []string, names ...string) []string {
	c := collate.New(language.French, collate.IgnoreCase, collate.IgnoreDiacritics)
	for _, name := range names {
		name = strings.Join(strings.Fields(name), " ")
		if name == "" || slices.ContainsFunc(list, func(x string) bool { return c.CompareString(x, name) == 0 }) {
			continue
		}
		list = append(list, name)
	}
	slices.SortStableFunc(list, c.CompareString)
	return list
}
