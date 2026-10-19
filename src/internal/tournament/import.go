package tournament

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// TypeEntrantsImported enters many entrants at once, from a file: one event,
// so one undo cancels the whole import.
const TypeEntrantsImported events.Type = "EntrantsImported"

// MaxImportRows bounds an imported file.
const MaxImportRows = 1000

// EntrantsImported is the payload of TypeEntrantsImported. Entrant IDs are
// "e<seq>.<n>", player IDs "p<seq>.<n>.<k>": unique and stable on replay.
type EntrantsImported struct {
	Entrants []EntrantRegistered `json:"entrants"`
}

// ImportRow is a line of a player list: a category (may be empty) and its
// players. The file layout is the one the entrants export writes:
// category; player 1; club 1; player 2; club 2.
type ImportRow struct {
	Line     int
	Category string
	Players  []PlayerInput
}

// ImportSkip is a line left out of an import, with why.
type ImportSkip struct {
	Line int         `json:"line"`
	Code engine.Code `json:"code"`
}

// ImportReport says what an import did.
type ImportReport struct {
	Added   int          `json:"added"`
	Skipped []ImportSkip `json:"skipped"`
}

// categoryCodes reads the category column, in English or French.
var categoryCodes = map[string]engine.Category{
	"MS": engine.MensSingles, "WS": engine.WomensSingles, "MD": engine.MensDoubles,
	"WD": engine.WomensDoubles, "XD": engine.MixedDoubles,
	"SH": engine.MensSingles, "SD": engine.WomensSingles, "DH": engine.MensDoubles,
	"DD": engine.WomensDoubles, "DM": engine.MixedDoubles,
}

// ParseEntrantsCSV reads a player list: UTF-8 (BOM or not), ";" or ","
// separated. A first line that names no category is a header and is left
// out. Blank lines are ignored.
func ParseEntrantsCSV(data []byte) ([]ImportRow, error) {
	data = bytes.TrimPrefix(data, []byte("\uFEFF"))
	first, _, _ := bytes.Cut(data, []byte("\n"))
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = ','
	if bytes.Count(first, []byte(";")) >= bytes.Count(first, []byte(",")) {
		r.Comma = ';'
	}
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, engine.NewError(engine.CodeInvalidFile)
	}
	var rows []ImportRow
	for i, rec := range records {
		for j := range rec {
			rec[j] = collapseSpaces(rec[j])
		}
		if strings.Join(rec, "") == "" {
			continue
		}
		cat := strings.ToUpper(rec[0])
		if i == 0 && categoryCodes[cat] == "" && cat != "" {
			continue // header
		}
		row := ImportRow{Line: i + 1, Category: cat}
		for k := 1; k < len(rec); k += 2 {
			p := PlayerInput{Name: rec[k]}
			if k+1 < len(rec) {
				p.Club = rec[k+1]
			}
			if p.Name != "" || p.Club != "" {
				row.Players = append(row.Players, p)
			}
		}
		rows = append(rows, row)
		if len(rows) > MaxImportRows {
			return nil, engine.NewError(engine.CodeInvalidFile)
		}
	}
	return rows, nil
}

// ImportEntrants enters the rows of a player list. A row without category
// goes to the fallback one. Rows that cannot be entered, or whose players
// are already entered together in that category, are skipped and reported:
// the rest is imported.
func (s State) ImportEntrants(rows []ImportRow, fallback engine.Category) (Change, ImportReport, error) {
	report := ImportReport{Skipped: []ImportSkip{}}
	seq := s.nextSeq()
	known := map[string]bool{}
	key := func(c engine.Category, players []Player) string {
		names := make([]string, len(players))
		for i, p := range players {
			names[i] = strings.ToLower(p.Name)
		}
		return string(c) + "|" + strings.Join(names, "|")
	}
	for _, e := range s.Entrants {
		known[key(e.Category, e.Players)] = true
	}
	var added []EntrantRegistered
	for _, row := range rows {
		c := fallback
		if row.Category != "" {
			c = categoryCodes[row.Category]
		}
		if !s.HasCategory(c) {
			report.Skipped = append(report.Skipped, ImportSkip{row.Line, engine.CodeCategoryNotFound})
			continue
		}
		players, err := normalizePlayers(c, row.Players)
		if err != nil {
			report.Skipped = append(report.Skipped, ImportSkip{row.Line, err.(*engine.Error).Code})
			continue
		}
		k := key(c, players)
		if known[k] {
			report.Skipped = append(report.Skipped, ImportSkip{row.Line, engine.CodeAlreadyEntered})
			continue
		}
		known[k] = true
		n := len(added) + 1
		for i := range players {
			players[i].ID = PlayerID(fmt.Sprintf("p%d.%d.%d", seq, n, i+1))
		}
		added = append(added, EntrantRegistered{
			ID: engine.EntrantID(fmt.Sprintf("e%d.%d", seq, n)), Category: c, Players: players,
		})
	}
	report.Added = len(added)
	if len(added) == 0 {
		return Change{}, report, nil
	}
	return Change{TypeEntrantsImported, EntrantsImported{Entrants: added}}, report, nil
}

func (s *State) applyEntrantsImported(p EntrantsImported) {
	for _, e := range p.Entrants {
		s.applyEntrantRegistered(e)
	}
}
