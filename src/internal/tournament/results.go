package tournament

import (
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// Ranked is a line of a category's full standings.
type Ranked struct {
	// Rank is the final position, 0 while not decided yet.
	Rank int
	// Tied is set when other entrants share this rank (entrants losing in
	// the same round of a bracket without full placement, or eliminated at
	// the same pool position).
	Tied    bool
	Entrant engine.EntrantID
	// Bracket is where the rank was earned, "" for entrants eliminated in
	// the pools.
	Bracket string
	// PoolPosition is the entrant's position in its pool.
	PoolPosition int
}

// FinalStandings ranks every drawn entrant of a category. Brackets come in
// the order of the pool positions they take (the bracket of the pool winners
// first), each bracket's places end to end; entrants eliminated in the pools
// follow, tied by pool position. complete is set when every rank is final.
func (s State) FinalStandings(c engine.Category) (rows []Ranked, complete bool) {
	d, ok := s.Draw(c)
	if !ok || !d.Confirmed {
		return nil, false
	}
	complete = true
	settled := 0
	for _, ref := range s.CompletePools() {
		if ref.Category == c {
			settled++
		}
	}
	if settled < len(d.Pools) {
		complete = false
	}

	// Pool position of every drawn entrant, from the current standings.
	position := map[engine.EntrantID]int{}
	for p := range d.Pools {
		for i, row := range s.PoolStandings(c, p).Rows {
			position[row.Entrant] = i + 1
		}
	}

	routing := s.routing[c]
	firstPosition := func(id string) int {
		return slices.Index(routing, id)
	}
	brackets := slices.DeleteFunc(s.Brackets(c), func(b Bracket) bool { return firstPosition(b.ID) < 0 })
	slices.SortStableFunc(brackets, func(a, b Bracket) int { return firstPosition(a.ID) - firstPosition(b.ID) })

	placed := map[engine.EntrantID]bool{}
	offset := 0
	for _, b := range brackets {
		var members []engine.EntrantID
		if b.Drawn() {
			for _, id := range b.Slots {
				if id != "" {
					members = append(members, id)
				}
			}
		} else {
			for _, e := range s.Routed(c, b.ID) {
				members = append(members, e.ID)
			}
		}
		places := map[engine.EntrantID]Place{}
		for _, p := range s.Places(c, b.ID) {
			places[p.Entrant] = p
		}
		for _, id := range members {
			placed[id] = true
			r := Ranked{Entrant: id, Bracket: b.ID, PoolPosition: position[id]}
			p, ok := places[id]
			if !ok {
				complete = false
				rows = append(rows, r)
				continue
			}
			// Byes leave holes in the bracket places: count entrants instead.
			better, same := 0, 0
			for _, q := range places {
				switch {
				case q.From < p.From:
					better++
				case q.From == p.From:
					same++
				}
			}
			r.Rank, r.Tied = offset+better+1, same > 1
			rows = append(rows, r)
		}
		offset += len(members)
	}

	// Entrants eliminated in the pools, grouped by pool position.
	var out []engine.EntrantID
	for p := range d.Pools {
		for _, id := range d.Pools[p] {
			if !placed[id] {
				out = append(out, id)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b engine.EntrantID) int { return position[a] - position[b] })
	for i := 0; i < len(out); {
		j := i
		for j < len(out) && position[out[j]] == position[out[i]] {
			j++
		}
		for _, id := range out[i:j] {
			rank := offset + 1
			if !complete && settled < len(d.Pools) {
				rank = 0
			}
			rows = append(rows, Ranked{Rank: rank, Tied: j-i > 1, Entrant: id, PoolPosition: position[id]})
		}
		offset += j - i
		i = j
	}

	slices.SortStableFunc(rows, func(a, b Ranked) int {
		switch {
		case a.Rank == b.Rank:
			return 0
		case a.Rank == 0:
			return 1
		case b.Rank == 0:
			return -1
		}
		return a.Rank - b.Rank
	})
	return rows, complete
}
