package tournament

import (
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestCheckIn(t *testing.T) {
	g := newGame(t, engine.MensDoubles, engine.WomensSingles)
	pair := g.register(engine.MensDoubles, "Paul", "Hugo")
	single := g.register(engine.WomensSingles, "Léa")
	p1, p2 := g.entrant(pair).Players[0].ID, g.entrant(pair).Players[1].ID
	solo := g.entrant(single).Players[0].ID

	g.do(func(s State) (Change, error) { return s.CheckInPlayer(solo) })
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(p2) })
	if e := g.entrant(pair); e.Bib != 2 || e.Complete() {
		t.Fatalf("pair after first check-in = bib %d complete %v", e.Bib, e.Complete())
	}
	if e := g.entrant(single); e.Bib != 1 || !e.Complete() {
		t.Errorf("single = bib %d complete %v", e.Bib, e.Complete())
	}

	// The partner's check-in keeps the pair's bib.
	ch := g.do(func(s State) (Change, error) { return s.CheckInPlayer(p1) })
	if ch.Data.(PlayerCheckedIn).Bib != 0 {
		t.Errorf("second check-in assigned a bib: %+v", ch)
	}
	if e := g.entrant(pair); e.Bib != 2 || !e.Complete() {
		t.Errorf("pair = bib %d complete %v", e.Bib, e.Complete())
	}

	// Checking in twice changes nothing.
	if ch := g.do(func(s State) (Change, error) { return s.CheckInPlayer(p1) }); !ch.None() {
		t.Errorf("double check-in = %+v", ch)
	}

	if at := g.entrant(pair).Players[0].CheckedInAt; !at.Equal(g.now) {
		t.Errorf("checked in at %v, want %v", at, g.now)
	}

	// Cancelling keeps the bib and clears the time.
	g.do(func(s State) (Change, error) { return s.CancelCheckIn(p1) })
	if e := g.entrant(pair); e.Bib != 2 || e.Complete() || !e.Players[0].CheckedInAt.IsZero() {
		t.Errorf("after cancel = bib %d complete %v at %v", e.Bib, e.Complete(), e.Players[0].CheckedInAt)
	}
	if ch := g.do(func(s State) (Change, error) { return s.CancelCheckIn(p1) }); !ch.None() {
		t.Errorf("double cancel = %+v", ch)
	}

	g.fails(engine.CodePlayerNotFound, func(s State) (Change, error) { return s.CheckInPlayer("p99.1") })
	g.fails(engine.CodePlayerNotFound, func(s State) (Change, error) { return s.CancelCheckIn("p99.1") })
}

func TestBibsAreNeverReused(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	a := g.register(engine.MensSingles, "A")
	b := g.register(engine.MensSingles, "B")
	c := g.register(engine.MensSingles, "C")
	player := func(id engine.EntrantID) PlayerID { return g.entrant(id).Players[0].ID }

	// A gets bib 1, the check-in is undone, B then gets bib 1.
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(player(a)) })
	checkIn := uint64(len(g.log))
	g.undoSeq(checkIn)
	undo := uint64(len(g.log))
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(player(b)) })
	if bib := g.entrant(b).Bib; bib != 1 {
		t.Fatalf("b bib = %d, want 1", bib)
	}

	// Undoing the undo restores A's bib 1: B moves to the next free number.
	g.undoSeq(undo)
	if a, b := g.entrant(a).Bib, g.entrant(b).Bib; a != 1 || b != 2 {
		t.Fatalf("after undo of undo: a=%d b=%d, want 1 and 2", a, b)
	}

	// A removed entrant keeps its number out of circulation.
	g.do(func(s State) (Change, error) { return s.RemoveEntrant(a) })
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(player(c)) })
	if bib := g.entrant(c).Bib; bib != 3 {
		t.Errorf("c bib = %d, want 3", bib)
	}
}

func TestCheckInEntrant(t *testing.T) {
	g := newGame(t, engine.MensDoubles)
	first := g.register(engine.MensDoubles, "Paul", "Hugo")
	pair := g.register(engine.MensDoubles, "Tom", "Max")
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(g.entrant(first).Players[1].ID) })

	// Both players at once, one bib, one event.
	ch := g.do(func(s State) (Change, error) { return s.CheckInEntrant(pair) })
	if p := ch.Data.(EntrantCheckedIn); len(p.Players) != 2 || p.Bib != 2 {
		t.Errorf("payload = %+v", p)
	}
	if e := g.entrant(pair); !e.Complete() || e.Bib != 2 || e.Players[0].CheckedInAt.IsZero() {
		t.Errorf("pair = %+v", e)
	}
	// Only the absent partner of a pair half there; its bib stays.
	ch = g.do(func(s State) (Change, error) { return s.CheckInEntrant(first) })
	if p := ch.Data.(EntrantCheckedIn); len(p.Players) != 1 || p.Bib != 0 {
		t.Errorf("partner only = %+v", p)
	}
	if e := g.entrant(first); !e.Complete() || e.Bib != 1 {
		t.Errorf("first = %+v", e)
	}
	// Everyone there: nothing to do. One undo cancels the whole team.
	if ch, err := g.state().CheckInEntrant(pair); err != nil || !ch.None() {
		t.Errorf("again = %+v, %v", ch, err)
	}
	g.undo()
	if e := g.entrant(first); e.Players[0].CheckedIn || !e.Players[1].CheckedIn {
		t.Errorf("after undo = %+v", e)
	}
	g.fails(engine.CodeEntrantNotFound, func(s State) (Change, error) { return s.CheckInEntrant("e99") })
}
