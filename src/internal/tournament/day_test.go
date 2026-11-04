package tournament

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

func TestSetDay(t *testing.T) {
	tests := []struct {
		name string
		day  DaySet
		ok   bool
	}{
		{"all set", DaySet{Date: "2026-11-14", Start: "09:00", End: "19:30"}, true},
		{"start only", DaySet{Start: "08:30"}, true},
		{"nothing set", DaySet{}, true},
		{"bad date", DaySet{Date: "14/11/2026"}, false},
		{"bad hour", DaySet{Start: "9h"}, false},
		{"hour out of range", DaySet{End: "25:00"}, false},
		{"end before start", DaySet{Start: "10:00", End: "09:00"}, false},
		{"end at start", DaySet{Start: "10:00", End: "10:00"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGame(t, engine.MensSingles)
			if !tt.ok {
				g.fails(engine.CodeInvalidTime, func(s State) (Change, error) { return s.SetDay(tt.day) })
				return
			}
			ch := g.do(func(s State) (Change, error) { return s.SetDay(tt.day) })
			if got := g.state().Day(); got != tt.day {
				t.Errorf("day = %+v, want %+v", got, tt.day)
			}
			// Setting it again changes nothing.
			if ch2, _ := g.state().SetDay(tt.day); !ch2.None() || (tt.day == DaySet{} && !ch.None()) {
				t.Errorf("unchanged day appended an event")
			}
		})
	}
}

func TestBreaks(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	for _, bad := range [][2]string{{"12:00", "12:00"}, {"13:00", "12:00"}, {"", "12:00"}, {"12h", "13:00"}} {
		g.fails(engine.CodeInvalidTime, func(s State) (Change, error) { return s.AddBreak(bad[0], bad[1]) })
	}
	g.do(func(s State) (Change, error) { return s.AddBreak("15:00", "15:15") })
	g.do(func(s State) (Change, error) { return s.AddBreak("12:00", "13:30") })
	got := g.state().Breaks()
	want := []Break{{ID: "k4", From: "12:00", To: "13:30"}, {ID: "k3", From: "15:00", To: "15:15"}}
	if !slices.Equal(got, want) {
		t.Fatalf("breaks = %v, want %v", got, want)
	}
	g.fails(engine.CodeInvalidSetting, func(s State) (Change, error) { return s.RemoveBreak("k9") })
	g.do(func(s State) (Change, error) { return s.RemoveBreak("k3") })
	if got := g.state().Breaks(); !slices.Equal(got, want[:1]) {
		t.Errorf("after removal = %v", got)
	}
	g.undo() // the removal
	g.undo() // the lunch break
	if got := g.state().Breaks(); !slices.Equal(got, []Break{{ID: "k3", From: "15:00", To: "15:15"}}) {
		t.Errorf("after undos = %v", got)
	}
}

func TestForecastStart(t *testing.T) {
	// t0 is 2026-10-17 09:00 UTC.
	tests := []struct {
		name   string
		day    DaySet
		now    time.Time
		called bool
		want   time.Time
	}{
		{"no start hour: now", DaySet{}, t0, false, t0},
		{"start ahead today", DaySet{Start: "10:30"}, t0, false, t0.Add(90 * time.Minute)},
		{"start past: now", DaySet{Start: "08:00"}, t0, false, t0},
		{"start on a later date", DaySet{Date: "2026-10-18", Start: "08:00"}, t0, false, t0.Add(23 * time.Hour)},
		{"date past: now", DaySet{Date: "2026-10-16", Start: "10:00"}, t0, false, t0},
		{"a match called: now", DaySet{Start: "10:30"}, t0, true, t0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGame(t, engine.MensSingles)
			g.confirmedPools(engine.MensSingles, 1, 4)
			g.do(func(s State) (Change, error) { return s.SetDay(tt.day) })
			if tt.called {
				g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 1) })
			}
			st := g.state()
			if got := st.ForecastStart(tt.now); !got.Equal(tt.want) {
				t.Errorf("start = %v, want %v", got, tt.want)
			}
			sc := st.Schedule(tt.now)
			first := sc.Slots[0].Start
			for _, sl := range sc.Slots {
				if sl.Start.Before(first) {
					first = sl.Start
				}
			}
			if !tt.called && !first.Equal(tt.want) {
				t.Errorf("first slot at %v, want %v", first, tt.want)
			}
		})
	}
}

func TestScheduleBreaksAndProposals(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4) // 3 rounds of 2 matches, 29 min each
	g.do(func(s State) (Change, error) { return s.SetCourts(2) })
	g.do(func(s State) (Change, error) { return s.SetDay(DaySet{Start: "08:00", End: "10:00"}) })
	g.do(func(s State) (Change, error) { return s.AddBreak("09:40", "10:30") })
	st := g.state()
	sc := st.Schedule(t0)
	brk := st.BreakTimes(t0)[0]
	for _, sl := range sc.Slots {
		if sl.Start.Before(brk.To) && sl.End.After(brk.From) {
			t.Errorf("slot %s %v-%v runs into the break", sl.Match, sl.Start, sl.End)
		}
	}
	// Round 1 at 9:00, round 2 at 9:29 would end at 9:58: after the break.
	if got := len(startsAt(sc, brk.To)); got != 2 {
		t.Errorf("%d matches start after the break, want 2", got)
	}
	if !sc.End.After(st.DayEnd(t0)) {
		t.Errorf("end %v should be after the end hour", sc.End)
	}

	// The desk may start before the start hour: proposals ignore it.
	g.do(func(s State) (Change, error) { return s.SetDay(DaySet{Start: "11:00"}) })
	if got := len(g.state().Proposals(t0)); got != 2 {
		t.Errorf("proposals before the start = %d, want 2", got)
	}
	if got := len(g.state().Proposals(brk.From.Add(5 * time.Minute))); got != 0 {
		t.Errorf("proposals during the break = %d, want 0", got)
	}
}

func TestGroupStyles(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 7) // pools of 4 and 3
	main := g.addStage(engine.MensSingles, StageBracket, "Main", true)
	cons := g.addStage(engine.MensSingles, StageBracket, "Consolation", false)
	g.editStage(engine.MensSingles, "S1", func(st *StageConfig) {
		st.Routing = []RouteRule{{Kind: RouteRanks, Ranks: []int{1, 2}, To: main}, {Kind: RouteRest, To: cons}}
	})
	styles := g.state().GroupStyles()
	want := map[string]GroupStyle{
		"MS/S1/A":    {Color: 1, Rounds: 3},
		"MS/S1/B":    {Color: 2, Rounds: 3},
		"MS/" + main: {Color: 3},
		"MS/" + cons: {Color: 4},
	}
	for k, w := range want {
		if styles[k] != w {
			t.Errorf("%s = %+v, want %+v", k, styles[k], w)
		}
	}
	if got := GroupStyleOf(styles, engine.MensSingles, main, -1); got.Color != 3 {
		t.Errorf("bracket style = %+v", got)
	}
}

// Colors run through the tournament: the second category goes on from the
// first, and a color comes back only past the palette.
func TestGroupStylesAcrossCategories(t *testing.T) {
	g := newGame(t, engine.MensSingles, engine.WomensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	g.confirmedPools(engine.WomensSingles, 1, 4)
	styles := g.state().GroupStyles()
	want := map[string]int{"MS/S1/A": 1, "MS/S1/B": 2, "WS/S1/A": 3}
	for k, w := range want {
		if styles[k].Color != w {
			t.Errorf("%s = %d, want %d", k, styles[k].Color, w)
		}
	}

	// 16 pools in MS use up the palette: WS pool A starts it again.
	g = newGame(t, engine.MensSingles, engine.WomensSingles)
	g.confirmedPools(engine.MensSingles, GroupColors, 2*GroupColors)
	g.confirmedPools(engine.WomensSingles, 1, 3)
	styles = g.state().GroupStyles()
	seen := map[int]bool{}
	for p := range GroupColors {
		seen[GroupStyleOf(styles, engine.MensSingles, "S1", p).Color] = true
	}
	if len(seen) != GroupColors {
		t.Errorf("%d colors for the first %d pools", len(seen), GroupColors)
	}
	if c := GroupStyleOf(styles, engine.WomensSingles, "S1", 0).Color; c != 1 {
		t.Errorf("pool past the palette = %d, want 1", c)
	}
}

func TestStageAcronym(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.editStage(engine.MensSingles, "S1", func(st *StageConfig) { st.Acronym = " P 1 " })
	if st, _ := g.state().Stage(engine.MensSingles, "S1"); st.Acronym != "P1" {
		t.Errorf("acronym = %q, want spaces dropped", st.Acronym)
	}
	g.fails(engine.CodeInvalidName, func(s State) (Change, error) {
		stages := s.StageConfigs(engine.MensSingles)
		stages[0].Acronym = "ABCD"
		return s.SetFormat(engine.MensSingles, stages)
	})

	// A started stage keeps its acronym editable.
	g.confirmedPools(engine.MensSingles, 1, 4)
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 1) })
	g.editStage(engine.MensSingles, "S1", func(st *StageConfig) { st.Acronym = "Q" })
	if st, _ := g.state().Stage(engine.MensSingles, "S1"); st.Acronym != "Q" {
		t.Errorf("started stage acronym = %q", st.Acronym)
	}
}

// Files written before the day, breaks and acronyms replay with none.
func TestReplayWithoutDayBreaksAcronyms(t *testing.T) {
	format := `{"category":"MS","stages":[{"id":"S1","name":"","kind":"pools","win":{"kind":"sets","setsToWin":2,"pointsPerSet":21,"cap":30},"pools":1,"tiebreaks":["wins"]}]}`
	log := []events.Event{
		{Seq: 1, Type: TypeCreated, Data: json.RawMessage(`{"name":"Plume"}`), At: t0},
		{Seq: 2, Type: TypeCategoryAdded, Data: json.RawMessage(`{"category":"MS"}`), At: t0},
		{Seq: 3, Type: TypeFormatSet, Data: json.RawMessage(format), At: t0},
	}
	s, err := Project(log)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := s.Stage(engine.MensSingles, "S1")
	if st.Acronym != "" || s.Day() != (DaySet{}) || len(s.Breaks()) != 0 || !s.ForecastStart(t0).Equal(t0) {
		t.Errorf("acronym %q, day %+v, breaks %v", st.Acronym, s.Day(), s.Breaks())
	}
}
