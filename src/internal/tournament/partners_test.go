package tournament

import (
	"fmt"
	"slices"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestWaitingPlayerHasNoBib(t *testing.T) {
	g := newGame(t, engine.WomensDoubles)
	julie := g.register(engine.WomensDoubles, "Julie")
	pair := g.register(engine.WomensDoubles, "Léa", "Inès")
	if e := g.entrant(julie); !e.Waiting() || g.entrant(pair).Waiting() {
		t.Fatalf("waiting = %v, pair waiting = %v", e.Waiting(), g.entrant(pair).Waiting())
	}
	g.do(func(s State) (Change, error) { return s.CheckInEntrant(julie) })
	e := g.entrant(julie)
	if e.Bib != 0 || !e.Players[0].CheckedIn || e.Complete() {
		t.Errorf("checked in waiting player = %+v, complete %v", e, e.Complete())
	}
	g.do(func(s State) (Change, error) { return s.CancelCheckIn(e.Players[0].ID) })
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(e.Players[0].ID) })
	if g.entrant(julie).Bib != 0 {
		t.Error("CheckInPlayer gave a waiting player a bib")
	}
	g.do(func(s State) (Change, error) { return s.CheckInEntrant(pair) })
	if b := g.entrant(pair).Bib; b != 1 {
		t.Errorf("pair bib = %d, want 1", b)
	}
	if p := g.state().Present(engine.WomensDoubles); len(p) != 1 || p[0].ID != pair {
		t.Errorf("present = %v", p)
	}
	if w := g.state().Waiting(engine.WomensDoubles); len(w) != 1 || w[0].ID != julie {
		t.Errorf("waiting = %v", w)
	}
	// Updating a waiting player keeps one player: partners have their own commands.
	g.do(func(s State) (Change, error) { return s.UpdateEntrant(julie, []PlayerInput{{"Julie Morel", "PLVPB"}}) })
	g.fails(engine.CodeWrongPlayerCount, func(s State) (Change, error) {
		return s.UpdateEntrant(julie, []PlayerInput{{"Julie Morel", ""}, {"Zoé Petit", ""}})
	})
}

func TestPairEntrants(t *testing.T) {
	g := newGame(t, engine.MensDoubles, engine.WomensDoubles)
	julie := g.register(engine.WomensDoubles, "Julie")
	zoe := g.register(engine.WomensDoubles, "Zoé")
	pair := g.register(engine.WomensDoubles, "Léa", "Inès")
	rayane := g.register(engine.MensDoubles, "Rayane")

	tests := []struct {
		name        string
		id, partner engine.EntrantID
		code        engine.Code
	}{
		{"unknown entrant", "e99", zoe, engine.CodeEntrantNotFound},
		{"unknown partner", julie, "e99", engine.CodeEntrantNotFound},
		{"with itself", julie, julie, engine.CodeNotWaitingPartner},
		{"with a pair", julie, pair, engine.CodeNotWaitingPartner},
		{"across categories", julie, rayane, engine.CodeNotSameCategory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g.fails(tt.code, func(s State) (Change, error) { return s.PairEntrants(tt.id, tt.partner) })
		})
	}

	// Zoé is present: the pair gets the next bib once formed.
	g.do(func(s State) (Change, error) { return s.CheckInEntrant(pair) })
	g.do(func(s State) (Change, error) { return s.CheckInEntrant(zoe) })
	seq := g.do(func(s State) (Change, error) { return s.PairEntrants(julie, zoe) })
	if p := seq.Data.(EntrantsPaired); p.Bib != 2 || len(p.Players) != 2 {
		t.Errorf("payload = %+v", p)
	}
	e := g.entrant(julie)
	if e.Waiting() || len(e.Players) != 2 || e.Bib != 2 || e.Players[0].CheckedIn || !e.Players[1].CheckedIn {
		t.Errorf("pair = %+v", e)
	}
	if _, ok := g.state().Entrant(zoe); ok {
		t.Error("partner entrant still there")
	}
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(e.Players[0].ID) })
	if p := g.state().Present(engine.WomensDoubles); len(p) != 2 {
		t.Errorf("present after pairing = %d", len(p))
	}

	// One undo cancels the pairing (after the check-in that followed).
	g.undo()
	g.undo()
	if a, b := g.entrant(julie), g.entrant(zoe); !a.Waiting() || !b.Waiting() || a.Bib != 0 || b.Bib != 0 {
		t.Errorf("after undo = %+v, %+v", a, b)
	}
	// A pair formed with nobody present has no bib yet.
	hugo := g.register(engine.MensDoubles, "Hugo")
	ch := g.do(func(s State) (Change, error) { return s.PairEntrants(rayane, hugo) })
	if ch.Data.(EntrantsPaired).Bib != 0 || g.entrant(rayane).Bib != 0 {
		t.Errorf("bib without anyone present = %+v", ch.Data)
	}
	g.do(func(s State) (Change, error) { return s.CheckInPlayer(g.entrant(rayane).Players[1].ID) })
	// The undone pairing freed number 2, like an undone check-in.
	if b := g.entrant(rayane).Bib; b != 2 {
		t.Errorf("bib at first check-in = %d, want 2", b)
	}
}

func TestAddPartner(t *testing.T) {
	g := newGame(t, engine.WomensDoubles, engine.MensSingles)
	julie := g.register(engine.WomensDoubles, "Julie")
	pair := g.register(engine.WomensDoubles, "Léa", "Inès")
	single := g.register(engine.MensSingles, "Paul")

	tests := []struct {
		name string
		id   engine.EntrantID
		in   PlayerInput
		code engine.Code
		set  int
	}{
		{"unknown", "e99", PlayerInput{"Zoé Petit", ""}, engine.CodeEntrantNotFound, 0},
		{"a pair", pair, PlayerInput{"Zoé Petit", ""}, engine.CodeNotWaitingPartner, 0},
		{"singles", single, PlayerInput{"Zoé Petit", ""}, engine.CodeNotWaitingPartner, 0},
		{"no name", julie, PlayerInput{"  ", ""}, engine.CodeInvalidPlayerName, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := g.state().AddPartner(tt.id, tt.in)
			e, ok := err.(*engine.Error)
			if !ok || e.Code != tt.code || e.Set != tt.set {
				t.Fatalf("error = %v, want %s set %d", err, tt.code, tt.set)
			}
		})
	}

	g.do(func(s State) (Change, error) { return s.CheckInEntrant(julie) })
	n := len(g.log) + 1
	g.do(func(s State) (Change, error) { return s.AddPartner(julie, PlayerInput{"  Zoé   Petit ", "PLVPB"}) })
	e := g.entrant(julie)
	if len(e.Players) != 2 || e.Players[1].Name != "Zoé Petit" || e.Players[1].ID != PlayerID(fmt.Sprintf("p%d.1", n)) ||
		e.Players[1].CheckedIn || e.Bib != 1 {
		t.Errorf("after partner added = %+v", e)
	}
	g.undo()
	if e := g.entrant(julie); !e.Waiting() || e.Bib != 0 {
		t.Errorf("after undo = %+v", e)
	}
}

func TestSplitEntrant(t *testing.T) {
	g := newGame(t, engine.WomensDoubles, engine.MensSingles)
	pair := g.register(engine.WomensDoubles, "Léa", "Inès")
	julie := g.register(engine.WomensDoubles, "Julie")
	single := g.register(engine.MensSingles, "Paul")
	ines := g.entrant(pair).Players[1].ID

	tests := []struct {
		name   string
		id     engine.EntrantID
		player PlayerID
		code   engine.Code
	}{
		{"unknown", "e99", ines, engine.CodeEntrantNotFound},
		{"waiting player", julie, g.entrant(julie).Players[0].ID, engine.CodeNotAPair},
		{"singles", single, g.entrant(single).Players[0].ID, engine.CodeNotAPair},
		{"player of another entrant", pair, g.entrant(julie).Players[0].ID, engine.CodePlayerNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g.fails(tt.code, func(s State) (Change, error) { return s.SplitEntrant(tt.id, tt.player) })
		})
	}

	// Draw the pair with two others, seeded, then split it before confirming.
	g.do(func(s State) (Change, error) { return s.CheckInEntrant(pair) })
	for _, names := range [][]string{{"Ana", "Eva"}, {"Mia", "Noa"}} {
		id := g.register(engine.WomensDoubles, names...)
		g.do(func(s State) (Change, error) { return s.CheckInEntrant(id) })
	}
	g.editStage(engine.WomensDoubles, "S1", func(st *StageConfig) { st.Pools = 1 })
	g.do(func(s State) (Change, error) { return s.SetSeeds(engine.WomensDoubles, []engine.EntrantID{pair}) })
	g.do(func(s State) (Change, error) { return s.DrawEntry(engine.WomensDoubles, rng()) })

	n := len(g.log) + 1
	g.do(func(s State) (Change, error) { return s.SplitEntrant(pair, ines) })
	st := g.state()
	lea, _ := st.Entrant(pair)
	newID := engine.EntrantID(fmt.Sprintf("e%d", n))
	alone, ok := st.Entrant(newID)
	if !ok || !alone.Waiting() || alone.Players[0].ID != ines || !alone.Players[0].CheckedIn || alone.Bib != 0 {
		t.Errorf("split player = %+v", alone)
	}
	if !lea.Waiting() || lea.Bib != 1 || lea.Seed != 0 {
		t.Errorf("remaining player = %+v", lea)
	}
	if d, _ := st.Draw(engine.WomensDoubles); slices.Contains(d.Pools[0], pair) {
		t.Errorf("split pair still drawn: %v", d.Pools)
	}

	// Paired again with Julie: the pair keeps Léa's bib.
	g.do(func(s State) (Change, error) { return s.PairEntrants(julie, pair) })
	if e := g.entrant(julie); e.Bib != 1 || len(e.Players) != 2 {
		t.Errorf("re-paired = %+v", e)
	}
	g.undo()
	g.undo()
	if e := g.entrant(pair); len(e.Players) != 2 || e.Bib != 1 || e.Seed != 1 {
		t.Errorf("after undoing the split = %+v", e)
	}

	// Once its matches exist, a pair is not split: declare a forfeit.
	g.do(func(s State) (Change, error) { return s.ConfirmDraw(engine.WomensDoubles) })
	g.fails(engine.CodeEntrantInPools, func(s State) (Change, error) { return s.SplitEntrant(pair, ines) })
}

func TestExpectedEntrantsCountsWaitingPairs(t *testing.T) {
	g := newGame(t, engine.WomensDoubles)
	g.register(engine.WomensDoubles, "Léa", "Inès")
	g.register(engine.WomensDoubles, "Julie")
	g.register(engine.WomensDoubles, "Zoé")
	g.register(engine.WomensDoubles, "Ana")
	if n := g.state().expectedEntrants(engine.WomensDoubles); n != 2 {
		t.Errorf("expected entrants = %d, want 2", n)
	}
}
