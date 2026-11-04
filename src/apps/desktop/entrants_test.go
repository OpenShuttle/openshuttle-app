package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

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
	if md.Category != engine.MensDoubles || !md.Doubles || len(md.Entrants) != 1 || md.Players != 2 || md.Entrants[0].Locked {
		t.Fatalf("MD = %+v", md)
	}
	pair := md.Entrants[0]

	// Hugo checks in: the pair gets bib 1 but is not complete yet.
	must(f.svc.CheckInPlayer(ctx, pair.Players[1].ID))
	ci := f.svc.GetCheckInView()
	if ci.Players != 3 || ci.Present != 1 {
		t.Fatalf("totals = %d/%d", ci.Present, ci.Players)
	}
	// One row per team, sorted by its first name in order, accents and case
	// ignored: the pair (Hugo Écuyer), then Léa durand.
	if len(ci.Teams) != 2 {
		t.Fatalf("teams = %+v", ci.Teams)
	}
	pairRow, single := ci.Teams[0], ci.Teams[1]
	if pairRow.Bib != 1 || pairRow.Complete || len(pairRow.Players) != 2 || !pairRow.Players[1].CheckedIn ||
		pairRow.Players[0].Name != "Paul Zola" || pairRow.Slot != 1 {
		t.Errorf("pair row = %+v", pairRow)
	}
	if len(single.Players) != 1 || single.Players[0].Name != "Léa durand" || single.Slot != 2 {
		t.Errorf("single row = %+v", single)
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

func TestImportEntrantsFile(t *testing.T) {
	f := newFixture(t)
	f.dialogs.save = filepath.Join(f.dir, "import")
	if _, err := f.svc.CreateTournament(f.ctx, "Plume"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AddCategory(f.ctx, engine.MensDoubles); err != nil {
		t.Fatal(err)
	}
	// Cancelled: nothing.
	if file, err := f.svc.ChooseImportFile(f.ctx, engine.MensDoubles); err != nil || file.Path != "" {
		t.Fatalf("cancelled import = %+v, %v", file, err)
	}
	f.dialogs.openCSV = filepath.Join(f.dir, "list.csv")
	content := "Catégorie;Joueur 1;Club 1;Joueur 2;Club 2\nDH;Paul Zola;BC Lyon;Hugo Écuyer;BC Lyon\n;Tom Petit;AS Bron;Max Roux;AS Bron\nDD;Léa Roux;;Zoé Petit;\n"
	if err := os.WriteFile(f.dialogs.openCSV, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := f.svc.ChooseImportFile(f.ctx, engine.MensDoubles)
	if err != nil || len(file.Lines) != 4 || !file.Mapping.Header || file.Mapping.Category != 0 {
		t.Fatalf("file = %+v, %v", file, err)
	}
	preview, err := f.svc.PreviewImport(file.Path, file.Mapping)
	if err != nil || preview.Report.Added != 2 || len(f.svc.GetEntrantsView().Categories[0].Entrants) != 0 {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	r, err := f.svc.ImportEntrants(f.ctx, file.Path, file.Mapping)
	if err != nil || r.Added != 2 || len(r.Skipped) != 1 || r.Skipped[0].Line != 4 {
		t.Fatalf("import = %+v, %v", r, err)
	}
	if v := f.svc.GetTournamentView(); v.LastAction == nil || v.LastAction.Params["count"] != "2" {
		t.Errorf("last action = %+v", v.LastAction)
	}
	// One undo cancels the whole import.
	if err := f.svc.Undo(f.ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(f.svc.GetEntrantsView().Categories[0].Entrants); n != 0 {
		t.Errorf("entrants after undo = %d", n)
	}
}

func TestCheckInTeam(t *testing.T) {
	f := newFixture(t)
	ctx := f.ctx
	if err := f.svc.CreateTournamentAt(ctx, filepath.Join(f.dir, "t.openshuttle"), "Plume"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AddCategory(ctx, engine.MensDoubles); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RegisterEntrant(ctx, engine.MensDoubles, []tournament.PlayerInput{{Name: "Paul Zola"}, {Name: "Hugo Écuyer"}}); err != nil {
		t.Fatal(err)
	}
	team := f.svc.GetCheckInView().Teams[0]
	if err := f.svc.CheckInEntrant(ctx, team.EntrantID); err != nil {
		t.Fatal(err)
	}
	if v := f.svc.GetCheckInView(); !v.Teams[0].Complete || v.Present != 2 || v.Teams[0].Bib != 1 {
		t.Errorf("after team check-in = %+v", v.Teams[0])
	}
	if a := f.svc.GetTournamentView().LastAction; a.Type != "EntrantCheckedIn" || a.Params["names"] != "Paul Zola / Hugo Écuyer" {
		t.Errorf("last action = %+v", a)
	}
}

func TestWaitingPartners(t *testing.T) {
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
	must(f.svc.AddCategory(ctx, engine.WomensDoubles))
	for _, name := range []string{"Julie Morel", "Zoé Petit", "Ana Bel"} {
		must(f.svc.RegisterEntrant(ctx, engine.WomensDoubles, []tournament.PlayerInput{{Name: name, Club: "PLVPB"}}))
	}
	wd := f.svc.GetEntrantsView().Categories[0]
	if wd.Waiting != 3 || !wd.Entrants[0].Waiting || wd.Entrants[0].Complete {
		t.Fatalf("WD = %+v", wd)
	}
	julie, zoe, ana := wd.Entrants[0], wd.Entrants[1], wd.Entrants[2]
	must(f.svc.CheckInEntrant(ctx, julie.ID))
	// Checked in: her row goes after the others (Ana, Zoé).
	if team := f.svc.GetCheckInView().Teams[2]; !team.Waiting || team.Bib != 0 || !team.Players[0].CheckedIn {
		t.Errorf("check-in row = %+v", team)
	}
	if d := f.svc.GetDrawView().Categories[0]; d.Waiting != 3 || d.Present != 0 {
		t.Errorf("draw = waiting %d, present %d", d.Waiting, d.Present)
	}
	if p := f.svc.GetTournamentView().Progress; p.Entrants != 0 || p.Players != 3 {
		t.Errorf("progress = %+v", p)
	}

	must(f.svc.PairEntrants(ctx, julie.ID, zoe.ID))
	if a := f.svc.GetTournamentView().LastAction; a.Type != "EntrantsPaired" || a.Params["names"] != "Julie Morel / Zoé Petit" {
		t.Errorf("pair action = %+v", a)
	}
	wantCode(t, f.svc.PairEntrants(ctx, julie.ID, ana.ID), engine.CodeNotWaitingPartner)
	must(f.svc.AddPartner(ctx, ana.ID, tournament.PlayerInput{Name: "Mia Roux", Club: "AS Bron"}))
	if a := f.svc.GetTournamentView().LastAction; a.Type != "PartnerAdded" || a.Params["names"] != "Ana Bel / Mia Roux" {
		t.Errorf("partner action = %+v", a)
	}
	if p, err := NewPresetService(f.app, f.rec.emit, func() time.Time { return f.clock }).GetPresets(ctx); err != nil || !slices.Contains(p.Clubs, "AS Bron") {
		t.Errorf("clubs = %v, %v", p.Clubs, err)
	}
	wd = f.svc.GetEntrantsView().Categories[0]
	if wd.Waiting != 0 || len(wd.Entrants) != 2 || wd.Entrants[0].Bib != 1 {
		t.Fatalf("after pairing = %+v", wd)
	}

	// Zoé leaves the pair: history names her, and later actions name her alone.
	zoeID := wd.Entrants[0].Players[1].ID
	must(f.svc.SplitEntrant(ctx, julie.ID, zoeID))
	if a := f.svc.GetTournamentView().LastAction; a.Type != "EntrantSplit" || a.Params["names"] != "Julie Morel / Zoé Petit" || a.Params["name"] != "Zoé Petit" {
		t.Errorf("split action = %+v", a)
	}
	wd = f.svc.GetEntrantsView().Categories[0]
	if wd.Waiting != 2 || len(wd.Entrants) != 3 {
		t.Fatalf("after split = %+v", wd)
	}
	// Split again after an undo: history names the pair, not the undone split.
	must(f.svc.Undo(ctx))
	must(f.svc.SplitEntrant(ctx, julie.ID, zoeID))
	if a := f.svc.GetTournamentView().LastAction; a.Params["names"] != "Julie Morel / Zoé Petit" {
		t.Errorf("split again = %+v", a)
	}
	wd = f.svc.GetEntrantsView().Categories[0]
	must(f.svc.RemoveEntrant(ctx, wd.Entrants[1].ID))
	if a := f.svc.GetTournamentView().LastAction; a.Params["names"] != "Zoé Petit" {
		t.Errorf("remove action = %+v", a)
	}
}
