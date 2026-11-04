package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// previewOf is the preview of a format category for one base.
func previewOf(fc FormatCategory, b PreviewBase) FormatPreviewView {
	for _, p := range fc.Previews {
		if p.Base == b {
			return p
		}
	}
	return FormatPreviewView{}
}

// reg is the preview of a format category for every registered entrant.
func reg(fc FormatCategory) FormatPreviewView { return previewOf(fc, PreviewRegistered) }

func TestFormatPreviewBases(t *testing.T) {
	// Men's doubles: 5 pairs (2 checked in) and 3 players waiting for a
	// partner. Women's singles: 4 players, 1 checked in.
	f := newFixture(t)
	ctx := f.ctx
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "b.openshuttle"), "Plume"))
	must(f.svc.AddCategory(ctx, engine.MensDoubles))
	must(f.svc.AddCategory(ctx, engine.WomensSingles))
	for i := range 5 {
		must(f.svc.RegisterEntrant(ctx, engine.MensDoubles, []tournament.PlayerInput{{Name: fmt.Sprint("A ", i)}, {Name: fmt.Sprint("B ", i)}}))
	}
	for i := range 3 {
		must(f.svc.RegisterEntrant(ctx, engine.MensDoubles, []tournament.PlayerInput{{Name: fmt.Sprint("W ", i)}}))
	}
	for i := range 4 {
		must(f.svc.RegisterEntrant(ctx, engine.WomensSingles, []tournament.PlayerInput{{Name: fmt.Sprint("S ", i)}}))
	}
	ev := f.svc.GetEntrantsView()
	must(f.svc.CheckInEntrant(ctx, ev.Categories[0].Entrants[0].ID))
	must(f.svc.CheckInEntrant(ctx, ev.Categories[0].Entrants[1].ID))
	must(f.svc.CheckInEntrant(ctx, ev.Categories[0].Entrants[5].ID)) // waiting: never present
	must(f.svc.CheckInEntrant(ctx, ev.Categories[1].Entrants[0].ID))

	type base struct {
		base     PreviewBase
		entrants int
	}
	tests := []struct {
		name  string
		cat   int
		bases []base
	}{
		{"doubles", 0, []base{{PreviewPaired, 5}, {PreviewRegistered, 8}, {PreviewPresent, 2}}},
		{"singles: no partner to wait for", 1, []base{{PreviewRegistered, 4}, {PreviewPresent, 1}}},
	}
	v := f.svc.GetFormatView()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := v.Categories[tt.cat]
			var got []base
			for _, p := range fc.Previews {
				got = append(got, base{p.Base, p.Entrants})
				if p.Stages[0].Entrants != p.Entrants {
					t.Errorf("%s: entry stage previews %d entrants, want %d", p.Base, p.Stages[0].Entrants, p.Entrants)
				}
			}
			if !reflect.DeepEqual(got, tt.bases) {
				t.Errorf("bases = %+v, want %+v", got, tt.bases)
			}
		})
	}
}
