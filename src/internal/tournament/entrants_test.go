package tournament

import (
	"slices"
	"strings"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestCategories(t *testing.T) {
	g := newGame(t)
	g.do(func(s State) (Change, error) { return s.AddCategory(engine.MensDoubles) })
	g.fails(engine.CodeCategoryExists, func(s State) (Change, error) { return s.AddCategory(engine.MensDoubles) })
	g.fails(engine.CodeInvalidCategory, func(s State) (Change, error) { return s.AddCategory("DH") })
	g.do(func(s State) (Change, error) { return s.AddCategory(engine.WomensDoubles) })
	g.fails(engine.CodeTooManyCategories, func(s State) (Change, error) { return s.AddCategory(engine.MixedDoubles) })

	if got := g.state().Categories; !slices.Equal(got, []engine.Category{"MD", "WD"}) {
		t.Fatalf("categories = %v", got)
	}

	g.register(engine.WomensDoubles, "Léa", "Inès")
	g.fails(engine.CodeCategoryNotEmpty, func(s State) (Change, error) { return s.RemoveCategory(engine.WomensDoubles) })
	g.fails(engine.CodeCategoryNotFound, func(s State) (Change, error) { return s.RemoveCategory(engine.MensSingles) })
	g.do(func(s State) (Change, error) { return s.RemoveCategory(engine.MensDoubles) })
	if got := g.state().Categories; !slices.Equal(got, []engine.Category{"WD"}) {
		t.Errorf("categories after remove = %v", got)
	}
	g.undo()
	if got := g.state().Categories; !slices.Equal(got, []engine.Category{"MD", "WD"}) {
		t.Errorf("categories after undo = %v", got)
	}
}

func TestRegisterEntrant(t *testing.T) {
	g := newGame(t, engine.MensSingles, engine.MixedDoubles)

	tests := []struct {
		name     string
		category engine.Category
		players  []PlayerInput
		code     engine.Code
		set      int
	}{
		{"singles", engine.MensSingles, []PlayerInput{{"Paul", "Durand", "BC Lyon"}}, "", 0},
		{"pair", engine.MixedDoubles, []PlayerInput{{"Paul", "Durand", ""}, {"Léa", "Roux", "ASPTT"}}, "", 0},
		{"pair in singles", engine.MensSingles, []PlayerInput{{"A", "B", ""}, {"C", "D", ""}}, engine.CodeWrongPlayerCount, 0},
		{"one player in doubles", engine.MixedDoubles, []PlayerInput{{"A", "B", ""}}, engine.CodeWrongPlayerCount, 0},
		{"category not played", engine.WomensSingles, []PlayerInput{{"A", "B", ""}}, engine.CodeCategoryNotFound, 0},
		{"missing last name", engine.MixedDoubles, []PlayerInput{{"A", "B", ""}, {"Léa", "  ", ""}}, engine.CodeInvalidPlayerName, 2},
		{"name too long", engine.MensSingles, []PlayerInput{{strings.Repeat("a", 51), "B", ""}}, engine.CodeInvalidPlayerName, 1},
		{"club too long", engine.MensSingles, []PlayerInput{{"A", "B", strings.Repeat("c", 81)}}, engine.CodeInvalidClub, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := g.state().RegisterEntrant(tt.category, tt.players)
			if tt.code == "" {
				if err != nil || ch.Type != TypeEntrantRegistered {
					t.Fatalf("RegisterEntrant() = %+v, %v", ch, err)
				}
				return
			}
			e, ok := err.(*engine.Error)
			if !ok || e.Code != tt.code || e.Set != tt.set {
				t.Fatalf("error = %v (%+v), want %s set %d", err, e, tt.code, tt.set)
			}
		})
	}
}

func TestEntrantLifecycle(t *testing.T) {
	g := newGame(t, engine.MixedDoubles)
	id := g.do(func(s State) (Change, error) {
		return s.RegisterEntrant(engine.MixedDoubles, []PlayerInput{
			{"  Paul ", " Durand", "BC  Lyon"}, {"Léa", "Roux", ""},
		})
	}).Data.(EntrantRegistered).ID

	e := g.entrant(id)
	// seq 1 is the creation, 2 the category: the entrant is the third event.
	if id != "e3" || e.Players[0].ID != "p3.1" || e.Players[1].ID != "p3.2" {
		t.Fatalf("ids = %s %s %s", id, e.Players[0].ID, e.Players[1].ID)
	}
	if e.Players[0].FirstName != "Paul" || e.Players[0].LastName != "Durand" || e.Players[0].Club != "BC Lyon" {
		t.Errorf("player not normalized: %+v", e.Players[0])
	}

	// Update keeps IDs and check-in.
	g.do(func(s State) (Change, error) { return s.CheckInPlayer("p3.2") })
	g.do(func(s State) (Change, error) {
		return s.UpdateEntrant(id, []PlayerInput{{"Paul", "Durand", "BC Lyon"}, {"Léa", "Roux-Petit", "ASPTT"}})
	})
	e = g.entrant(id)
	if e.Players[1].ID != "p3.2" || e.Players[1].LastName != "Roux-Petit" || !e.Players[1].CheckedIn {
		t.Errorf("after update = %+v", e.Players[1])
	}
	// Same values: nothing to append.
	if ch := g.do(func(s State) (Change, error) {
		return s.UpdateEntrant(id, []PlayerInput{{"Paul", "Durand", "BC Lyon"}, {"Léa", "Roux-Petit", "ASPTT"}})
	}); !ch.None() {
		t.Errorf("unchanged update produced %+v", ch)
	}

	g.do(func(s State) (Change, error) { return s.RemoveEntrant(id) })
	if len(g.state().Entrants) != 0 {
		t.Fatal("entrant not removed")
	}
	g.fails(engine.CodeEntrantNotFound, func(s State) (Change, error) { return s.RemoveEntrant(id) })
	g.undo()
	if e = g.entrant(id); e.Bib != 1 || !e.Players[1].CheckedIn {
		t.Errorf("after undo = %+v", e)
	}
}

func TestDuplicatePlayers(t *testing.T) {
	g := newGame(t, engine.MensSingles, engine.WomensSingles)
	a := g.register(engine.MensSingles, "Camille")
	b := g.register(engine.WomensSingles, "camille")
	g.register(engine.WomensSingles, "Léa")
	dup := g.state().DuplicatePlayers()
	if len(dup) != 2 || !dup[g.entrant(a).Players[0].ID] || !dup[g.entrant(b).Players[0].ID] {
		t.Errorf("duplicates = %v", dup)
	}
}
