package tournament

import (
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

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
