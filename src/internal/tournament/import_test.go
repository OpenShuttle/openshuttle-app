package tournament

import (
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestParseEntrantsCSV(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []ImportRow
	}{
		{"french export, BOM and header", "\uFEFFCatégorie;Joueur 1;Club 1;Joueur 2;Club 2\r\nDH;Paul  Zola;BC Lyon;Hugo Écuyer;BC Lyon\r\n\r\n",
			[]ImportRow{{Line: 2, Category: "DH", Players: []PlayerInput{{"Paul Zola", "BC Lyon"}, {"Hugo Écuyer", "BC Lyon"}}}}},
		{"english, commas, no header", "ws,Léa Roux,\n,Zoé Petit,ASPTT\n",
			[]ImportRow{
				{Line: 1, Category: "WS", Players: []PlayerInput{{"Léa Roux", ""}}},
				{Line: 2, Category: "", Players: []PlayerInput{{"Zoé Petit", "ASPTT"}}},
			}},
		{"quoted separator", `MD;"Durand; Paul";"BC ""8""";Roux;`,
			[]ImportRow{{Line: 1, Category: "MD", Players: []PlayerInput{{"Durand; Paul", `BC "8"`}, {"Roux", ""}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEntrantsCSV([]byte(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(got, tt.want, func(a, b ImportRow) bool {
				return a.Line == b.Line && a.Category == b.Category && slices.Equal(a.Players, b.Players)
			}) {
				t.Errorf("rows = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestImportEntrants(t *testing.T) {
	g := newGame(t, engine.MensDoubles, engine.WomensSingles)
	g.do(func(s State) (Change, error) {
		return s.RegisterEntrant(engine.WomensSingles, []PlayerInput{{"Léa Roux", ""}})
	})
	rows := []ImportRow{
		{Line: 2, Category: "DH", Players: []PlayerInput{{"Paul Zola", "BC Lyon"}, {"Hugo Écuyer", "BC Lyon"}}},
		{Line: 3, Category: "", Players: []PlayerInput{{"Zoé Petit", "ASPTT"}}},
		{Line: 4, Category: "WS", Players: []PlayerInput{{"léa roux", ""}}},                       // already entered
		{Line: 5, Category: "MD", Players: []PlayerInput{{"Paul Zola", ""}, {"Hugo Écuyer", ""}}}, // twice in the file
		{Line: 6, Category: "XD", Players: []PlayerInput{{"A", ""}, {"B", ""}}},                   // not played
		{Line: 7, Category: "MD", Players: []PlayerInput{{"Solo", ""}}},                           // pair expected
		{Line: 8, Category: "??", Players: []PlayerInput{{"X", ""}}},                              // unknown category
	}
	var report ImportReport
	ch := g.do(func(s State) (Change, error) {
		ch, r, err := s.ImportEntrants(rows, engine.WomensSingles)
		report = r
		return ch, err
	})
	want := []ImportSkip{
		{4, engine.CodeAlreadyEntered}, {5, engine.CodeAlreadyEntered}, {6, engine.CodeCategoryNotFound},
		{7, engine.CodeWrongPlayerCount}, {8, engine.CodeCategoryNotFound},
	}
	if report.Added != 2 || !slices.Equal(report.Skipped, want) {
		t.Errorf("report = %+v", report)
	}
	imported := ch.Data.(EntrantsImported).Entrants
	if imported[0].ID != "e5.1" || imported[0].Players[1].ID != "p5.1.2" || imported[1].Category != engine.WomensSingles {
		t.Errorf("imported = %+v", imported)
	}
	if n := len(g.state().Entrants); n != 3 {
		t.Errorf("entrants = %d", n)
	}
	// Importing the same rows again adds nobody.
	_, again, _ := g.state().ImportEntrants(rows[:2], engine.WomensSingles)
	if again.Added != 0 || len(again.Skipped) != 2 {
		t.Errorf("second import = %+v", again)
	}
}
