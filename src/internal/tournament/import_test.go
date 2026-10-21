package tournament

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestReadImportTable(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []ImportLine
	}{
		{"french export, BOM, CRLF, blank line", "\uFEFFCatégorie;Joueur 1\r\nDH;Paul  Zola\r\n\r\n;Léa\r\n",
			[]ImportLine{{1, []string{"Catégorie", "Joueur 1"}}, {2, []string{"DH", "Paul Zola"}}, {4, []string{"", "Léa"}}}},
		{"commas", "ws,Léa Roux,\n",
			[]ImportLine{{1, []string{"ws", "Léa Roux", ""}}}},
		{"tabs", "Nom\tPrénom\tClub, ville\nRoux\tLéa\tASPTT\n",
			[]ImportLine{{1, []string{"Nom", "Prénom", "Club, ville"}}, {2, []string{"Roux", "Léa", "ASPTT"}}}},
		{"quoted separator and line break", "MD;\"Durand; Paul\";\"BC \"\"8\"\"\nLyon\"\nMD;Roux\n",
			[]ImportLine{{1, []string{"MD", "Durand; Paul", `BC "8" Lyon`}}, {3, []string{"MD", "Roux"}}}},
		{"Windows-1252 from Excel", "Pr\xe9nom;Club\nZo\xe9;\x80 Plume\n",
			[]ImportLine{{1, []string{"Prénom", "Club"}}, {2, []string{"Zoé", "€ Plume"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadImportTable([]byte(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("lines = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGuessCategory(t *testing.T) {
	tests := []struct {
		value string
		want  engine.Category
	}{
		{"DH", engine.MensDoubles},
		{"xd", engine.MixedDoubles},
		{"Double Hommes - 12 €", engine.MensDoubles},
		{"Double dames", engine.WomensDoubles},
		{"Women's doubles", engine.WomensDoubles},
		{"Men's singles", engine.MensSingles},
		{"Simple féminin", engine.WomensSingles},
		{"Double mixte", engine.MixedDoubles},
		{"Repas du midi", ""},
		{"Double", ""},
	}
	for _, tt := range tests {
		if got := GuessCategory(tt.value); got != tt.want {
			t.Errorf("GuessCategory(%q) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestGuessImportMapping(t *testing.T) {
	p := func(name, first, last, club int) ImportPlayerColumns {
		return ImportPlayerColumns{Name: name, First: first, Last: last, Club: club}
	}
	const no = NoColumn
	tests := []struct {
		name     string
		header   []string
		want     []ImportPlayerColumns
		category int
		hasHead  bool
	}{
		{"french export", []string{"Catégorie", "Joueur 1", "Club 1", "Joueur 2", "Club 2"},
			[]ImportPlayerColumns{p(1, no, no, 2), p(3, no, no, 4)}, 0, true},
		{"english export", []string{"Category", "Player 1", "Club 1", "Player 2", "Club 2"},
			[]ImportPlayerColumns{p(1, no, no, 2), p(3, no, no, 4)}, 0, true},
		{"export without header", []string{"DH", "Paul Zola", "BC Lyon", "Hugo Écuyer", "BC Lyon"},
			[]ImportPlayerColumns{p(1, no, no, 2), p(3, no, no, 4)}, 0, false},
		{"HelloAsso", []string{"Numéro de commande", "Nom payeur", "Prénom payeur", "Nom participant", "Prénom participant", "Tarif", "Email", "Club", "Nom et prénom du partenaire", "Club du partenaire"},
			[]ImportPlayerColumns{p(no, 4, 3, 7), p(8, no, no, 9)}, 5, true},
		{"partner as a last name column", []string{"Prénom", "Nom", "Nom du partenaire", "Catégorie"},
			[]ImportPlayerColumns{p(no, 0, 1, no), p(2, no, no, no)}, 3, true},
		{"singles", []string{"First name", "Last name", "Club"},
			[]ImportPlayerColumns{p(no, 0, 1, 2)}, no, true},
		{"nothing known", []string{"A", "B", "C"},
			[]ImportPlayerColumns{p(1, no, no, 2)}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := GuessImportMapping([]ImportLine{{1, tt.header}}, engine.MensDoubles)
			if !reflect.DeepEqual(m.Players, tt.want) || m.Category != tt.category || m.Header != tt.hasHead || m.Fallback != engine.MensDoubles {
				t.Errorf("mapping = %+v", m)
			}
		})
	}
}

func TestImportEntrants(t *testing.T) {
	g := newGame(t, engine.MensDoubles, engine.WomensSingles)
	g.do(func(s State) (Change, error) {
		return s.RegisterEntrant(engine.WomensSingles, []PlayerInput{{"Léa Roux", ""}})
	})
	lines := []ImportLine{
		{1, []string{"Tableau", "Prénom", "Nom", "Club", "Partenaire"}},
		{2, []string{"DH", "Paul", "Zola", "BC Lyon", "Hugo Écuyer"}},
		{3, []string{"", "Zoé", "Petit", "ASPTT"}},
		{4, []string{"Simple dames", "léa", "roux"}},                      // already entered
		{5, []string{"Double hommes", "Hugo", "Écuyer", "", "Paul Zola"}}, // the same pair, registered by the partner
		{6, []string{"XD", "A", "B", "", "C"}},                            // not played
		{7, []string{"MD", "Solo", "", "", ""}},                           // pair expected
		{8, []string{"Tournoi", "X", "Y"}},                                // unknown category
		{9, []string{"Repas", "Jo", "Durand"}},                            // ignored on purpose
		{10, []string{"Double hommes", "Tom", "Petit", "AS Bron", "Max Roux"}},
	}
	m := ImportMapping{
		Header: true, Category: 0, Fallback: engine.WomensSingles,
		Categories: map[string]engine.Category{"Repas": ""},
		Players:    []ImportPlayerColumns{{NoColumn, 1, 2, 3}, {4, NoColumn, NoColumn, SameClub}},
	}
	preview := g.state().PreviewImport(lines, m)
	wantValues := []ImportValue{
		{"DH", engine.MensDoubles, 1}, {"Simple dames", engine.WomensSingles, 1}, {"Double hommes", engine.MensDoubles, 2},
		{"XD", engine.MixedDoubles, 1}, {"MD", engine.MensDoubles, 1}, {"Tournoi", "", 1}, {"Repas", "", 1},
	}
	if !reflect.DeepEqual(preview.Values, wantValues) {
		t.Errorf("values = %+v", preview.Values)
	}
	var report ImportReport
	ch := g.do(func(s State) (Change, error) {
		ch, r, err := s.ImportEntrants(lines, m)
		report = r
		return ch, err
	})
	want := []ImportSkip{
		{4, engine.CodeAlreadyEntered}, {5, engine.CodeAlreadyEntered}, {6, engine.CodeCategoryNotFound},
		{7, engine.CodeWrongPlayerCount}, {8, engine.CodeCategoryNotFound},
	}
	if report.Added != 3 || report.Ignored != 1 || !slices.Equal(report.Skipped, want) || !reflect.DeepEqual(report, preview.Report) {
		t.Errorf("report = %+v", report)
	}
	imported := ch.Data.(EntrantsImported).Entrants
	if imported[0].ID != "e5.1" || imported[0].Players[1].ID != "p5.1.2" || imported[0].Players[1].Club != "BC Lyon" ||
		imported[0].Players[0].Name != "Paul Zola" || imported[1].Category != engine.WomensSingles || imported[2].Players[1].Club != "AS Bron" {
		t.Errorf("imported = %+v", imported)
	}
	if n := len(g.state().Entrants); n != 4 {
		t.Errorf("entrants = %d", n)
	}
	// Importing the same lines again adds nobody.
	_, again, _ := g.state().ImportEntrants(lines[:3], m)
	if again.Added != 0 || len(again.Skipped) != 2 {
		t.Errorf("second import = %+v", again)
	}
}
