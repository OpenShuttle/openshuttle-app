package main

import (
	"slices"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// sortTeams orders check-in teams by the first of their players' names, the
// way a French or English reader expects (accents ignored, case-insensitive).
func sortTeams(teams []CheckInTeam) {
	c := collate.New(language.French, collate.IgnoreCase, collate.IgnoreDiacritics)
	first := func(t CheckInTeam) string {
		name := ""
		for _, p := range t.Players {
			if n := strings.TrimSpace(p.Name); name == "" || c.CompareString(n, name) < 0 {
				name = n
			}
		}
		return name
	}
	slices.SortStableFunc(teams, func(a, b CheckInTeam) int { return c.CompareString(first(a), first(b)) })
}
