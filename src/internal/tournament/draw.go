package tournament

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeEntrantSeeded events.Type = "EntrantSeeded"
	TypeEntryDrawn    events.Type = "EntryDrawn"
	TypeEntrantMoved  events.Type = "EntrantMoved"
	TypeDrawConfirmed events.Type = "DrawConfirmed"
	TypeDrawReopened  events.Type = "DrawReopened"
)

// MaxSeed is the highest seed number.
const MaxSeed = 32

// Draw is the entry stage of a category once drawn: its pools, or a single
// group of bracket slots. A draft until confirmed.
type Draw struct {
	Stage     string
	Kind      StageKind
	Pools     [][]engine.EntrantID
	Confirmed bool
	// ConfirmedAt is when the draw was last confirmed.
	ConfirmedAt time.Time
}

// EntrantSeeded is the payload of TypeEntrantSeeded. Seed 0 clears the seed.
// A seed number is unique within a category: giving it to an entrant takes
// it from the one that had it.
type EntrantSeeded struct {
	ID   engine.EntrantID `json:"id"`
	Seed int              `json:"seed"`
}

// EntryDrawn is the payload of TypeEntryDrawn: the entry stage as drawn,
// pools or a single group of bracket slots, so replay does not depend on
// randomness.
type EntryDrawn struct {
	Category engine.Category      `json:"category"`
	Groups   [][]engine.EntrantID `json:"groups"`
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
	e := s.entry(c)
	if e == nil || !e.Filled() {
		return Draw{}, false
	}
	return Draw{Stage: e.ID, Kind: e.Kind, Pools: cloneGroups(e.Groups), Confirmed: e.Confirmed, ConfirmedAt: e.ConfirmedAt}, true
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

// DrawEntry draws the entry stage of a category among present entrants:
// pools, or the slots of a bracket. rng is seeded by the caller; the event
// stores the result.
func (s State) DrawEntry(c engine.Category, rng *rand.Rand) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	e := s.entry(c)
	if e.Confirmed {
		return Change{}, engine.NewError(engine.CodeDrawConfirmed)
	}
	present := s.Present(c)
	in := make([]engine.DrawEntrant, len(present))
	for i, p := range present {
		in[i] = engine.DrawEntrant{ID: p.ID, Seed: p.Seed, Clubs: p.Clubs()}
	}
	if e.Kind == StageBracket {
		if len(present) < 2 {
			return Change{}, engine.NewError(engine.CodeNotEnoughEntrants)
		}
		return Change{TypeEntryDrawn, EntryDrawn{Category: c, Groups: [][]engine.EntrantID{engine.DrawBracket(in, rng)}}}, nil
	}
	sizes := e.PoolSizesFor(len(present))
	if sizes == nil {
		// Too few entrants for the pools, or no split with the sizes allowed.
		if len(e.PoolSizes) > 0 && len(present) >= 2 {
			return Change{}, engine.NewError(engine.CodeNoPoolSplit)
		}
		return Change{}, engine.NewError(engine.CodeNotEnoughEntrants)
	}
	return Change{TypeEntryDrawn, EntryDrawn{Category: c, Groups: engine.DrawPoolsSized(in, sizes, rng)}}, nil
}

// MoveEntrant moves an entrant to another pool of the draft draw. An entrant
// checked in after the draw can be placed this way too.
func (s State) MoveEntrant(id engine.EntrantID, pool int) (Change, error) {
	e, ok := s.Entrant(id)
	if !ok {
		return Change{}, engine.NewError(engine.CodeEntrantNotFound)
	}
	d, ok := s.Draw(e.Category)
	switch {
	case !ok:
		return Change{}, engine.NewError(engine.CodeNoDraw)
	case d.Kind != StagePools:
		return Change{}, engine.NewError(engine.CodeInvalidPool)
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

// ConfirmDraw locks the draw of a category: the entry stage starts.
func (s State) ConfirmDraw(c engine.Category) (Change, error) {
	d, ok := s.Draw(c)
	if !ok {
		return Change{}, engine.NewError(engine.CodeNoDraw)
	}
	if d.Confirmed {
		return Change{}, nil
	}
	return Change{TypeDrawConfirmed, DrawConfirmed{Category: c}}, nil
}

// ReopenDraw unlocks a confirmed draw.
func (s State) ReopenDraw(c engine.Category) (Change, error) {
	d, ok := s.Draw(c)
	if !ok {
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

func (d Draw) poolOf(id engine.EntrantID) int {
	for i, pool := range d.Pools {
		if slices.Contains(pool, id) {
			return i
		}
	}
	return -1
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

func (s *State) applyEntryDrawn(p EntryDrawn) {
	if e := s.entry(p.Category); e != nil {
		e.Groups, e.Confirmed = cloneGroups(p.Groups), false
	}
}

func (s *State) applyEntrantMoved(p EntrantMoved) {
	e := s.entry(p.Category)
	if e == nil || p.Pool < 0 || p.Pool >= len(e.Groups) {
		return
	}
	for i := range e.Groups {
		e.Groups[i] = slices.DeleteFunc(e.Groups[i], func(x engine.EntrantID) bool { return x == p.Entrant })
	}
	e.Groups[p.Pool] = append(e.Groups[p.Pool], p.Entrant)
}

// setConfirmed starts or stops the entry stage: its matches follow.
func (s *State) setConfirmed(c engine.Category, confirmed bool, at time.Time) {
	e := s.entry(c)
	if e == nil || !e.Filled() {
		return
	}
	e.Confirmed, e.ConfirmedAt = confirmed, at
	if confirmed {
		s.generateStageMatches(c, e)
	} else {
		s.dropStageMatches(c, e.ID)
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
	d, ok := s.Draw(c)
	if !ok {
		return DrawCheck{}
	}
	if d.Kind == StageBracket {
		return s.checkBracketDraw(c, d)
	}
	placed := 0
	for _, pool := range d.Pools {
		placed += len(pool)
	}
	// Pools are unbalanced when their size is not what the format plans.
	planned := map[int]bool{}
	if e := s.entry(c); len(e.PoolSizes) > 0 {
		for _, size := range e.PoolSizes {
			planned[size] = true
		}
	} else {
		for _, size := range engine.PoolSizes(placed, len(d.Pools)) {
			planned[size] = true
		}
	}

	check := DrawCheck{Pools: make([]PoolCheck, len(d.Pools))}
	for i, pool := range d.Pools {
		pc := PoolCheck{
			Unbalanced: !planned[len(pool)],
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

// checkBracketDraw reviews a bracket entry stage: club mates meeting in the
// first round, absent and unplaced entrants. The single "pool" is the bracket.
func (s State) checkBracketDraw(c engine.Category, d Draw) DrawCheck {
	slots := d.Pools[0]
	pc := PoolCheck{SameClub: map[engine.EntrantID][]engine.EntrantID{}}
	for i, id := range slots {
		e, ok := s.Entrant(id)
		if id == "" || !ok {
			continue
		}
		if o, ok := s.Entrant(slots[i^1]); ok && slots[i^1] != "" && shareClub(e, o) {
			pc.SameClub[id] = []engine.EntrantID{o.ID}
		}
	}
	check := DrawCheck{Pools: []PoolCheck{pc}}
	for _, id := range slots {
		if e, ok := s.Entrant(id); ok && id != "" && !e.Complete() {
			check.Absent = append(check.Absent, id)
		}
	}
	for _, e := range s.Present(c) {
		if !slices.Contains(slots, e.ID) {
			check.Unplaced = append(check.Unplaced, e.ID)
		}
	}
	return check
}
