package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// A tournament whose players typed a club by name or by acronym, before the
// list knew the acronym, shows them as one club once it does.
func TestClubsFollowTheList(t *testing.T) {
	f := newFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	presets := NewPresetService(f.app, f.svc.followClubs(f.rec.emit), func() time.Time { return f.clock })
	f.dialogs.save = filepath.Join(f.dir, "clubs")
	if _, err := f.svc.CreateTournament(f.ctx, "Plume"); err != nil {
		t.Fatal(err)
	}
	must(f.svc.AddCategory(f.ctx, engine.MensSingles))
	for _, p := range []tournament.PlayerInput{{Name: "Paul Zola", Club: "AUVR"}, {Name: "Tom Petit", Club: "Rillieux"}, {Name: "Max Roux", Club: "AS Bron"}} {
		must(f.svc.RegisterEntrant(f.ctx, engine.MensSingles, []tournament.PlayerInput{p}))
	}
	for _, e := range f.svc.GetEntrantsView().Categories[0].Entrants {
		must(f.svc.CheckInEntrant(f.ctx, e.ID))
	}
	badges := func() [][]ClubBadge {
		var out [][]ClubBadge
		for _, e := range f.svc.GetDrawView().Categories[0].Unseeded {
			out = append(out, e.Clubs)
		}
		return out
	}
	colorsOf := func(b [][]ClubBadge) []int {
		var out []int
		for _, x := range b {
			out = append(out, x[0].Color)
		}
		return out
	}
	// Typed alike nowhere: three clubs, three colors.
	if got := colorsOf(badges()); !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("colors = %v", got)
	}

	// The list learns that AUVR is Rillieux: both windows refetch.
	f.rec.events = nil
	must(presets.SaveClub(f.ctx, "AUVR", tournament.Club{Name: "Rillieux", Acronym: "AUVR"}))
	if !slices.ContainsFunc(f.rec.events, func(e emitted) bool { return e.name == EventTournamentUpdated }) {
		t.Errorf("events = %+v", f.rec.events)
	}
	b := badges()
	if got := colorsOf(b); !slices.Equal(got, []int{1, 1, 2}) || b[0][0].Name != "Rillieux" {
		t.Errorf("badges = %+v", b)
	}
	players := f.svc.GetEntrantsView().Categories[0].Entrants
	if p := players[0].Players[0]; p.Club != "Rillieux" || p.ClubShown != "Rillieux" {
		t.Errorf("player = %+v", p)
	}

	// Shown by acronym, wherever clubs show.
	must(f.settings.SetClubDisplay(f.ctx, tournament.ClubDisplayAcronym))
	b = badges()
	if b[0][0].Name != "AUVR" || b[0][0].Full != "Rillieux" || b[2][0].Name != "AS Bron" {
		t.Errorf("badges = %+v", b)
	}
	for _, team := range f.svc.GetCheckInView().Teams {
		if p := team.Players[0]; p.Name == "Paul Zola" && (p.Club != "Rillieux" || p.ClubShown != "AUVR") {
			t.Errorf("check-in player = %+v", p)
		}
	}
	side := sideViewer(f.svc.state)(players[1].ID)
	if !slices.Equal(side.Clubs, []string{"AUVR"}) || !slices.Equal(side.ClubNames, []string{"Rillieux"}) {
		t.Errorf("side = %+v", side)
	}
}

// Import: club cells match the list by name or acronym; the others are
// added, mapped to a listed club or kept as typed.
func TestImportClubChoices(t *testing.T) {
	f := newFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	presets := NewPresetService(f.app, f.svc.followClubs(f.rec.emit), func() time.Time { return f.clock })
	must(presets.SaveClub(f.ctx, "", tournament.Club{Name: "Rillieux", Acronym: "AUVR"}))
	must(presets.SaveClub(f.ctx, "", tournament.Club{Name: "Lyon Saint-Maurice", Acronym: "LSM"}))
	f.dialogs.save = filepath.Join(f.dir, "import")
	if _, err := f.svc.CreateTournament(f.ctx, "Plume"); err != nil {
		t.Fatal(err)
	}
	must(f.svc.AddCategory(f.ctx, engine.MensSingles))
	f.dialogs.openCSV = filepath.Join(f.dir, "list.csv")
	content := "Nom;Club\nPaul Zola;auvr\nTom Petit;PLVPB\nMax Roux;BCVO\nLéo Blanc;Club du coin\nJo Durand;Rillieux\n"
	must(os.WriteFile(f.dialogs.openCSV, []byte(content), 0o644))
	file, err := f.svc.ChooseImportFile(f.ctx, engine.MensSingles)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.svc.PreviewImport(file.Path, file.Mapping)
	if want := []tournament.ImportClub{{Value: "PLVPB", Lines: 1}, {Value: "BCVO", Lines: 1}, {Value: "Club du coin", Lines: 1}}; err != nil || !slices.Equal(preview.Clubs, want) {
		t.Fatalf("unknown clubs = %+v, %v", preview.Clubs, err)
	}
	m := file.Mapping
	m.Clubs = map[string]string{"PLVPB": "Pierre-Bénite", "BCVO": "LSM"}
	r, err := f.svc.ImportEntrants(f.ctx, file.Path, m, []tournament.Club{{Name: "Pierre-Bénite", Acronym: "PLVPB"}})
	if err != nil || r.Added != 5 {
		t.Fatalf("import = %+v, %v", r, err)
	}
	var clubs []string
	for _, e := range f.svc.GetEntrantsView().Categories[0].Entrants {
		clubs = append(clubs, e.Players[0].Club)
	}
	if want := []string{"Rillieux", "Pierre-Bénite", "Lyon Saint-Maurice", "Club du coin", "Rillieux"}; !slices.Equal(clubs, want) {
		t.Errorf("clubs = %v", clubs)
	}
	p, _ := presets.GetPresets(f.ctx)
	want := []tournament.Club{{Name: "Lyon Saint-Maurice", Acronym: "LSM"}, {Name: "Pierre-Bénite", Acronym: "PLVPB"}, {Name: "Rillieux", Acronym: "AUVR"}}
	if !slices.Equal(p.Clubs, want) {
		t.Errorf("club list = %+v", p.Clubs)
	}
}

// The club list of an older app.db holds plain names: read as clubs without
// acronym, written back as clubs.
func TestOlderClubList(t *testing.T) {
	f := newBareFixture(t)
	if err := f.app.SetSetting(f.ctx, settingPresets, `{"clubs":["AS Bron","Rillieux"]}`); err != nil {
		t.Fatal(err)
	}
	presets := NewPresetService(f.app, f.rec.emit, func() time.Time { return f.clock })
	if err := presets.SaveClub(f.ctx, "Rillieux", tournament.Club{Name: "Rillieux", Acronym: "AUVR"}); err != nil {
		t.Fatal(err)
	}
	p, err := presets.GetPresets(f.ctx)
	if want := []tournament.Club{{Name: "AS Bron"}, {Name: "Rillieux", Acronym: "AUVR"}}; err != nil || !slices.Equal(p.Clubs, want) {
		t.Errorf("clubs = %+v, %v", p.Clubs, err)
	}
	raw, _, _ := f.app.Setting(f.ctx, settingPresets)
	if !strings.Contains(raw, `"clubs":[{"name":"AS Bron","acronym":""},{"name":"Rillieux","acronym":"AUVR"}]`) {
		t.Errorf("stored = %s", raw)
	}
}
