package main

import (
	"path/filepath"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

func TestEntrantsAndCheckIn(t *testing.T) {
	f := newFixture(t)
	ctx := f.ctx
	if err := f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "t.openshuttle"), "Plume"); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.svc.AddCategory(ctx, engine.MensDoubles))
	must(f.svc.AddCategory(ctx, engine.WomensSingles))
	wantCode(t, f.svc.AddCategory(ctx, engine.MixedDoubles), engine.CodeTooManyCategories)

	must(f.svc.RegisterEntrant(ctx, engine.MensDoubles, []tournament.PlayerInput{
		{Name: "Paul Zola", Club: "BC Lyon"},
		{Name: "Hugo Écuyer", Club: "BC Lyon"},
	}))
	must(f.svc.RegisterEntrant(ctx, engine.WomensSingles, []tournament.PlayerInput{
		{Name: "Léa durand", Club: ""},
	}))

	v := f.svc.GetTournamentView()
	if len(v.Categories) != 2 || v.Categories[1] != (CategoryRef{engine.WomensSingles, 2}) {
		t.Fatalf("categories = %+v", v.Categories)
	}

	ev := f.svc.GetEntrantsView()
	md := ev.Categories[0]
	if md.Category != engine.MensDoubles || !md.Doubles || len(md.Entrants) != 1 || md.Players != 2 {
		t.Fatalf("MD = %+v", md)
	}
	pair := md.Entrants[0]

	// Hugo checks in: the pair gets bib 1 but is not complete yet.
	must(f.svc.CheckInPlayer(ctx, pair.Players[1].ID))
	ci := f.svc.GetCheckInView()
	if ci.Players != 3 || ci.Present != 1 {
		t.Fatalf("totals = %d/%d", ci.Present, ci.Players)
	}
	// Sorted by name, accents and case ignored: Hugo Écuyer, Léa durand, Paul Zola.
	var order []string
	for _, r := range ci.Rows {
		order = append(order, r.Name)
	}
	if order[0] != "Hugo Écuyer" || order[1] != "Léa durand" || order[2] != "Paul Zola" {
		t.Errorf("order = %v", order)
	}
	hugo := ci.Rows[0]
	if !hugo.CheckedIn || hugo.Bib != 1 || hugo.Partner == nil || hugo.Partner.Name != "Paul Zola" || hugo.Slot != 1 {
		t.Errorf("hugo row = %+v", hugo)
	}
	if ci.Rows[1].Partner != nil || ci.Rows[1].Slot != 2 {
		t.Errorf("single row = %+v", ci.Rows[0])
	}
	if c := ci.Categories[0]; c.PlayersPresent != 1 || c.EntrantsComplete != 0 {
		t.Errorf("MD totals = %+v", c)
	}

	// History names the player, and the undo names the undone action.
	if a := f.svc.GetTournamentView().LastAction; a.Type != "PlayerCheckedIn" || a.Params["name"] != "Hugo Écuyer" {
		t.Errorf("last action = %+v", a)
	}
	must(f.svc.Undo(ctx))
	a := f.svc.GetTournamentView().LastAction
	if a.Type != "ActionUndone" || a.Params["action"] != "PlayerCheckedIn" || a.Params["name"] != "Hugo Écuyer" {
		t.Errorf("undo action = %+v", a)
	}

	must(f.svc.RemoveEntrant(ctx, pair.ID))
	if a := f.svc.GetTournamentView().LastAction; a.Params["names"] != "Paul Zola / Hugo Écuyer" {
		t.Errorf("remove action = %+v", a)
	}
	wantCode(t, f.svc.CheckInPlayer(ctx, pair.Players[0].ID), engine.CodePlayerNotFound)
}
