package tournament

import (
	"encoding/json"

	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// Duplicate returns the changes that start a new tournament named name with
// the setup of src: courts, rest, court priority, categories, pool stages,
// brackets and routing. Entrants, draws and results are not copied.
func Duplicate(src State, name string) ([]Change, error) {
	name, err := NormalizeName(name)
	if err != nil {
		return nil, err
	}
	var (
		log []events.Event
		st  State
		out []Change
	)
	apply := func(ch Change, err error) error {
		if err != nil || ch.None() {
			return err
		}
		raw, err := json.Marshal(ch.Data)
		if err != nil {
			return err
		}
		log = append(log, events.Event{Seq: uint64(len(log) + 1), Type: ch.Type, Data: raw})
		if st, err = Project(log); err != nil {
			return err
		}
		out = append(out, ch)
		return nil
	}

	rest := src.MinRest()
	created := Created{Name: name, Courts: src.Courts(), MinRest: &rest, TimedPools: src.defaultTimed}
	if err := apply(Change{TypeCreated, created}, nil); err != nil {
		return nil, err
	}
	for _, c := range src.Categories {
		if err := apply(st.AddCategory(c)); err != nil {
			return nil, err
		}
		if stage, ok := src.stages[c]; ok {
			if err := apply(st.ConfigurePoolStage(c, stage)); err != nil {
				return nil, err
			}
		}
		// Bracket IDs restart from T1: map the old ones for the routing.
		ids := map[string]string{"": ""}
		for _, b := range src.Brackets(c) {
			if err := apply(st.AddBracket(c, b.BracketConfig)); err != nil {
				return nil, err
			}
			added := st.Brackets(c)
			ids[b.ID] = added[len(added)-1].ID
		}
		routing := src.Routing(c)
		for i, id := range routing {
			routing[i] = ids[id]
		}
		if err := apply(st.SetRouting(c, routing)); err != nil {
			return nil, err
		}
	}
	if err := apply(st.SetCourtPriority(src.CourtPriority())); err != nil {
		return nil, err
	}
	return out, nil
}
