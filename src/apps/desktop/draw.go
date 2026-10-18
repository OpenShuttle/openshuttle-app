package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// Draw statuses.
const (
	DrawNone      = "none"
	DrawDraft     = "draft"
	DrawConfirmed = "confirmed"
)

// FormatCategory is one category of the format screen.
type FormatCategory struct {
	CategoryRef
	Stage tournament.PoolStage `json:"stage"`
	// TimedAllowed is false with two categories: timed rounds need all courts.
	TimedAllowed bool `json:"timedAllowed"`
	Registered   int  `json:"registered"`
	Present      int  `json:"present"`
	// Sizes and Matches preview the pools for the registered entrants.
	Sizes   []int `json:"sizes"`
	Matches int   `json:"matches"`
	// PoolsLocked is set once the draw is confirmed.
	PoolsLocked bool          `json:"poolsLocked"`
	Brackets    []BracketView `json:"brackets"`
	// Routing[i] is the bracket of pool position i+1, "" for eliminated;
	// RoutingRows is how many positions to show (the largest pool).
	Routing     []string `json:"routing"`
	RoutingRows int      `json:"routingRows"`
	// PoolRounds is the number of rounds of the largest pool.
	PoolRounds int `json:"poolRounds"`
}

// FormatView is the format screen.
type FormatView struct {
	Categories []FormatCategory `json:"categories"`
}

// DrawEntrantView is a card of the draw screen.
type DrawEntrantView struct {
	ID      engine.EntrantID `json:"id"`
	Bib     int              `json:"bib"`
	Seed    int              `json:"seed"`
	Players []PlayerView     `json:"players"`
	Clubs   []string         `json:"clubs"`
	// SameClub lists the bibs of pool mates sharing a club.
	SameClub []int `json:"sameClub"`
	// Absent is set when a drawn entrant is no longer checked in.
	Absent bool `json:"absent"`
}

// DrawPoolView is a column of the draw screen.
type DrawPoolView struct {
	Name       string            `json:"name"`
	Entrants   []DrawEntrantView `json:"entrants"`
	Unbalanced bool              `json:"unbalanced"`
	Seeds      int               `json:"seeds"`
}

// DrawCategory is one category of the draw screen.
type DrawCategory struct {
	CategoryRef
	Status  string         `json:"status"`
	Pools   []DrawPoolView `json:"pools"`
	Planned int            `json:"planned"` // pools in the format
	Present int            `json:"present"`
	// CanDraw is false when there are fewer than two present entrants per pool.
	CanDraw bool `json:"canDraw"`
	// Unplaced are present entrants missing from the draw.
	Unplaced []DrawEntrantView `json:"unplaced"`
	// Seeded lists every present entrant with its seed, to set seeds before drawing.
	Seeded []DrawEntrantView `json:"seeded"`
}

// DrawView is the draw screen.
type DrawView struct {
	Categories []DrawCategory `json:"categories"`
}

// ConfigurePoolStage sets the pool stage of a category.
func (s *TournamentService) ConfigurePoolStage(ctx context.Context, c engine.Category, stage tournament.PoolStage) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ConfigurePoolStage(c, stage) })
}

// SetSeed seeds an entrant, or clears its seed with 0.
func (s *TournamentService) SetSeed(ctx context.Context, id engine.EntrantID, seed int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetSeed(id, seed) })
}

// DrawPools draws (or redraws) the pools of a category.
func (s *TournamentService) DrawPools(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.DrawPools(c, s.rng()) })
}

// MoveEntrant moves an entrant to another pool (0 is A).
func (s *TournamentService) MoveEntrant(ctx context.Context, id engine.EntrantID, pool int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.MoveEntrant(id, pool) })
}

// ConfirmDraw locks the pools of a category.
func (s *TournamentService) ConfirmDraw(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ConfirmDraw(c) })
}

// ReopenDraw unlocks a confirmed draw.
func (s *TournamentService) ReopenDraw(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ReopenDraw(c) })
}

// GetFormatView returns the format screen.
func (s *TournamentService) GetFormatView() FormatView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	v := FormatView{Categories: []FormatCategory{}}
	for _, ref := range categoryRefs(st) {
		stage := st.PoolStage(ref.Category)
		fc := FormatCategory{
			CategoryRef:  ref,
			Stage:        stage,
			TimedAllowed: len(st.Categories) == 1,
			Present:      len(st.Present(ref.Category)),
		}
		for _, e := range st.Entrants {
			if e.Category == ref.Category {
				fc.Registered++
			}
		}
		fc.Sizes = engine.PoolSizes(fc.Registered, stage.Pools)
		for _, size := range fc.Sizes {
			fc.Matches += engine.RoundRobinMatches(size)
		}
		if d, ok := st.Draw(ref.Category); ok && d.Confirmed {
			fc.PoolsLocked = true
		}
		fc.Routing = st.Routing(ref.Category)
		if fc.Routing == nil {
			fc.Routing = []string{}
		}
		sizes := fc.Sizes
		if d, ok := st.Draw(ref.Category); ok {
			sizes = nil
			for _, pool := range d.Pools {
				sizes = append(sizes, len(pool))
			}
		}
		for _, size := range sizes {
			fc.PoolRounds = max(fc.PoolRounds, engine.RoundRobinRounds(size))
		}
		fc.Brackets = bracketPreviews(st.Brackets(ref.Category), fc.Routing, sizes)
		fc.RoutingRows = max(len(fc.Routing), 2)
		if len(fc.Sizes) > 0 {
			fc.RoutingRows = max(fc.RoutingRows, fc.Sizes[0])
		}
		if d, ok := st.Draw(ref.Category); ok {
			for _, pool := range d.Pools {
				fc.RoutingRows = max(fc.RoutingRows, len(pool))
			}
		}
		v.Categories = append(v.Categories, fc)
	}
	return v
}

// GetDrawView returns the draw screen.
func (s *TournamentService) GetDrawView() DrawView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	dup := st.DuplicatePlayers()
	card := func(id engine.EntrantID) DrawEntrantView {
		e, _ := st.Entrant(id)
		v := DrawEntrantView{ID: id, Bib: e.Bib, Seed: e.Seed, Clubs: e.Clubs(), SameClub: []int{}, Absent: !e.Complete()}
		for _, p := range e.Players {
			v.Players = append(v.Players, playerView(p, dup[p.ID]))
		}
		if v.Clubs == nil {
			v.Clubs = []string{}
		}
		return v
	}

	v := DrawView{Categories: []DrawCategory{}}
	for _, ref := range categoryRefs(st) {
		present := st.Present(ref.Category)
		planned := st.PoolStage(ref.Category).Pools
		dc := DrawCategory{
			CategoryRef: ref, Status: DrawNone, Planned: planned, Present: len(present),
			CanDraw: len(present) >= 2*planned, Pools: []DrawPoolView{}, Unplaced: []DrawEntrantView{},
			Seeded: []DrawEntrantView{},
		}
		for _, e := range present {
			dc.Seeded = append(dc.Seeded, card(e.ID))
		}
		if d, ok := st.Draw(ref.Category); ok {
			dc.Status = DrawDraft
			if d.Confirmed {
				dc.Status = DrawConfirmed
			}
			check := st.CheckDraw(ref.Category)
			for i, pool := range d.Pools {
				pv := DrawPoolView{
					Name: engine.PoolName(i), Entrants: []DrawEntrantView{},
					Unbalanced: check.Pools[i].Unbalanced, Seeds: check.Pools[i].Seeds,
				}
				for _, id := range pool {
					c := card(id)
					for _, mate := range check.Pools[i].SameClub[id] {
						if m, ok := st.Entrant(mate); ok {
							c.SameClub = append(c.SameClub, m.Bib)
						}
					}
					pv.Entrants = append(pv.Entrants, c)
				}
				dc.Pools = append(dc.Pools, pv)
			}
			for _, id := range check.Unplaced {
				dc.Unplaced = append(dc.Unplaced, card(id))
			}
		}
		v.Categories = append(v.Categories, dc)
	}
	return v
}

// cryptoRand returns a generator seeded from the OS. The seed is not stored:
// the draw event records the resulting pools.
func cryptoRand() *rand.Rand {
	var b [16]byte
	_, _ = cryptorand.Read(b[:])
	return rand.New(rand.NewPCG(binary.LittleEndian.Uint64(b[:8]), binary.LittleEndian.Uint64(b[8:])))
}

// bracketPreviews sums up each bracket from the pool sizes and the routing.
// Places follow the order of the pool positions brackets take, as in the
// full standings.
func bracketPreviews(brackets []tournament.Bracket, routing []string, sizes []int) []BracketView {
	entrants := map[string]int{}
	var order []string
	for i, id := range routing {
		if id == "" {
			continue
		}
		if _, seen := entrants[id]; !seen {
			order = append(order, id)
		}
		for _, size := range sizes {
			if size > i {
				entrants[id]++
			}
		}
	}
	places := map[string][2]int{}
	next := 1
	for _, id := range order {
		places[id] = [2]int{next, next + entrants[id] - 1}
		next += entrants[id]
	}
	out := []BracketView{}
	for _, b := range brackets {
		n := entrants[b.ID]
		v := BracketView{ID: b.ID, BracketConfig: b.BracketConfig, Drawn: b.Drawn(), Entrants: n}
		if n > 0 {
			v.Byes = engine.NextPowerOfTwo(n) - n
			v.Matches = engine.BracketMatches(n, b.FullPlacement)
			v.Rounds = engine.BracketRounds(n)
			v.PlaceFrom, v.PlaceTo = places[b.ID][0], places[b.ID][1]
		}
		out = append(out, v)
	}
	return out
}
