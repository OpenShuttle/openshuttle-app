package main

import (
	"slices"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// sortRows orders check-in rows by last name, then first name, the way a
// French or English reader expects (accents ignored, case-insensitive).
func sortRows(rows []CheckInRow) {
	c := collate.New(language.French, collate.IgnoreCase, collate.IgnoreDiacritics)
	slices.SortStableFunc(rows, func(a, b CheckInRow) int {
		if r := c.CompareString(a.LastName, b.LastName); r != 0 {
			return r
		}
		return c.CompareString(strings.TrimSpace(a.FirstName), strings.TrimSpace(b.FirstName))
	})
}
