package tournament

import (
	"slices"
	"strconv"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

var sets21 = engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 30}

func idNumber(id engine.EntrantID) int {
	n, _ := strconv.Atoi(string(id[1:]))
	return n
}

// finishPools scores every pool match: the entrant registered first wins, so
// standings follow registration order without ties.
func (g *game) finishPools(c engine.Category) {
	g.t.Helper()
	for _, m := range g.state().Matches() {
		if m.Category != c || m.Bracket != "" || g.state().Decided(m) {
			continue
		}
		score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
		if idNumber(m.B) < idNumber(m.A) {
			score = engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
		}
		g.do(func(s State) (Change, error) { return s.RecordScore(m.ID, score) })
	}
}

func (g *game) bracketMatches(c engine.Category, id string) []Match {
	var out []Match
	for _, m := range g.state().Matches() {
		if m.Category == c && m.Bracket == id {
			out = append(out, m)
		}
	}
	return out
}

// win scores a bracket match for the given side (1 set of 21).
func (g *game) win(id MatchID, side engine.Side) {
	g.t.Helper()
	score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 15}}}
	if side == engine.SideB {
		score = engine.Score{Sets: []engine.SetScore{{A: 15, B: 21}}}
	}
	g.do(func(s State) (Change, error) { return s.RecordScore(id, score) })
}

func TestBracketFormat(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	cfg := BracketConfig{Name: " Tableau  principal ", Win: sets21, FullPlacement: true}
	ch := g.do(func(s State) (Change, error) { return s.AddBracket(engine.MensSingles, cfg) })
	if ch.Data.(BracketAdded).ID != "T1" || ch.Data.(BracketAdded).Name != "Tableau principal" {
		t.Fatalf("added = %+v", ch.Data)
	}
	g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "Consolante", Win: sets21})
	})
	g.fails(engine.CodeInvalidName, func(s State) (Change, error) { return s.AddBracket(engine.MensSingles, BracketConfig{Win: sets21}) })
	g.fails(engine.CodeInvalidWinCondition, func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "X"})
	})

	g.do(func(s State) (Change, error) {
		return s.SetRouting(engine.MensSingles, []string{"T1", "T1", "T2", "", ""})
	})
	if got := g.state().Routing(engine.MensSingles); !slices.Equal(got, []string{"T1", "T1", "T2"}) {
		t.Errorf("routing = %v (trailing eliminations trimmed)", got)
	}
	g.fails(engine.CodeInvalidRouting, func(s State) (Change, error) { return s.SetRouting(engine.MensSingles, []string{"T9"}) })

	// Removing T2 clears its routing; numbers are never reused.
	g.do(func(s State) (Change, error) { return s.RemoveBracket(engine.MensSingles, "T2") })
	if got := g.state().Routing(engine.MensSingles); !slices.Equal(got, []string{"T1", "T1", ""}) {
		t.Errorf("routing after removal = %v", got)
	}
	ch = g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "Bis", Win: sets21})
	})
	if ch.Data.(BracketAdded).ID != "T3" {
		t.Errorf("new id = %s, want T3", ch.Data.(BracketAdded).ID)
	}
	g.do(func(s State) (Change, error) {
		return s.ConfigureBracket(engine.MensSingles, "T1", BracketConfig{Name: "Principal", Win: sets21})
	})
	if b, _ := g.state().Bracket(engine.MensSingles, "T1"); b.Name != "Principal" || b.FullPlacement {
		t.Errorf("configured = %+v", b)
	}
}

func TestBracketFullPlacement(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "Principal", Win: sets21, FullPlacement: true})
	})
	g.do(func(s State) (Change, error) { return s.SetRouting(engine.MensSingles, []string{"T1", "T1"}) })
	g.fails(engine.CodePoolsNotComplete, func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })

	g.finishPools(engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })

	ms := g.bracketMatches(engine.MensSingles, "T1")
	if len(ms) != 4 {
		t.Fatalf("%d matches, want 2 semis + final + 3rd place", len(ms))
	}
	semi1, semi2, final, third := ms[0], ms[1], ms[2], ms[3]
	if !g.state().Playable(semi1) || g.state().Playable(final) || final.Start != 1 || third.Start != 3 {
		t.Fatalf("matches = %+v", ms)
	}
	// Pool mates never meet in the semi-finals.
	for _, m := range []Match{semi1, semi2} {
		if poolOfEntrant(g.state(), m.A) == poolOfEntrant(g.state(), m.B) {
			t.Errorf("pool mates in %s", m.ID)
		}
	}

	g.win(semi1.ID, engine.SideA)
	g.win(semi2.ID, engine.SideB)
	ms = g.bracketMatches(engine.MensSingles, "T1")
	final, third = ms[2], ms[3]
	if final.A != semi1.A || final.B != semi2.B || third.A != semi1.B || third.B != semi2.A {
		t.Fatalf("final %s-%s, third %s-%s", final.A, final.B, third.A, third.B)
	}

	// Once the final started, the semi result can no longer flip.
	g.do(func(s State) (Change, error) { return s.CallMatch(final.ID, 1) })
	g.fails(engine.CodeBracketNextStarted, func(s State) (Change, error) {
		return s.RecordScore(semi1.ID, engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}}})
	})
	g.fails(engine.CodeBracketNextStarted, func(s State) (Change, error) { return s.ClearScore(semi1.ID) })
	// The same winner with another score is fine.
	g.do(func(s State) (Change, error) {
		return s.RecordScore(semi1.ID, engine.Score{Sets: []engine.SetScore{{A: 22, B: 20}}})
	})
	g.fails(engine.CodeBracketStarted, func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })
	g.fails(engine.CodeBracketStarted, func(s State) (Change, error) { return s.SwapBracketSlots(engine.MensSingles, "T1", 0, 1) })

	g.win(final.ID, engine.SideB)
	g.win(third.ID, engine.SideA)
	places := g.state().Places(engine.MensSingles, "T1")
	want := []Place{{1, 1, semi2.B}, {2, 2, semi1.A}, {3, 3, semi1.B}, {4, 4, semi2.A}}
	if !slices.Equal(places, want) {
		t.Errorf("places = %+v, want %+v", places, want)
	}
}

func poolOfEntrant(s State, id engine.EntrantID) int {
	d, _ := s.Draw(engine.MensSingles)
	for p, pool := range d.Pools {
		if slices.Contains(pool, id) {
			return p
		}
	}
	return -1
}

func TestBracketByesAndTiedPlaces(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 3, 9) // 3 pools of 3
	g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "Top", Win: sets21})
	})
	g.do(func(s State) (Change, error) { return s.SetRouting(engine.MensSingles, []string{"T1"}) })
	g.finishPools(engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })

	b, _ := g.state().Bracket(engine.MensSingles, "T1")
	if len(b.Slots) != 4 || b.Slots[1] != "" {
		t.Fatalf("slots = %v (the top seed gets the bye)", b.Slots)
	}
	ms := g.bracketMatches(engine.MensSingles, "T1")
	// Semi 1 is a bye: never queued, and the top seed is already in the final.
	if !ms[0].ByeB || g.state().Playable(ms[0]) || ms[2].A != b.Slots[0] || ms[2].B != "" {
		t.Fatalf("matches = %+v", ms)
	}
	for _, q := range g.state().Queue(t0) {
		if q.Match.ID == ms[0].ID || q.Match.ID == ms[2].ID {
			t.Errorf("%s should not be queued", q.Match.ID)
		}
	}
	g.win(ms[1].ID, engine.SideA)
	g.win(ms[2].ID, engine.SideA)
	places := g.state().Places(engine.MensSingles, "T1")
	want := []Place{{1, 1, b.Slots[0]}, {2, 2, b.Slots[2]}, {3, 4, b.Slots[3]}}
	if !slices.Equal(places, want) {
		t.Errorf("places = %+v, want %+v", places, want)
	}
}

func TestBracketForfeitKeepsPoolResults(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "P", Win: sets21})
	})
	g.do(func(s State) (Change, error) { return s.SetRouting(engine.MensSingles, []string{"T1", "T1"}) })
	g.finishPools(engine.MensSingles)
	g.do(func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })

	before := g.state().PoolStandings(engine.MensSingles, 0)
	semi := g.bracketMatches(engine.MensSingles, "T1")[0]
	g.do(func(s State) (Change, error) { return s.WithdrawEntrant(semi.A) })
	after := g.state().PoolStandings(engine.MensSingles, 0)
	if !slices.Equal(before.Rows, after.Rows) {
		t.Errorf("pool standings changed by a bracket forfeit")
	}
	ms := g.bracketMatches(engine.MensSingles, "T1")
	if !g.state().Walkover(ms[0]) || ms[2].A != semi.B {
		t.Errorf("opponent should advance: %+v", ms[2])
	}
}

func TestDrawBracketPreconditions(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 1, 3)
	g.do(func(s State) (Change, error) {
		return s.AddBracket(engine.MensSingles, BracketConfig{Name: "P", Win: sets21})
	})
	// A perfect three-way tie in the pool.
	for _, m := range g.state().Matches() {
		score := engine.Score{Sets: []engine.SetScore{{A: 21, B: 10}, {A: 21, B: 10}}}
		if m.ID == "MS-A2" {
			score = engine.Score{Sets: []engine.SetScore{{A: 10, B: 21}, {A: 10, B: 21}}}
		}
		g.do(func(s State) (Change, error) { return s.RecordScore(m.ID, score) })
	}
	g.do(func(s State) (Change, error) { return s.SetRouting(engine.MensSingles, []string{"T1"}) })
	g.fails(engine.CodeTieUnresolved, func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })
	g.do(func(s State) (Change, error) { return s.DrawLots(engine.MensSingles, 0, rng()) })
	// Only the pool winner is routed: not enough for a bracket.
	g.fails(engine.CodeNotEnoughEntrants, func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })
	g.do(func(s State) (Change, error) { return s.SetRouting(engine.MensSingles, []string{"T1", "T1"}) })
	g.do(func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })
	g.do(func(s State) (Change, error) { return s.SwapBracketSlots(engine.MensSingles, "T1", 0, 1) })
	g.fails(engine.CodeInvalidSlot, func(s State) (Change, error) { return s.SwapBracketSlots(engine.MensSingles, "T1", 0, 5) })
}

func TestBracketMatchesFollowFormatOrder(t *testing.T) {
	g := newGame(t, engine.MensSingles)
	g.confirmedPools(engine.MensSingles, 2, 8)
	for _, name := range []string{"Main", "Consolation"} {
		g.do(func(s State) (Change, error) {
			return s.AddBracket(engine.MensSingles, BracketConfig{Name: name, Win: sets21})
		})
	}
	g.do(func(s State) (Change, error) {
		return s.SetRouting(engine.MensSingles, []string{"T1", "T1", "T2", "T2"})
	})
	g.finishPools(engine.MensSingles)
	// Filled in reverse order: the main bracket still comes first.
	g.do(func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T2") })
	g.do(func(s State) (Change, error) { return s.DrawBracket(engine.MensSingles, "T1") })
	var order []string
	for _, m := range g.state().Matches() {
		if m.Bracket != "" && (len(order) == 0 || order[len(order)-1] != m.Bracket) {
			order = append(order, m.Bracket)
		}
	}
	if !slices.Equal(order, []string{"T1", "T2"}) {
		t.Errorf("bracket order = %v", order)
	}
}
