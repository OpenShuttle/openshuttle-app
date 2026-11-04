package tournament

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestClubKey(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"Rillieux", "rillieux"},
		{"  RILLIEUX ", "rillieux"},
		{"Rillieux-la-Pape", "rillieuxlapape"},
		{"rillieux la pape", "rillieuxlapape"},
		{"A.U.V.R.", "auvr"},
		{"Bourg-en-Bresse Bad'", "bourgenbressebad"},
		{"Évian", "evian"},
		{"BC 08", "bc08"},
	}
	for _, tt := range tests {
		if got := ClubKey(tt.in); got != tt.want {
			t.Errorf("ClubKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestClubBook(t *testing.T) {
	list := []Club{
		{Name: "Rillieux", Acronym: "AUVR"},
		{Name: "Bad Club Villefranche Ouest", Acronym: "BCVO"},
		{Name: "Lyon Saint-Maurice"},
		// Its acronym is another club's name: the name wins.
		{Name: "Les Sports de Mâcon", Acronym: "Rillieux"},
	}
	tests := []struct {
		value                       string
		found                       bool
		name, id, byName, byAcronym string
	}{
		{"", false, "", "", "", ""},
		{"Rillieux", true, "Rillieux", "rillieux", "Rillieux", "AUVR"},
		{"auvr", true, "Rillieux", "rillieux", "Rillieux", "AUVR"},
		{"A.U.V.R", true, "Rillieux", "rillieux", "Rillieux", "AUVR"},
		{"bad club villefranche-ouest", true, "Bad Club Villefranche Ouest", "badclubvillefrancheouest", "Bad Club Villefranche Ouest", "BCVO"},
		{"lyon saint maurice", true, "Lyon Saint-Maurice", "lyonsaintmaurice", "Lyon Saint-Maurice", "Lyon Saint-Maurice"},
		{"Les sports de macon", true, "Les Sports de Mâcon", "lessportsdemacon", "Les Sports de Mâcon", "Rillieux"},
		{"PLVPB", false, "PLVPB", "plvpb", "PLVPB", "PLVPB"},
	}
	byName := NewClubBook(list, ClubDisplayName)
	byAcronym := NewClubBook(list, ClubDisplayAcronym)
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if _, ok := byName.Find(tt.value); ok != tt.found {
				t.Errorf("Find found %v, want %v", ok, tt.found)
			}
			if got := byName.Name(tt.value); got != tt.name {
				t.Errorf("Name = %q, want %q", got, tt.name)
			}
			if got := byName.ID(tt.value); got != tt.id {
				t.Errorf("ID = %q, want %q", got, tt.id)
			}
			if got := byName.Label(tt.value); got != tt.byName {
				t.Errorf("Label by name = %q, want %q", got, tt.byName)
			}
			if got := byAcronym.Label(tt.value); got != tt.byAcronym {
				t.Errorf("Label by acronym = %q, want %q", got, tt.byAcronym)
			}
		})
	}
	var zero ClubBook
	if got := zero.Label("AUVR"); got != "AUVR" {
		t.Errorf("zero book Label = %q, want as typed", got)
	}
}

func TestClubJSON(t *testing.T) {
	tests := []struct {
		name, in string
		want     []Club
	}{
		{"plain names of older lists", `["AS Bron","Rillieux"]`, []Club{{Name: "AS Bron"}, {Name: "Rillieux"}}},
		{"name and acronym", `[{"name":"Rillieux","acronym":"AUVR"},{"name":"AS Bron"}]`, []Club{{Name: "Rillieux", Acronym: "AUVR"}, {Name: "AS Bron"}}},
		{"mixed", `["AS Bron",{"name":"Rillieux","acronym":"AUVR"}]`, []Club{{Name: "AS Bron"}, {Name: "Rillieux", Acronym: "AUVR"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []Club
			if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEntrantClubs(t *testing.T) {
	st := State{}.WithClubs(NewClubBook([]Club{{Name: "Rillieux", Acronym: "AUVR"}}, ClubDisplayAcronym))
	e := Entrant{Players: []Player{{Club: "auvr"}, {Club: "Rillieux"}, {Club: ""}, {Club: "AS Bron"}}}
	want := []EntrantClub{{"rillieux", "Rillieux", "AUVR"}, {"asbron", "AS Bron", "AS Bron"}}
	if got := st.EntrantClubs(e); !slices.Equal(got, want) {
		t.Errorf("EntrantClubs = %+v, want %+v", got, want)
	}
}

func TestShareClubByAcronym(t *testing.T) {
	a := Entrant{Players: []Player{{Club: "AUVR"}}}
	b := Entrant{Players: []Player{{Club: "Bron"}, {Club: "rillieux"}}}
	if (State{}).shareClub(a, b) {
		t.Error("unlisted acronym shares a club with the name")
	}
	st := State{}.WithClubs(NewClubBook([]Club{{Name: "Rillieux", Acronym: "AUVR"}}, ClubDisplayName))
	if !st.shareClub(a, b) {
		t.Error("acronym and name of a listed club are not the same club")
	}
}
