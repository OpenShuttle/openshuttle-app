package tournament

import (
	"slices"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// startsAt lists the slots starting at t, by court.
func startsAt(sc Schedule, t time.Time) []ScheduleSlot {
	var out []ScheduleSlot
	for _, s := range sc.Slots {
		if s.Start.Equal(t) {
			out = append(out, s)
		}
	}
	return out
}

func slotCategories(slots []ScheduleSlot) []engine.Category {
	var out []engine.Category
	for _, s := range slots {
		out = append(out, s.Category)
	}
	return out
}

// est is the estimate of the default win condition (2 sets of 21): 29 min.
var est = 29 * time.Minute

func TestScheduleAlternate(t *testing.T) {
	g := newGame(t, engine.MensSingles, engine.WomensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4) // rounds of 2 matches
	g.confirmedPools(engine.WomensSingles, 1, 3)

	// Round 1 of MS, then round 1 of WS; round 2 waits for its players.
	sc := g.state().Schedule(t0)
	if got := slotCategories(startsAt(sc, t0)); !slices.Equal(got, []engine.Category{"MS", "MS", "WS"}) {
		t.Errorf("first wave = %v", got)
	}
	if len(sc.Slots) != 6+3 || !sc.End.After(t0) {
		t.Errorf("slots = %d, end %v", len(sc.Slots), sc.End)
	}
	g.do(func(s State) (Change, error) { return s.SetScheduleFirst(engine.WomensSingles) })
	if got := slotCategories(startsAt(g.state().Schedule(t0), t0)); !slices.Equal(got, []engine.Category{"WS", "MS", "MS"}) {
		t.Errorf("WS first = %v", got)
	}
}

func TestScheduleBracketRounds(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	main := g.addStage(engine.MensSingles, StageBracket, "Main", true)
	g.route(engine.MensSingles, "S1", main, main)
	g.finishPools(engine.MensSingles)
	g.fill(engine.MensSingles, main)

	// Four courts, but the final and third place wait for both semis.
	st, _ := g.state().Stage(engine.MensSingles, main)
	semi := time.Duration(st.Win.EstimatedMinutes()) * time.Minute
	sc := g.state().Schedule(t0)
	if len(sc.Slots) != 4 {
		t.Fatalf("slots = %+v", sc.Slots)
	}
	for _, s := range sc.Slots {
		want := t0
		if s.Level == 1 {
			want = t0.Add(semi)
		}
		if !s.Start.Equal(want) || s.Key != "MS/"+main+"/R"+string(rune('1'+s.Level)) {
			t.Errorf("slot %s level %d at %v (%s)", s.Match, s.Level, s.Start, s.Key)
		}
	}
}

func TestAutoOrder(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 4, 18) // pools of 5, 5, 4 and 4

	// Round by round, the pools of 5 (more matches) first, then those of 4;
	// the pools of 5 play two more rounds on their own.
	var pools []string
	for _, id := range g.state().AutoOrder() {
		m, _ := g.state().Match(id)
		pools = append(pools, engine.PoolName(m.Pool))
	}
	round := []string{"A", "A", "B", "B", "C", "C", "D", "D"}
	want := slices.Concat(round, round, round, []string{"A", "A", "B", "B"}, []string{"A", "A", "B", "B"})
	if !slices.Equal(pools, want) {
		t.Errorf("order = %v\nwant    %v", pools, want)
	}
	// With nothing validated, the schedule follows that order.
	if !slices.Equal(g.state().ScheduleOrder(), g.state().AutoOrder()) {
		t.Error("schedule order differs from the rules before validation")
	}
}

func TestValidateAndMove(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.SetCourts(1) })
	g.confirmedPools(engine.MensSingles, 2, 8)
	g.fails(engine.CodeScheduleNotValidated, func(s State) (Change, error) { return s.MoveScheduledMatch("MS-B1", "") })

	g.do(func(s State) (Change, error) { return s.ValidateSchedule() })
	if g.state().ScheduleValidatedAt().IsZero() {
		t.Fatal("not validated")
	}
	order := g.state().ScheduleOrder()

	// One match to the front.
	last := order[len(order)-1]
	g.do(func(s State) (Change, error) { return s.MoveScheduledMatch(last, order[0]) })
	if got := g.state().ScheduleOrder(); got[0] != last || len(got) != len(order) {
		t.Errorf("after moving %s: %v", last, got)
	}
	if first := g.state().Schedule(t0).Slots[0]; first.Match != last {
		t.Errorf("first match played = %s, want %s", first.Match, last)
	}

	// The whole pool B to the end, its matches in their order.
	g.do(func(s State) (Change, error) { return s.MoveScheduledPool(engine.MensSingles, "S1", 1, "") })
	got := g.state().ScheduleOrder()
	for i, id := range got {
		m, _ := g.state().Match(id)
		if (i >= len(got)-6) != (m.Pool == 1) {
			t.Fatalf("pool B not at the end: %v", got)
		}
	}
	// Played matches leave the order; the moves stay.
	g.do(func(s State) (Change, error) { return s.CallMatch(got[0], 1) })
	if next := g.state().ScheduleOrder(); next[0] != got[1] {
		t.Errorf("after the call = %v", next)
	}
	g.fails(engine.CodeMatchNotFound, func(s State) (Change, error) { return s.MoveScheduledMatch(got[0], "") })

	// Validating again recomputes the order from the rules.
	g.do(func(s State) (Change, error) { return s.ValidateSchedule() })
	if !slices.Equal(g.state().ScheduleOrder(), g.state().AutoOrder()) {
		t.Error("validating again kept the moves")
	}
}

func TestMatchesLeft(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4)
	m, _ := g.state().Match("MS-A1")
	g.do(func(s State) (Change, error) { return s.RecordScore("MS-A1", straight) })
	left := g.state().MatchesLeft(engine.MensSingles, "S1")
	if left[m.A] != 2 || left[m.B] != 2 || len(left) != 4 {
		t.Errorf("left = %v", left)
	}
}

func TestScheduleUnrested(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.SetCourts(2) })
	g.confirmedPools(engine.MensSingles, 1, 4)

	// Round 2 starts when round 1 ends: nobody has rested 10 minutes.
	sc := g.state().Schedule(t0)
	for _, s := range sc.Slots {
		if s.Unrested != !s.Start.Equal(t0) {
			t.Errorf("slot %s at %v unrested %v", s.Match, s.Start, s.Unrested)
		}
	}
	g.do(func(s State) (Change, error) { return s.SetMinRest(0) })
	for _, s := range g.state().Schedule(t0).Slots {
		if s.Unrested {
			t.Errorf("no minimum rest: %s unrested", s.Match)
		}
	}
}

func TestScheduleReserved(t *testing.T) {
	g := newGame(t, engine.MensSingles, engine.WomensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4)
	g.confirmedPools(engine.WomensSingles, 1, 4)
	g.do(func(s State) (Change, error) { return s.SetScheduleMode(ScheduleReserved) })

	courtsOf := func(sc Schedule) map[engine.Category][]int {
		out := map[engine.Category][]int{}
		for _, s := range sc.Slots {
			if !slices.Contains(out[s.Category], s.Court) {
				out[s.Category] = append(out[s.Category], s.Court)
			}
		}
		for _, v := range out {
			slices.Sort(v)
		}
		return out
	}
	sc := g.state().Schedule(t0)
	if got := courtsOf(sc); !slices.Equal(got["MS"], []int{1, 2}) || !slices.Equal(got["WS"], []int{3, 4}) {
		t.Errorf("default share = %v (%v)", got, sc.Reserved)
	}
	g.do(func(s State) (Change, error) { return s.SetCourtCategory(4, engine.MensSingles) })
	if r := g.state().Schedule(t0).Reserved; r[4] != "MS" || r[3] != "WS" {
		t.Errorf("court 4 reserved for MS = %v", r)
	}
	g.fails(engine.CodeInvalidCourt, func(s State) (Change, error) { return s.SetCourtCategory(9, engine.MensSingles) })
}

func TestSchedulePlaceholders(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	main := g.addStage(engine.MensSingles, StageBracket, "Main", true)
	g.route(engine.MensSingles, "S1", main, main)

	sc := g.state().Schedule(t0)
	var poolsEnd time.Time
	var stand []ScheduleSlot
	for _, s := range sc.Slots {
		if s.Stage == "S1" {
			poolsEnd = later(poolsEnd, s.End)
		} else {
			stand = append(stand, s)
		}
	}
	// Semis then final and third place, unknown players, after the pools.
	if len(stand) != 4 {
		t.Fatalf("stand-ins = %+v", stand)
	}
	for _, s := range stand {
		if s.Match != "" || s.Start.Before(poolsEnd) {
			t.Errorf("stand-in %+v before %v", s, poolsEnd)
		}
	}
	if !sc.End.Equal(stand[3].End) {
		t.Errorf("end = %v", sc.End)
	}
}

func TestScheduleLateMatch(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.SetCourts(2) })
	g.confirmedPools(engine.MensSingles, 1, 4)
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 1) })

	// Long after its estimate, MS-A1 is still on court: its players are not
	// proposed elsewhere.
	late := t0.Add(2 * time.Hour)
	for court, p := range g.state().Proposals(late) {
		m, _ := g.state().Match(p.Match)
		a1, _ := g.state().Match("MS-A1")
		if m.Has(a1.A) || m.Has(a1.B) {
			t.Errorf("court %d proposes %s with a player on court", court, p.Match)
		}
	}
}
