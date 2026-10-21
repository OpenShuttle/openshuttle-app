package tournament

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// ImportLine is a non-blank line of a player list: its number in the file
// and its cells, spaces collapsed.
type ImportLine struct {
	Line  int      `json:"line"`
	Cells []string `json:"cells"`
}

// ReadImportTable reads a CSV player list, whatever tool wrote it: UTF-8
// with or without BOM, or Windows-1252 (Excel); ";", "," or tab separated,
// the one the first line uses most. Blank lines are left out.
func ReadImportTable(data []byte) ([]ImportLine, error) {
	data = bytes.TrimPrefix(data, []byte("\uFEFF"))
	if !utf8.Valid(data) {
		data = decodeWindows1252(data)
	}
	first, _, _ := bytes.Cut(data, []byte("\n"))
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = ','
	best := bytes.Count(first, []byte(","))
	for _, sep := range []rune{';', '\t'} {
		if n := bytes.Count(first, []byte(string(sep))); n > best {
			r.Comma, best = sep, n
		}
	}
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var lines []ImportLine
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, engine.NewError(engine.CodeInvalidFile)
		}
		line, _ := r.FieldPos(0)
		for j := range rec {
			rec[j] = collapseSpaces(rec[j])
		}
		if strings.Join(rec, "") == "" {
			continue
		}
		lines = append(lines, ImportLine{Line: line, Cells: rec})
		if len(lines) > MaxImportRows+1 {
			return nil, engine.NewError(engine.CodeInvalidFile)
		}
	}
	return lines, nil
}

// windows1252 maps the bytes 0x80 to 0x9F of Windows-1252; the others are
// Latin-1.
var windows1252 = [32]rune{
	'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F,
	0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ',
}

func decodeWindows1252(data []byte) []byte {
	var b strings.Builder
	for _, c := range data {
		if c >= 0x80 && c < 0xA0 {
			b.WriteRune(windows1252[c-0x80])
		} else {
			b.WriteRune(rune(c))
		}
	}
	return []byte(b.String())
}

// NoColumn is a column index meaning none.
const NoColumn = -1

// SameClub, as the club column of a partner, gives it the club of the
// first player.
const SameClub = -2

// ImportPlayerColumns says where a player is in a line: a full name column,
// or first and last name columns, and a club column. Indexes from 0;
// NoColumn for none.
type ImportPlayerColumns struct {
	Name  int `json:"name"`
	First int `json:"first"`
	Last  int `json:"last"`
	Club  int `json:"club"`
}

// ImportMapping says what the columns of a player list hold.
//
// The category of a line comes from its value in the Category column:
// Categories maps a value to a category ("" leaves the line out on
// purpose); other values are guessed (codes such as "DH", words such as
// "Double hommes"). Without category column or value, Fallback.
type ImportMapping struct {
	Header     bool                       `json:"header"`
	Category   int                        `json:"category"`
	Categories map[string]engine.Category `json:"categories"`
	Fallback   engine.Category            `json:"fallback"`
	Players    []ImportPlayerColumns      `json:"players"`
}

// importRow is a line read with a mapping.
type importRow struct {
	Line     int
	Value    string // in the category column
	Category engine.Category
	Ignored  bool
	Players  []PlayerInput
}

func (m ImportMapping) rows(lines []ImportLine) []importRow {
	if m.Header && len(lines) > 0 {
		lines = lines[1:]
	}
	rows := make([]importRow, 0, len(lines))
	for _, l := range lines {
		cell := func(i int) string {
			if i < 0 || i >= len(l.Cells) {
				return ""
			}
			return l.Cells[i]
		}
		row := importRow{Line: l.Line, Value: cell(m.Category), Category: m.Fallback}
		if row.Value != "" {
			c, ok := m.Categories[row.Value]
			if !ok {
				c = GuessCategory(row.Value)
			}
			row.Category, row.Ignored = c, ok && c == ""
		}
		for k, pc := range m.Players {
			name := cell(pc.Name)
			if pc.Name < 0 {
				name = collapseSpaces(cell(pc.First) + " " + cell(pc.Last))
			}
			club := cell(pc.Club)
			if pc.Club == SameClub && k > 0 && len(row.Players) > 0 {
				club = row.Players[0].Club
			}
			if name != "" || (club != "" && pc.Club != SameClub) {
				row.Players = append(row.Players, PlayerInput{Name: name, Club: club})
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// categoryCodes reads short category codes, in English or French.
var categoryCodes = map[string]engine.Category{
	"MS": engine.MensSingles, "WS": engine.WomensSingles, "MD": engine.MensDoubles,
	"WD": engine.WomensDoubles, "XD": engine.MixedDoubles,
	"SH": engine.MensSingles, "SD": engine.WomensSingles, "DH": engine.MensDoubles,
	"DD": engine.WomensDoubles, "DM": engine.MixedDoubles,
}

// GuessCategory reads a category from a value of a registration tool: a
// short code ("DH", "MD"), or words ("Double hommes", "Women's singles",
// "Mixte - 12 €"). "" when it says none.
func GuessCategory(value string) engine.Category {
	if c, ok := categoryCodes[strings.ToUpper(strings.TrimSpace(value))]; ok {
		return c
	}
	has := wordsOf(value).has
	switch {
	case has("mixte", "mixtes", "mixed", "mix", "dm", "xd"):
		return engine.MixedDoubles
	case has("double", "doubles"):
		if has("dame", "dames", "femme", "femmes", "feminin", "women", "womens", "ladies", "dd", "wd") {
			return engine.WomensDoubles
		}
		if has("homme", "hommes", "masculin", "men", "mens", "dh", "md") {
			return engine.MensDoubles
		}
	case has("simple", "simples", "single", "singles"):
		if has("dame", "dames", "femme", "femmes", "feminin", "women", "womens", "ladies") {
			return engine.WomensSingles
		}
		if has("homme", "hommes", "masculin", "men", "mens") {
			return engine.MensSingles
		}
	}
	return ""
}

// words is the set of words of a text, simplified.
type words map[string]bool

func wordsOf(s string) words {
	w := words{}
	for _, word := range strings.Fields(simplify(s)) {
		w[word] = true
	}
	return w
}

// has reports whether one of ws is a word of the text.
func (w words) has(ws ...string) bool {
	for _, word := range ws {
		if w[word] {
			return true
		}
	}
	return false
}

// simplify lowercases a text and drops its accents and punctuation:
// "Prénom du partenaire" → "prenom du partenaire", "Women's" → "women s".
func simplify(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(strings.ReplaceAll(b.String(), "women s", "womens")), " ")
}

// GuessImportMapping guesses what the columns of a player list hold from
// its first line. A first line starting with a category code is data in
// the layout of the entrants export: category, player 1, club 1, player 2,
// club 2. Otherwise it names the columns (OpenShuttle's export, HelloAsso
// and other registration tools): the guess goes by words such as
// "catégorie", "prénom", "nom", "club", "partenaire" or "2".
func GuessImportMapping(lines []ImportLine, fallback engine.Category) ImportMapping {
	m := ImportMapping{Category: NoColumn, Categories: map[string]engine.Category{}, Fallback: fallback}
	player := func() ImportPlayerColumns {
		return ImportPlayerColumns{Name: NoColumn, First: NoColumn, Last: NoColumn, Club: NoColumn}
	}
	exportLayout := func(width int) {
		m.Category = 0
		m.Players = nil
		for k := 1; k < width || len(m.Players) == 0; k += 2 {
			p := player()
			p.Name = k
			if k+1 < width {
				p.Club = k + 1
			}
			m.Players = append(m.Players, p)
		}
	}
	if len(lines) == 0 {
		m.Players = []ImportPlayerColumns{player()}
		return m
	}
	header := lines[0].Cells
	if _, ok := categoryCodes[strings.ToUpper(header[0])]; ok {
		exportLayout(len(header))
		return m
	}
	m.Header = true
	players := []ImportPlayerColumns{player(), player()}
	weakCategory := NoColumn
	set := func(at *int, i int) {
		if *at == NoColumn {
			*at = i
		}
	}
	for i, name := range header {
		has := wordsOf(name).has
		if has("payeur", "payer", "acheteur", "buyer", "email", "mail", "date", "numero", "number", "montant", "amount", "code", "statut", "status", "telephone", "phone", "licence", "license") {
			continue
		}
		p := &players[0]
		if has("partenaire", "partner", "coequipier", "binome", "2") {
			p = &players[1]
		}
		switch {
		case has("club"):
			set(&p.Club, i)
		case has("categorie", "categories", "category", "tableau", "epreuve", "serie"):
			set(&m.Category, i)
		case has("tarif", "billet", "formule", "ticket", "rate"):
			set(&weakCategory, i)
		case has("prenom") && has("nom"), has("nom", "name") && has("complet", "full"):
			set(&p.Name, i)
		case has("prenom", "firstname") || has("first") && has("name"):
			set(&p.First, i)
		case has("nom", "lastname", "surname") || has("last") && has("name"):
			set(&p.Last, i)
		case has("joueur", "joueuse", "player", "name", "participant", "partenaire", "partner", "coequipier", "binome"):
			set(&p.Name, i)
		}
	}
	if m.Category == NoColumn {
		m.Category = weakCategory
	}
	for k := range players {
		p := &players[k]
		if p.Name == NoColumn && p.First == NoColumn && p.Last != NoColumn {
			p.Name, p.Last = p.Last, NoColumn // "Nom du partenaire": the whole name
		}
		if p.Name != NoColumn {
			p.First, p.Last = NoColumn, NoColumn
		}
	}
	found := func(p ImportPlayerColumns) bool {
		return p.Name != NoColumn || p.First != NoColumn || p.Last != NoColumn
	}
	switch {
	case !found(players[0]):
		exportLayout(len(header))
	case found(players[1]):
		m.Players = players
	default:
		m.Players = players[:1]
	}
	return m
}
