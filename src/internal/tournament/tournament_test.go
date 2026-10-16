package tournament

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

var t0 = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

type logBuilder struct {
	t   *testing.T
	log []events.Event
}

func (b *logBuilder) add(typ events.Type, data any) {
	b.t.Helper()
	d, err := events.NewDraft(typ, data, t0, "Julie")
	if err != nil {
		b.t.Fatal(err)
	}
	b.log = append(b.log, events.Event{Seq: uint64(len(b.log) + 1), Type: d.Type, Data: d.Data, At: d.At})
}

func (b *logBuilder) undo() {
	b.t.Helper()
	d, err := events.UndoLast(b.log, Undoable, t0, "Julie")
	if err != nil {
		b.t.Fatal(err)
	}
	b.log = append(b.log, events.Event{Seq: uint64(len(b.log) + 1), Type: d.Type, Data: d.Data, At: d.At})
}

func TestProject(t *testing.T) {
	b := &logBuilder{t: t}
	b.add(TypeCreated, Created{Name: "Plume d'automne"})
	b.add(TypeRenamed, Renamed{Name: "Plume d'automne 2026"})
	b.add(TypeRenamed, Renamed{Name: "Nuit du volant"})

	s, err := Project(b.log)
	if err != nil || s.Name != "Nuit du volant" || s.Version != 3 {
		t.Fatalf("Project() = %+v, %v", s, err)
	}

	b.undo()
	s, _ = Project(b.log)
	if s.Name != "Plume d'automne 2026" || s.Version != 4 {
		t.Errorf("after undo = %+v", s)
	}

	b.undo()
	s, _ = Project(b.log)
	if s.Name != "Plume d'automne" {
		t.Errorf("after second undo = %+v", s)
	}

	// Only the creation is left: nothing more to undo.
	if _, err := events.UndoLast(b.log, Undoable, t0, ""); err == nil {
		t.Error("creation should not be undoable")
	}
}

func TestRename(t *testing.T) {
	s := State{Name: "Plume"}
	tests := []struct {
		name    string
		input   string
		want    string
		changed bool
		code    engine.Code
	}{
		{"new name", "Nuit du volant", "Nuit du volant", true, ""},
		{"spaces are normalized", "  Nuit   du\tvolant ", "Nuit du volant", true, ""},
		{"same name", " Plume ", "", false, ""},
		{"empty", "   ", "", false, engine.CodeInvalidName},
		{"too long", strings.Repeat("é", MaxNameLength+1), "", false, engine.CodeInvalidName},
		{"longest", strings.Repeat("é", MaxNameLength), strings.Repeat("é", MaxNameLength), true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed, err := Rename(s, tt.input)
			if tt.code != "" {
				var e *engine.Error
				if !errors.As(err, &e) || e.Code != tt.code {
					t.Fatalf("error = %v, want %s", err, tt.code)
				}
				return
			}
			if err != nil || changed != tt.changed || got.Name != tt.want {
				t.Errorf("Rename() = %q, %v, %v; want %q, %v", got.Name, changed, err, tt.want, tt.changed)
			}
		})
	}
}
