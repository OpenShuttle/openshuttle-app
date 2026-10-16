package tournament

import (
	"math/rand/v2"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeEntrantSeeded events.Type = "EntrantSeeded"
	TypePoolsDrawn    events.Type = "PoolsDrawn"
	TypeEntrantMoved  events.Type = "EntrantMoved"
	TypeDrawConfirmed events.Type = "DrawConfirmed"
	TypeDrawReopened  events.Type = "DrawReopened"
)

// MaxSeed is the highest seed number.
const MaxSeed = 32

// Draw is the pools of a category, draft until confirmed.
type Draw struct {
	Pools     [][]engine.EntrantID
	Confirmed bool
}

// EntrantSeeded is the payload of TypeEntrantSeeded. Seed 0 clears the seed.
// A seed number is unique within a category: giving it to an entrant takes
// it from the one that had it.
type EntrantSeeded struct {
	ID   engine.EntrantID `json:"id"`
	Seed int              `json:"seed"`
}

// PoolsDrawn is the payload of TypePoolsDrawn: the resulting pools, so replay
// does not depend on randomness.
type PoolsDrawn struct {
	Category engine.Category      `json:"category"`
	Pools    [][]engine.EntrantID `json:"pools"`
}

// EntrantMoved is the payload of TypeEntrantMoved: the entrant goes to the
// end of pool Pool (0 is A).
type EntrantMoved struct {
	Category engine.Category  `json:"category"`
	Entrant  engine.EntrantID `json:"entrant"`
	Pool     int              `json:"pool"`
}

// DrawConfirmed is the payload of TypeDrawConfirmed and TypeDrawReopened.
type DrawConfirmed struct {
	Category engine.Category `json:"category"`
}

// Draw returns the draw of a category, if any.
func (s State) Draw(c engine.Category) (Draw, bool) {
	d := s.draws[c]
	if d == nil {
		return Draw{}, false
	}
	return *d, true
}

// Present returns the entrants of a category whose players are all checked
// in, in registration order. Only they are drawn.
func (s State) Present(c engine.Category) []Entrant {
	var out []Entrant
	for _, e := range s.Entrants {
		if e.Category == c && e.Complete() {
			out = append(out, e)
		}
	}
	return out
}

// SetSeed seeds an entrant (1 is the top seed) or clears its seed (0).
func (s State) SetSeed(id engine.EntrantID, seed int) (Change, error) {
	e, ok := s.Entrant(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	if seed < 0 || seed > MaxSeed {
		return Change{}, engine.NewError(engine.CodeInvalidSeed)
	}
	if e.Seed == seed {
		return Change{}, nil
	}
	return Change{TypeEntrantSeeded, EntrantSeeded{ID: id, Seed: seed}}, nil
}

// DrawPools draws the pools of a category among present entrants. rng is
// seeded by the caller; the event stores the result.
func (s State) DrawPools(c engine.Category, rng *rand.Rand) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	if d := s.draws[c]; d != nil && d.Confirmed {
		return Change{}, engine.NewError(engine.CodeDrawConfirmed)
	}
	pools := s.PoolStage(c).Pools
	present := s.Present(c)
	if len(present) < 2*pools {
		// A pool of one entrant has no match to play.
		return Change{}, engine.NewError(engine.CodeNotEnoughEntrants)
	}
	in := make([]engine.DrawEntrant, len(present))
	for i, e := range present {
		in[i] = engine.DrawEntrant{ID: e.ID, Seed: e.Seed, Clubs: e.Clubs()}
	}
	return Change{TypePoolsDrawn, PoolsDrawn{Category: c, Pools: engine.DrawPools(in, pools, rng)}}, nil
}

// MoveEntrant moves an entrant to another pool of the draft draw. An entrant
// checked in after the draw can be placed this way too.
func (s State) MoveEntrant(id engine.EntrantID, pool int) (Change, error) {
	e, ok := s.Entrant(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	d := s.draws[e.Category]
	switch {
	case d == nil:
		return Change{}, engine.NewError(engine.CodeNoDraw)
	case d.Confirmed:
		return Change{}, engine.NewError(engine.CodeDrawConfirmed)
	case pool < 0 || pool >= len(d.Pools):
		return Change{}, engine.NewError(engine.CodeInvalidPool)
	}
	from := d.poolOf(id)
	if from < 0 && !e.Complete() {
		return Change{}, engine.NewError(engine.CodeEntrantNotInDraw)
	}
	if from == pool {
		return Change{}, nil
	}
	return Change{TypeEntrantMoved, EntrantMoved{Category: e.Category, Entrant: id, Pool: pool}}, nil
}

// ConfirmDraw locks the pools of a category.
func (s State) ConfirmDraw(c engine.Category) (Change, error) {
	d := s.draws[c]
	if d == nil {
		return Change{}, engine.NewError(engine.CodeNoDraw)
	}
	if d.Confirmed {
		return Change{}, nil
	}
	return Change{TypeDrawConfirmed, DrawConfirmed{Category: c}}, nil
}

// ReopenDraw unlocks a confirmed draw.
func (s State) ReopenDraw(c engine.Category) (Change, error) {
	d := s.draws[c]
	if d == nil {
		return Change{}, engine.NewError(engine.CodeNoDraw)
	}
	if !d.Confirmed {
		return Change{}, nil
	}
	if s.MatchesStarted(c) {
		return Change{}, engine.NewError(engine.CodeMatchesStarted)
	}
	return Change{TypeDrawReopened, DrawConfirmed{Category: c}}, nil
}

// Clubs returns the distinct clubs of an entrant's players.
func (e Entrant) Clubs() []string {
	var clubs []string
	for _, p := range e.Players {
		if p.Club != "" && !slices.Contains(clubs, p.Club) {
			clubs = append(clubs, p.Club)
		}
	}
	return clubs
}

func (d *Draw) poolOf(id engine.EntrantID) int {
	for i, pool := range d.Pools {
		if slices.Contains(pool, id) {
			return i
		}
	}
	return -1
}

func (d *Draw) remove(id engine.EntrantID) {
	for i := range d.Pools {
		d.Pools[i] = slices.DeleteFunc(d.Pools[i], func(x engine.EntrantID) bool { return x == id })
	}
}

func (s *State) applyEntrantSeeded(p EntrantSeeded) {
	i := s.entrantIndex(p.ID)
	if i < 0 {
		return
	}
	if p.Seed != 0 {
		for j := range s.Entrants {
			if s.Entrants[j].Category == s.Entrants[i].Category && s.Entrants[j].Seed == p.Seed {
				s.Entrants[j].Seed = 0
			}
		}
	}
	s.Entrants[i].Seed = p.Seed
}

func (s *State) applyPoolsDrawn(p PoolsDrawn) {
	if s.draws == nil {
		s.draws = map[engine.Category]*Draw{}
	}
	pools := make([][]engine.EntrantID, len(p.Pools))
	for i, pool := range p.Pools {
		pools[i] = slices.Clone(pool)
	}
	s.draws[p.Category] = &Draw{Pools: pools}
}

func (s *State) applyEntrantMoved(p EntrantMoved) {
	d := s.draws[p.Category]
	if d == nil || p.Pool < 0 || p.Pool >= len(d.Pools) {
		return
	}
	d.remove(p.Entrant)
	d.Pools[p.Pool] = append(d.Pools[p.Pool], p.Entrant)
}

func (s *State) setConfirmed(c engine.Category, confirmed bool) {
	if d := s.draws[c]; d != nil {
		d.Confirmed = confirmed
	}
}

// PoolCheck lists what may be wrong with one pool. Warnings only: the desk
// decides.
type PoolCheck struct {
	// Unbalanced is set when the pool size differs from the even split.
	Unbalanced bool
	// Seeds is the number of seeded entrants in the pool.
	Seeds int
	// SameClub maps an entrant to the others of the pool sharing a club.
	SameClub map[engine.EntrantID][]engine.EntrantID
}

// DrawCheck is the review of a draw before it is confirmed.
type DrawCheck struct {
	Pools []PoolCheck
	// Unplaced are present entrants missing from the draw (checked in late).
	Unplaced []engine.EntrantID
	// Absent are drawn entrants no longer complete (a check-in was cancelled).
	Absent []engine.EntrantID
}

// CheckDraw reviews the draw of a category.
func (s State) CheckDraw(c engine.Category) DrawCheck {
	d := s.draws[c]
	if d == nil {
		return DrawCheck{}
	}
	placed := 0
	for _, pool := range d.Pools {
		placed += len(pool)
	}
	sizes := slices.Clone(engine.PoolSizes(placed, len(d.Pools)))
	minSize, maxSize := slices.Min(sizes), slices.Max(sizes)

	check := DrawCheck{Pools: make([]PoolCheck, len(d.Pools))}
	for i, pool := range d.Pools {
		pc := PoolCheck{
			Unbalanced: len(pool) < minSize || len(pool) > maxSize,
			SameClub:   map[engine.EntrantID][]engine.EntrantID{},
		}
		for _, id := range pool {
			e, ok := s.Entrant(id)
			if !ok {
				continue
			}
			if e.Seed > 0 {
				pc.Seeds++
			}
			if !e.Complete() {
				check.Absent = append(check.Absent, id)
			}
			for _, other := range pool {
				o, ok := s.Entrant(other)
				if other == id || !ok {
					continue
				}
				if shareClub(e, o) {
					pc.SameClub[id] = append(pc.SameClub[id], other)
				}
			}
		}
		check.Pools[i] = pc
	}
	for _, e := range s.Present(c) {
		if d.poolOf(e.ID) < 0 {
			check.Unplaced = append(check.Unplaced, e.ID)
		}
	}
	return check
}

func shareClub(a, b Entrant) bool {
	for _, c := range a.Clubs() {
		if slices.Contains(b.Clubs(), c) {
			return true
		}
	}
	return false
}
