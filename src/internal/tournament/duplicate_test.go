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
	g.editStage(engine.MensDoubles, "S1", func(st *StageConfig) { st.Pools = 3 })
	// S2 is removed: the copy renumbers S1, S3 and S4 to S1, S2 and S3.
	for _, name := range []string{"Old", "Principal", "Consolante"} {
		g.addStage(engine.MensDoubles, StageBracket, name, false)
	}
	g.do(func(s State) (Change, error) { return s.RemoveStage(engine.MensDoubles, "S2") })
	g.route(engine.MensDoubles, "S1", "S3", "S3", "S4", "")

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
	stages := got.StageConfigs(engine.MensDoubles)
	var names []string
	for _, st := range stages {
		names = append(names, st.ID+" "+st.Name)
	}
	if !slices.Equal(names, []string{"S1 ", "S2 Principal", "S3 Consolante"}) || stages[0].Pools != 3 {
		t.Errorf("stages = %v", names)
	}
	if r := stages[0].Routing; !slices.Equal(r, []string{"S2", "S2", "S3"}) {
		t.Errorf("routing = %v", r)
	}

	if _, err := Duplicate(g.state(), "  "); err == nil {
		t.Error("blank name accepted")
	}
}
