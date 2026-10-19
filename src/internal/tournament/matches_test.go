package tournament

import (
	"slices"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// confirmedPools sets up a category with the given pools, draws and confirms.
// Clubs are all distinct so the draw is free.
func (g *game) confirmedPools(c engine.Category, pools, entrants int) {
	g.t.Helper()
	clubs := make([]string, entrants)
	for i := range clubs {
		clubs[i] = string(rune('a' + i))
	}
	g.presentSingles(c, clubs, entrants)
	g.editStage(c, "S1", func(st *StageConfig) { st.Pools = pools })
	g.do(func(s State) (Change, error) { return s.DrawEntry(c, rng()) })
	g.do(func(s State) (Change, error) { return s.ConfirmDraw(c) })
}

func ids(ms []Match) []MatchID {
	out := []MatchID{}
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

var straight = engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 12}}}

func TestMatchesFromConfirmedDraw(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	ms := g.state().Matches()
	if len(ms) != 12 {
		t.Fatalf("%d matches, want 12", len(ms))
	}
	// Round by round, pools interleaved.
	want := []MatchID{"MS-A1", "MS-A2", "MS-B1", "MS-B2", "MS-A3", "MS-A4"}
	if got := ids(ms)[:6]; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	// Reopening drops them; confirming again rebuilds the same ones.
	g.do(func(s State) (Change, error) { return s.ReopenDraw(engine.MensSingles) })
	if len(g.state().Matches()) != 0 {
		t.Fatal("matches kept after reopening")
	}
	g.do(func(s State) (Change, error) { return s.ConfirmDraw(engine.MensSingles) })
	if !slices.Equal(ids(g.state().Matches()), ids(ms)) {
		t.Error("matches differ after confirming again")
	}

	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 1) })
	g.fails(engine.CodeMatchesStarted, func(s State) (Change, error) { return s.ReopenDraw(engine.MensSingles) })
	m, _ := g.state().Match("MS-A1")
	g.fails(engine.CodeEntrantInPools, func(s State) (Change, error) { return s.RemoveEntrant(m.A) })
}

func TestMatchLifecycle(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)

	g.fails(engine.CodeMatchNotFound, func(s State) (Change, error) { return s.CallMatch("MS-Z9", 1) })
	g.fails(engine.CodeInvalidCourt, func(s State) (Change, error) { return s.CallMatch("MS-A1", 5) })
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 1) })
	g.fails(engine.CodeMatchStarted, func(s State) (Change, error) { return s.CallMatch("MS-A1", 2) })
	g.fails(engine.CodeCourtBusy, func(s State) (Change, error) { return s.CallMatch("MS-B1", 1) })
	// MS-A3 shares an entrant with MS-A1 (round 2 of the same pool).
	a1, _ := g.state().Match("MS-A1")
	busy := slices.IndexFunc(g.state().Matches(), func(m Match) bool { return m.ID != a1.ID && (m.Has(a1.A) || m.Has(a1.B)) })
	other := g.state().Matches()[busy].ID
	g.fails(engine.CodeEntrantPlaying, func(s State) (Change, error) { return s.CallMatch(other, 2) })

	if c := g.state().CourtStates()[0]; c.State != CourtPlaying || c.Match.ID != "MS-A1" {
		t.Fatalf("court 1 = %+v", c)
	}

	// End at 10:12, the court is finished, the score pending.
	g.now = t0.Add(12 * time.Minute)
	g.do(func(s State) (Change, error) { return s.EndMatch("MS-A1") })
	g.fails(engine.CodeMatchNotPlaying, func(s State) (Change, error) { return s.EndMatch("MS-A1") })
	if c := g.state().CourtStates()[0]; c.State != CourtFinished {
		t.Fatalf("court 1 after end = %+v", c)
	}

	// The next match starts on the finished court before the score is entered.
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-B1", 1) })
	if c := g.state().CourtStates()[0]; c.State != CourtPlaying || c.Match.ID != "MS-B1" {
		t.Fatalf("court 1 after next call = %+v", c)
	}
	if got := ids(g.state().PendingScores()); !slices.Equal(got, []MatchID{"MS-A1"}) {
		t.Errorf("pending = %v", got)
	}

	// Scores: invalid, then valid; a score on a match in progress ends it.
	g.fails(engine.CodeInvalidSetScore, func(s State) (Change, error) {
		return s.RecordScore("MS-A1", engine.Score{Sets: []engine.SetScore{{A: 21, B: 20}, {A: 21, B: 3}}})
	})
	g.do(func(s State) (Change, error) { return s.RecordScore("MS-A1", straight) })
	if len(g.state().PendingScores()) != 0 {
		t.Error("pending after score")
	}
	g.now = t0.Add(30 * time.Minute)
	g.do(func(s State) (Change, error) { return s.RecordScore("MS-B1", straight) })
	b1, _ := g.state().Match("MS-B1")
	if !b1.EndedAt.Equal(t0.Add(30*time.Minute)) || g.state().CourtStates()[0].State != CourtFree {
		t.Errorf("B1 = %+v, court 1 = %+v", b1, g.state().CourtStates()[0])
	}

	// Undo the score: the court shows the finished match again.
	g.undo()
	if c := g.state().CourtStates()[0]; c.State != CourtPlaying {
		t.Errorf("after undo court 1 = %+v", c)
	}
}

func TestQueueRestAndProposals(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.SetCourts(2) })
	g.confirmedPools(engine.MensSingles, 1, 4) // 6 matches, 3 rounds of 2

	q := g.state().Queue(t0)
	if len(q) != 6 || !q[0].Ready() || !q[1].Ready() {
		t.Fatalf("queue = %+v", q)
	}
	props := g.state().Proposals(t0)
	if props[1].Match != "MS-A1" || props[2].Match != "MS-A2" {
		t.Fatalf("proposals = %v %v", props[1].Match, props[2].Match)
	}

	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 1) })
	q = g.state().Queue(t0)
	// MS-A2 is still ready; round 2 matches wait for MS-A1's players.
	if q[0].Match.ID != "MS-A2" || !q[0].Ready() || q[1].Reason != ReasonPlaying || q[1].Court != 1 {
		t.Fatalf("queue after call = %+v", q[:2])
	}

	g.now = t0.Add(15 * time.Minute)
	g.do(func(s State) (Change, error) { return s.RecordScore("MS-A1", straight) })
	at := func(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }

	// 5 minutes after the end: resting (min rest 10).
	q = g.state().Queue(at(20))
	if q[1].Reason != ReasonResting || !q[1].RestingSince.Equal(at(15)) {
		t.Fatalf("resting = %+v", q[1])
	}
	if p := g.state().Proposals(at(20)); p[1].Match != "MS-A2" || p[2].Match != "" {
		t.Errorf("proposals while resting = %v / %v", p[1].Match, p[2].Match)
	}
	// Rest is a warning: the desk can still force the call.
	if _, err := g.state().CallMatch(q[1].Match.ID, 2); err != nil {
		t.Errorf("forcing a resting match: %v", err)
	}
	// After the rest, ready again.
	if q = g.state().Queue(at(26)); !q[1].Ready() {
		t.Errorf("after rest = %+v", q[1])
	}
	g.do(func(s State) (Change, error) { return s.SetMinRest(0) })
	if q = g.state().Queue(at(16)); !q[1].Ready() {
		t.Errorf("no minimum rest = %+v", q[1])
	}

	// An absent player.
	m := g.state().Queue(at(26))[1].Match
	e, _ := g.state().Entrant(m.A)
	g.do(func(s State) (Change, error) { return s.CancelCheckIn(e.Players[0].ID) })
	if q = g.state().Queue(at(26)); q[1].Reason != ReasonAbsent {
		t.Errorf("absent = %+v", q[1])
	}
}

func TestWithdrawal(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 4)
	a1, _ := g.state().Match("MS-A1")
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 1) })

	g.do(func(s State) (Change, error) { return s.WithdrawEntrant(a1.A) })
	st := g.state()
	if st.CourtStates()[0].State != CourtFree {
		t.Errorf("court should be free: %+v", st.CourtStates()[0])
	}
	for _, q := range st.Queue(t0) {
		if q.Match.Has(a1.A) {
			t.Errorf("walkover still queued: %v", q.Match.ID)
		}
	}
	g.fails(engine.CodeMatchWalkover, func(s State) (Change, error) { return s.RecordScore("MS-A1", straight) })

	rows := st.PoolStandings(engine.MensSingles, "S1", 0).Rows
	if last := rows[len(rows)-1]; last.Entrant != a1.A || !last.Withdrawn {
		t.Errorf("withdrawn should rank last: %+v", rows)
	}
	if ch := g.do(func(s State) (Change, error) { return s.WithdrawEntrant(a1.A) }); !ch.None() {
		t.Error("double withdrawal appended")
	}
	g.fails(engine.CodeEntrantNotFound, func(s State) (Change, error) { return s.WithdrawEntrant("e999") })
}

func TestCompletePools(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 6) // 2 pools of 3: 3 matches each
	for _, m := range g.state().Matches() {
		if m.Pool == 0 {
			g.do(func(s State) (Change, error) { return s.RecordScore(m.ID, straight) })
		}
	}
	if got := g.state().CompletePools(); len(got) != 1 || got[0] != (PoolRef{engine.MensSingles, "S1", 0}) {
		t.Errorf("complete = %v", got)
	}
}

func TestCourtSettings(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	if s := g.state(); s.Courts() != DefaultCourts || s.MinRest() != DefaultMinRest || s.CourtPriority() != PriorityBalanced {
		t.Fatalf("defaults = %d %d %s", s.Courts(), s.MinRest(), s.CourtPriority())
	}
	g.fails(engine.CodeInvalidCourts, func(s State) (Change, error) { return s.SetCourts(0) })
	g.fails(engine.CodeInvalidCourts, func(s State) (Change, error) { return s.SetCourts(MaxCourts + 1) })
	g.fails(engine.CodeInvalidRest, func(s State) (Change, error) { return s.SetMinRest(-1) })

	g.confirmedPools(engine.MensSingles, 2, 8)
	g.do(func(s State) (Change, error) { return s.CallMatch("MS-A1", 4) })
	g.fails(engine.CodeCourtBusy, func(s State) (Change, error) { return s.SetCourts(3) })
	g.do(func(s State) (Change, error) { return s.SetCourts(6) })
	if len(g.state().CourtStates()) != 6 {
		t.Error("courts not set")
	}
}
