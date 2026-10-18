package engine

import "slices"

// CrossPool is how entrants of the same rank in different pools compare,
// for instance to pick the best thirds.
type CrossPool string

const (
	// CrossPoolRaw compares the results as they are: in larger pools,
	// entrants played more matches.
	CrossPoolRaw CrossPool = "raw"
	// CrossPoolSameMatches leaves out, in larger pools, the matches against
	// the entrants ranked below the size of the smallest pool, so everyone
	// is compared on the same number of matches.
	CrossPoolSameMatches CrossPool = "sameMatches"
)

// Valid reports whether the rule is known.
func (c CrossPool) Valid() bool { return c == CrossPoolRaw || c == CrossPoolSameMatches }

// RankedPool is a pool with its standings, for comparing across pools.
type RankedPool struct {
	Pool      Pool
	Standings PoolStandings
}

// AcrossKey is how an entrant compares with entrants of other pools: more
// wins, then a better set difference, then a better point difference.
type AcrossKey struct {
	Wins, SetDifference, PointDifference int
}

// Better reports whether k ranks before o.
func (k AcrossKey) Better(o AcrossKey) bool {
	if k.Wins != o.Wins {
		return k.Wins > o.Wins
	}
	if k.SetDifference != o.SetDifference {
		return k.SetDifference > o.SetDifference
	}
	return k.PointDifference > o.PointDifference
}

// AcrossKeys returns the comparison key of every entrant of the pools. With
// CrossPoolSameMatches, matches against entrants ranked below the smallest
// pool's size are left out of the larger pools. Withdrawn entrants get the
// lowest key.
func AcrossKeys(pools []RankedPool, rule CrossPool) map[EntrantID]AcrossKey {
	keep := 0
	for i, p := range pools {
		if i == 0 || len(p.Pool.Entrants) < keep {
			keep = len(p.Pool.Entrants)
		}
	}
	out := map[EntrantID]AcrossKey{}
	for _, p := range pools {
		rank := map[EntrantID]int{}
		for i, row := range p.Standings.Rows {
			rank[row.Entrant] = i + 1
		}
		counted := func(id EntrantID) bool {
			return rule != CrossPoolSameMatches || rank[id] <= keep
		}
		rows := map[EntrantID]*Standing{}
		for _, id := range p.Pool.Entrants {
			rows[id] = &Standing{Entrant: id, Withdrawn: p.Pool.Withdrawn[id]}
		}
		for _, m := range p.Pool.Matches {
			a, b := rows[m.A], rows[m.B]
			if a == nil || b == nil || !counted(m.A) || !counted(m.B) {
				continue
			}
			switch {
			case a.Withdrawn || b.Withdrawn:
				recordWalkover(a, b)
			case m.Score != nil:
				recordScore(a, b, *m.Score)
			}
		}
		for id, row := range rows {
			k := AcrossKey{Wins: row.Wins, SetDifference: row.SetDifference(), PointDifference: row.PointDifference()}
			if row.Withdrawn {
				k = AcrossKey{Wins: -1 << 30}
			}
			out[id] = k
		}
	}
	return out
}

// SortAcross orders entrants of the same rank from different pools, best
// first; equal keys keep their order (pool order).
func SortAcross(ids []EntrantID, keys map[EntrantID]AcrossKey) []EntrantID {
	out := slices.Clone(ids)
	slices.SortStableFunc(out, func(a, b EntrantID) int {
		switch {
		case keys[a].Better(keys[b]):
			return -1
		case keys[b].Better(keys[a]):
			return 1
		}
		return 0
	})
	return out
}
