package engine

import "slices"

// EntrantID identifies an entrant (a player in singles, a pair in doubles).
type EntrantID string

// Tiebreak is a pool ranking criterion. Criteria always use differences,
// never ratios, so walkovers stay neutral.
type Tiebreak string

const (
	TiebreakWins            Tiebreak = "wins"
	TiebreakHeadToHead      Tiebreak = "headToHead"
	TiebreakSetDifference   Tiebreak = "setDifference"
	TiebreakPointDifference Tiebreak = "pointDifference"
	TiebreakDraw            Tiebreak = "draw"
)

// DefaultTiebreaks is the default criteria order of a pool stage.
var DefaultTiebreaks = []Tiebreak{
	TiebreakWins,
	TiebreakHeadToHead,
	TiebreakSetDifference,
	TiebreakPointDifference,
	TiebreakDraw,
}

// PoolMatch is a match between two entrants of a pool. Score is nil while the
// match has not been played.
type PoolMatch struct {
	A     EntrantID
	B     EntrantID
	Score *Score
}

// Pool is the input of a pool ranking.
type Pool struct {
	// Entrants in draw order. Unresolved ties keep this order.
	Entrants []EntrantID
	Matches  []PoolMatch
	// Withdrawn entrants: all their matches are walkovers, played or not.
	Withdrawn map[EntrantID]bool
	// Tiebreaks in the order they apply.
	Tiebreaks []Tiebreak
	// DrawLots are the lots drawn to break remaining ties, recorded as an
	// event so replay stays deterministic. Lower lots rank first.
	DrawLots map[EntrantID]int
}

// Standing is one row of a pool ranking.
type Standing struct {
	Entrant EntrantID
	// Rank is 1-based. Tied entrants share the same rank.
	Rank       int
	Played     int // decided matches, walkovers included
	Wins       int
	Losses     int
	Walkovers  int // matches decided by a walkover, won or lost
	SetsWon    int
	SetsLost   int
	PointsWon  int
	PointsLost int
	Withdrawn  bool
	// DrawNeeded is set when the pool is complete and this entrant is still
	// tied after every criterion but the draw: lots must be drawn.
	DrawNeeded bool
}

// SetDifference returns sets won minus sets lost.
func (s Standing) SetDifference() int { return s.SetsWon - s.SetsLost }

// PointDifference returns points won minus points lost.
func (s Standing) PointDifference() int { return s.PointsWon - s.PointsLost }

// PoolStandings is the ranking of a pool.
type PoolStandings struct {
	Rows []Standing
	// Complete is set when every match of the pool is decided.
	Complete bool
}

// RankPool computes the standings of a pool.
//
// A withdrawn entrant loses all its matches by walkover and ranks last. Those
// matches count as wins for the opponents but are excluded from sets and
// points. Among non-withdrawn entrants, the tiebreaks apply in order. When a
// criterion splits a tied group, each subgroup is ranked again from the first
// criterion, so head-to-head (which only applies between exactly two entrants)
// can decide a pair left over from a larger tie.
func RankPool(p Pool) PoolStandings {
	rows := make(map[EntrantID]*Standing, len(p.Entrants))
	for _, id := range p.Entrants {
		rows[id] = &Standing{Entrant: id, Withdrawn: p.Withdrawn[id]}
	}

	complete := true
	headToHead := map[[2]EntrantID]EntrantID{}
	for _, m := range p.Matches {
		a, b := rows[m.A], rows[m.B]
		if a == nil || b == nil {
			continue
		}
		switch {
		case a.Withdrawn || b.Withdrawn:
			recordWalkover(a, b)
		case m.Score != nil:
			winner := recordScore(a, b, *m.Score)
			if winner != "" {
				headToHead[pairKey(m.A, m.B)] = winner
			}
		default:
			complete = false
		}
	}

	r := ranker{tiebreaks: p.Tiebreaks, lots: p.DrawLots, headToHead: headToHead, complete: complete}

	var active, withdrawn []*Standing
	for _, id := range p.Entrants {
		if rows[id].Withdrawn {
			withdrawn = append(withdrawn, rows[id])
		} else {
			active = append(active, rows[id])
		}
	}

	var groups [][]*Standing
	if len(active) > 0 {
		groups = r.order(active)
	}
	for _, w := range withdrawn {
		groups = append(groups, []*Standing{w})
	}

	out := PoolStandings{Complete: complete, Rows: make([]Standing, 0, len(p.Entrants))}
	position := 1
	for _, g := range groups {
		for _, row := range g {
			row.Rank = position
			out.Rows = append(out.Rows, *row)
		}
		position += len(g)
	}
	return out
}

func recordWalkover(a, b *Standing) {
	a.Played++
	b.Played++
	a.Walkovers++
	b.Walkovers++
	switch {
	case a.Withdrawn && b.Withdrawn:
		// Nobody wins.
		a.Losses++
		b.Losses++
	case a.Withdrawn:
		b.Wins++
		a.Losses++
	default:
		a.Wins++
		b.Losses++
	}
}

func recordScore(a, b *Standing, s Score) EntrantID {
	setsA, setsB := s.SetsWon()
	pointsA, pointsB := s.Points()
	a.Played++
	b.Played++
	a.SetsWon += setsA
	a.SetsLost += setsB
	b.SetsWon += setsB
	b.SetsLost += setsA
	a.PointsWon += pointsA
	a.PointsLost += pointsB
	b.PointsWon += pointsB
	b.PointsLost += pointsA
	switch s.Winner() {
	case SideA:
		a.Wins++
		b.Losses++
		return a.Entrant
	case SideB:
		b.Wins++
		a.Losses++
		return b.Entrant
	}
	return ""
}

func pairKey(a, b EntrantID) [2]EntrantID {
	if a > b {
		a, b = b, a
	}
	return [2]EntrantID{a, b}
}

type ranker struct {
	tiebreaks  []Tiebreak
	lots       map[EntrantID]int
	headToHead map[[2]EntrantID]EntrantID
	complete   bool
}

// order splits a group of entrants into ordered groups of tied entrants.
func (r ranker) order(group []*Standing) [][]*Standing {
	if len(group) == 1 {
		return [][]*Standing{group}
	}
	for _, tb := range r.tiebreaks {
		parts := r.split(group, tb)
		if len(parts) > 1 {
			var out [][]*Standing
			for _, part := range parts {
				out = append(out, r.order(part)...)
			}
			return out
		}
	}
	// Still tied after every criterion.
	if r.complete && slices.Contains(r.tiebreaks, TiebreakDraw) {
		for _, row := range group {
			row.DrawNeeded = true
		}
	}
	return [][]*Standing{group}
}

// split partitions a group by one criterion, best first. It returns the group
// unchanged when the criterion does not apply or does not separate anyone.
func (r ranker) split(group []*Standing, tb Tiebreak) [][]*Standing {
	switch tb {
	case TiebreakWins:
		return splitBy(group, func(s *Standing) int { return s.Wins })
	case TiebreakSetDifference:
		return splitBy(group, func(s *Standing) int { return s.SetDifference() })
	case TiebreakPointDifference:
		return splitBy(group, func(s *Standing) int { return s.PointDifference() })
	case TiebreakHeadToHead:
		if len(group) != 2 {
			return [][]*Standing{group}
		}
		winner, ok := r.headToHead[pairKey(group[0].Entrant, group[1].Entrant)]
		if !ok {
			return [][]*Standing{group}
		}
		if winner == group[0].Entrant {
			return [][]*Standing{{group[0]}, {group[1]}}
		}
		return [][]*Standing{{group[1]}, {group[0]}}
	case TiebreakDraw:
		for _, s := range group {
			if _, ok := r.lots[s.Entrant]; !ok {
				return [][]*Standing{group}
			}
		}
		// Lower lots first: negate so splitBy's descending order applies.
		return splitBy(group, func(s *Standing) int { return -r.lots[s.Entrant] })
	}
	return [][]*Standing{group}
}

// splitBy groups entrants by key, highest key first, keeping input order
// within a group.
func splitBy(group []*Standing, key func(*Standing) int) [][]*Standing {
	sorted := slices.Clone(group)
	slices.SortStableFunc(sorted, func(a, b *Standing) int { return key(b) - key(a) })
	var out [][]*Standing
	for i, s := range sorted {
		if i == 0 || key(s) != key(sorted[i-1]) {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], s)
	}
	return out
}
