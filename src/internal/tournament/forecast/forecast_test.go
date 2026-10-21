package forecast

import (
	"maps"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

var t0 = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

const md, wd = engine.MensDoubles, engine.WomensDoubles

// at is t0 plus minutes.
func at(minutes int) time.Time { return t0.Add(time.Duration(minutes) * time.Minute) }

// match is a pool match of MD stage S1, 20 minutes.
func match(id string, a, b engine.EntrantID) Item {
	return Item{Key: "MD/S1/A/1", Category: md, Stage: "S1", Match: id, A: a, B: b, Minutes: 20}
}

// placed sums a forecast up: match → court and start minute.
type placed struct {
	court, start int
	unrested     bool
}

func summary(r Result) map[string]placed {
	out := map[string]placed{}
	for _, s := range r.Slots {
		out[s.Match] = placed{s.Court, int(s.Start.Sub(t0).Minutes()), s.Unrested}
	}
	return out
}

func TestRun(t *testing.T) {
	semi := func(id string) Item {
		return Item{Key: "MD/S2/R1", Category: md, Stage: "S2", Pool: -1, Level: 0, Match: id, Minutes: 20}
	}
	final := Item{Key: "MD/S2/R2", Category: md, Stage: "S2", Pool: -1, Level: 1, Match: "F", Minutes: 20}
	tests := []struct {
		name string
		in   Input
		want map[string]placed
		end  int
	}{
		{
			name: "courts take matches in order, a player waits for his previous match",
			in:   Input{Courts: 2, Items: []Item{match("m1", "a", "b"), match("m2", "c", "d"), match("m3", "a", "c")}},
			want: map[string]placed{"m1": {1, 0, false}, "m2": {2, 0, false}, "m3": {1, 20, false}},
			end:  40,
		},
		{
			name: "rest is flagged, never waited for",
			in:   Input{Courts: 1, Rest: 10 * time.Minute, Items: []Item{match("m1", "a", "b"), match("m2", "a", "c")}},
			want: map[string]placed{"m1": {1, 0, false}, "m2": {1, 20, true}},
			end:  40,
		},
		{
			name: "a match on court holds its court and players; a played match starts the rest",
			// Court 2 frees up first: m1 waits for a, so it takes m2 (c not rested).
			in: Input{Courts: 2, Rest: 30 * time.Minute,
				OnCourt: []OnCourt{{Court: 1, Category: md, Stage: "S1", A: "a", B: "x", End: at(15)}},
				Played:  []Played{{A: "c", B: "y", End: at(-5)}},
				Items:   []Item{match("m1", "a", "b"), match("m2", "c", "d")}},
			want: map[string]placed{"m1": {1, 15, false}, "m2": {2, 0, true}},
			end:  35,
		},
		{
			name: "a bracket round waits for the previous one",
			in:   Input{Courts: 3, Items: []Item{final, semi("S1"), semi("S2")}},
			want: map[string]placed{"S1": {1, 0, false}, "S2": {2, 0, false}, "F": {1, 20, false}},
			end:  40,
		},
		{
			name: "a stage not filled yet waits for its sources",
			in: Input{Courts: 2, Items: []Item{
				{Key: "MD/S2/R1", Category: md, Stage: "S2", Pool: -1, Minutes: 20, After: []string{"S1"}, Match: "stand-in"},
				match("m1", "a", "b"),
			}},
			want: map[string]placed{"m1": {1, 0, false}, "stand-in": {1, 20, false}},
			end:  40,
		},
		{
			name: "reserved courts until the category is over",
			in: Input{Courts: 2, Reserved: map[int]engine.Category{1: md, 2: wd}, Items: []Item{
				match("m1", "a", "b"), match("m2", "c", "d"),
				{Key: "WD/S1/A/1", Category: wd, Stage: "S1", Match: "w1", A: "e", B: "f", Minutes: 20},
			}},
			want: map[string]placed{"m1": {1, 0, false}, "w1": {2, 0, false}, "m2": {1, 20, false}},
			end:  40,
		},
		{
			name: "stages waiting on each other are never placed",
			in: Input{Courts: 1, Items: []Item{
				match("m1", "a", "b"),
				{Key: "MD/S2/R1", Category: md, Stage: "S2", Pool: -1, Match: "x", Minutes: 20, After: []string{"S3"}},
				{Key: "MD/S3/R1", Category: md, Stage: "S3", Pool: -1, Match: "y", Minutes: 20, After: []string{"S2"}},
			}},
			want: map[string]placed{"m1": {1, 0, false}},
			end:  20,
		},
		{
			name: "no courts",
			in:   Input{Courts: 0, Items: []Item{match("m1", "a", "b")}},
			want: map[string]placed{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.Now = t0
			r := Run(tt.in)
			if got := summary(r); !maps.Equal(got, tt.want) {
				t.Errorf("placed = %v\nwant     %v", got, tt.want)
			}
			if tt.end > 0 && !r.End.Equal(at(tt.end)) {
				t.Errorf("end = %v, want %v", r.End, at(tt.end))
			}
		})
	}
}

func TestReserveCourts(t *testing.T) {
	tests := []struct {
		name       string
		courts     int
		manual     map[int]engine.Category
		categories []engine.Category
		left       map[engine.Category]int
		want       map[int]engine.Category
	}{
		{"one category: no reservation", 4, nil, []engine.Category{md}, map[engine.Category]int{md: 10}, map[int]engine.Category{}},
		{"shared by the matches left", 4, nil, []engine.Category{md, wd}, map[engine.Category]int{md: 30, wd: 10},
			map[int]engine.Category{1: md, 2: md, 3: md, 4: wd}},
		{"at least one court each", 4, nil, []engine.Category{md, wd}, map[engine.Category]int{md: 1, wd: 99},
			map[int]engine.Category{1: md, 2: wd, 3: wd, 4: wd}},
		{"courts reserved by hand count", 4, map[int]engine.Category{4: md}, []engine.Category{md, wd}, map[engine.Category]int{md: 10, wd: 10},
			map[int]engine.Category{1: md, 2: wd, 3: wd, 4: md}},
		{"nothing left: only the manual ones", 3, map[int]engine.Category{2: wd}, []engine.Category{md, wd}, map[engine.Category]int{},
			map[int]engine.Category{2: wd}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReserveCourts(tt.courts, tt.manual, tt.categories, tt.left); !maps.Equal(got, tt.want) {
				t.Errorf("courts = %v, want %v", got, tt.want)
			}
		})
	}
}
