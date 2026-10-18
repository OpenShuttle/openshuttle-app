package tournament

import (
	"encoding/json"

	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

// Duplicate returns the changes that start a new tournament named name with
// the setup of src: courts, rest, court priority, categories and formats.
// Entrants, draws and results are not copied.
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
		if src.formatSet[c] {
			if err := apply(st.ApplyFormat(c, src.StageConfigs(c))); err != nil {
				return nil, err
			}
		}
	}
	if err := apply(st.SetCourtPriority(src.CourtPriority())); err != nil {
		return nil, err
	}
	return out, nil
}
