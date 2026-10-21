package tournament

import (
	"fmt"
	"slices"
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

// ImportSkip is a line left out of an import, with why.
type ImportSkip struct {
	Line int         `json:"line"`
	Code engine.Code `json:"code"`
}

// ImportReport says what an import did: entrants added, lines skipped
// because they cannot be entered, lines ignored on purpose (their category
// value is mapped to none).
type ImportReport struct {
	Added   int          `json:"added"`
	Skipped []ImportSkip `json:"skipped"`
	Ignored int          `json:"ignored"`
}

// ImportValue is a value of the category column, with the category its
// lines go to ("" for none) and how many lines hold it.
type ImportValue struct {
	Value    string          `json:"value"`
	Category engine.Category `json:"category"`
	Lines    int             `json:"lines"`
}

// ImportPreview is what an import would do, nothing saved: the report, the
// entrants it would add, and the values of the category column.
type ImportPreview struct {
	Report   ImportReport        `json:"report"`
	Entrants []EntrantRegistered `json:"entrants"`
	Values   []ImportValue       `json:"values"`
}

// ImportEntrants enters the lines of a player list, read with a mapping.
// Lines that cannot be entered, or whose players are already entered
// together in that category, are skipped and reported: the rest is
// imported.
func (s State) ImportEntrants(lines []ImportLine, m ImportMapping) (Change, ImportReport, error) {
	p := s.PreviewImport(lines, m)
	if len(p.Entrants) == 0 {
		return Change{}, p.Report, nil
	}
	return Change{TypeEntrantsImported, EntrantsImported{Entrants: p.Entrants}}, p.Report, nil
}

// PreviewImport is what ImportEntrants would do.
func (s State) PreviewImport(lines []ImportLine, m ImportMapping) ImportPreview {
	p := ImportPreview{Report: ImportReport{Skipped: []ImportSkip{}}, Entrants: []EntrantRegistered{}, Values: []ImportValue{}}
	seq := s.nextSeq()
	known := map[string]bool{}
	for _, e := range s.Entrants {
		known[entrantKey(e.Category, e.Players)] = true
	}
	values := map[string]int{}
	for _, row := range m.rows(lines) {
		if m.Category >= 0 && row.Value != "" {
			i, ok := values[row.Value]
			if !ok {
				i = len(p.Values)
				values[row.Value] = i
				p.Values = append(p.Values, ImportValue{Value: row.Value, Category: row.Category})
			}
			p.Values[i].Lines++
		}
		if row.Ignored {
			p.Report.Ignored++
			continue
		}
		skip := func(code engine.Code) {
			p.Report.Skipped = append(p.Report.Skipped, ImportSkip{row.Line, code})
		}
		if !s.HasCategory(row.Category) {
			skip(engine.CodeCategoryNotFound)
			continue
		}
		players, err := normalizePlayers(row.Category, row.Players)
		if err != nil {
			skip(err.(*engine.Error).Code)
			continue
		}
		k := entrantKey(row.Category, players)
		if known[k] {
			skip(engine.CodeAlreadyEntered)
			continue
		}
		known[k] = true
		n := len(p.Entrants) + 1
		for i := range players {
			players[i].ID = PlayerID(fmt.Sprintf("p%d.%d.%d", seq, n, i+1))
		}
		p.Entrants = append(p.Entrants, EntrantRegistered{
			ID: engine.EntrantID(fmt.Sprintf("e%d.%d", seq, n)), Category: row.Category, Players: players,
		})
	}
	p.Report.Added = len(p.Entrants)
	return p
}

// entrantKey identifies an entrant by category and player names, in any
// order: a pair registered by each of its players is entered once.
func entrantKey(c engine.Category, players []Player) string {
	names := make([]string, len(players))
	for i, p := range players {
		names[i] = strings.ToLower(p.Name)
	}
	slices.Sort(names)
	return string(c) + "|" + strings.Join(names, "|")
}

func (s *State) applyEntrantsImported(p EntrantsImported) {
	for _, e := range p.Entrants {
		s.applyEntrantRegistered(e)
	}
}
