package main

import (
	"slices"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// sortRows orders check-in rows by name, the way a French or English reader
// expects (accents ignored, case-insensitive).
func sortRows(rows []CheckInRow) {
	c := collate.New(language.French, collate.IgnoreCase, collate.IgnoreDiacritics)
	slices.SortStableFunc(rows, func(a, b CheckInRow) int {
		return c.CompareString(strings.TrimSpace(a.Name), strings.TrimSpace(b.Name))
	})
}
