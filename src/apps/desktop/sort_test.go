package main

import (
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestSortTeams(t *testing.T) {
	team := func(id string, players ...PlayerView) CheckInTeam {
		return CheckInTeam{EntrantID: engine.EntrantID(id), Players: players}
	}
	in := func(name string) PlayerView { return PlayerView{Name: name, CheckedIn: true} }
	out := func(name string) PlayerView { return PlayerView{Name: name} }

	tests := []struct {
		name  string
		teams []CheckInTeam
		want  []string
	}{
		{"by first name, accents and case ignored",
			[]CheckInTeam{team("a", out("Zoé Petit")), team("b", out("élise Roux")), team("c", out("Bruno Ali"))},
			[]string{"c", "b", "a"}},
		{"teams all checked in go last, still by name",
			[]CheckInTeam{team("a", in("Anne Roy")), team("b", out("Zoé Petit")), team("c", in("Bruno Ali")), team("d", out("Léa Roux"))},
			[]string{"d", "b", "a", "c"}},
		{"a pair with one player here stays on top",
			[]CheckInTeam{team("a", in("Anne Roy"), in("Marc Roy")), team("b", in("Zoé Petit"), out("Paul Petit"))},
			[]string{"b", "a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teams := slices.Clone(tt.teams)
			sortTeams(teams)
			var got []string
			for _, team := range teams {
				got = append(got, string(team.EntrantID))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("order = %v, want %v", got, tt.want)
			}
		})
	}
}
