package tournament

import (
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

func TestDuplicate(t *testing.T) {
	g := newGame(t, engine.MensDoubles, engine.WomensDoubles)
	g.register(engine.MensDoubles, "A", "B")
	g.do(func(s State) (Change, error) { return s.SetCourts(7) })
	g.do(func(s State) (Change, error) { return s.SetMinRest(12) })
	g.do(func(s State) (Change, error) { return s.SetCourtPriority("WD") })
	stage := DefaultPoolStage()
	stage.Pools = 3
	g.do(func(s State) (Change, error) { return s.ConfigurePoolStage(engine.MensDoubles, stage) })
	// T1 is removed: the copy renumbers T2 and T3 to T1 and T2.
	for _, name := range []string{"Old", "Principal", "Consolante"} {
		g.do(func(s State) (Change, error) {
			return s.AddBracket(engine.MensDoubles, BracketConfig{Name: name, Win: sets21})
		})
	}
	g.do(func(s State) (Change, error) { return s.RemoveBracket(engine.MensDoubles, "T1") })
	g.do(func(s State) (Change, error) { return s.SetRouting(engine.MensDoubles, []string{"T2", "T2", "T3", ""}) })

	changes, err := Duplicate(g.state(), "  Plume 2027 ")
	if err != nil {
		t.Fatal(err)
	}
	var log []events.Event
	for i, ch := range changes {
		d, err := events.NewDraft(ch.Type, ch.Data, t0, "")
		if err != nil {
			t.Fatal(err)
		}
		log = append(log, events.Event{Seq: uint64(i + 1), Type: d.Type, Data: d.Data, At: d.At})
	}
	got, err := Project(log)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Plume 2027" || got.Courts() != 7 || got.MinRest() != 12 || got.CourtPriority() != "WD" {
		t.Errorf("setup = %q %d %d %q", got.Name, got.Courts(), got.MinRest(), got.CourtPriority())
	}
	if !slices.Equal(got.Categories, []engine.Category{engine.MensDoubles, engine.WomensDoubles}) {
		t.Errorf("categories = %v", got.Categories)
	}
	if len(got.Entrants) != 0 {
		t.Errorf("entrants copied: %v", got.Entrants)
	}
	if got.PoolStage(engine.MensDoubles).Pools != 3 {
		t.Errorf("pools = %d", got.PoolStage(engine.MensDoubles).Pools)
	}
	var names []string
	for _, b := range got.Brackets(engine.MensDoubles) {
		names = append(names, b.ID+" "+b.Name)
	}
	if !slices.Equal(names, []string{"T1 Principal", "T2 Consolante"}) {
		t.Errorf("brackets = %v", names)
	}
	if r := got.Routing(engine.MensDoubles); !slices.Equal(r, []string{"T1", "T1", "T2"}) {
		t.Errorf("routing = %v", r)
	}

	if _, err := Duplicate(g.state(), "  "); err == nil {
		t.Error("blank name accepted")
	}
}
