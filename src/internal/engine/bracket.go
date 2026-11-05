package engine

import (
	"math/bits"
	"slices"
)

// NextPowerOfTwo returns the smallest power of two >= n (1 for n <= 1).
func NextPowerOfTwo(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

// BracketMatches counts the matches a bracket of n entrants plays: n-1 for
// a plain knockout. With full placement every place is played: the losers of
// a round go on in their own sub-bracket. Byes go to the top seeds, so the
// first round has n - size/2 matches.
func BracketMatches(n int, fullPlacement bool) int {
	if n < 2 {
		return 0
	}
	if !fullPlacement {
		return n - 1
	}
	half := NextPowerOfTwo(n) / 2
	first := n - half
	return first + BracketMatches(half, true) + BracketMatches(first, true)
}

// BracketRounds is the number of rounds of a bracket of n entrants.
func BracketRounds(n int) int {
	return bits.Len(uint(NextPowerOfTwo(n))) - 1
}

// RoundRobinRounds is the number of rounds of a pool of n entrants: n-1,
// or n when one entrant sits out each round.
func RoundRobinRounds(n int) int {
	if n < 2 {
		return 0
	}
	if n%2 == 1 {
		return n
	}
	return n - 1
}

// StandardOrder returns the seed numbers (1-based) of a bracket of the given
// size, slot by slot: 1, 8, 4, 5, 2, 7, 3, 6 for 8. Seed 1 and 2 can only
// meet in the final, and every first-round match pairs a strong seed with a
// weak one.
func StandardOrder(size int) []int {
	order := []int{1}
	for n := 1; n < size; n *= 2 {
		next := make([]int, 0, 2*len(order))
		for _, s := range order {
			next = append(next, s, 2*n+1-s)
		}
		order = next
	}
	return order
}

// MeetRound is the round where the slots p and q of a bracket would meet:
// 1 for the first round, log2(size) for the final.
func MeetRound(p, q int) int {
	return bits.Len(uint(p ^ q))
}

// BracketEntrant is an entrant routed from the pools to a bracket.
type BracketEntrant struct {
	ID   EntrantID
	Rank int // position in its pool, 1 is first
	Pool int // 0 is A
}

// SeedBracket places entrants into the first-round slots of a bracket whose
// size is the next power of two. Empty slots are byes, given to the best
// seeds.
//
// Entrants come best first: by rank in their pool, and within a rank in the
// given order (their results compared across pools). Entrants of one rank
// share the slots of their seed numbers and, by order of priority:
//
//  1. the two best entrants of a pool (its winner and runner-up) sit in
//     opposite halves: they can only meet in the final;
//  2. pool mates never meet in the first round;
//  3. pool mates meet as late as possible;
//  4. the better entrant takes the better slot (the bye, the weakest
//     opponent).
//
// When a rank cannot keep rules 1 and 2 on its own slots (a single best
// fourth facing the top seed from its pool), an entrant swaps with a seed of
// another rank: the swap meeting pool mates the latest, then the closest
// seeds, the weaker ones first.
func SeedBracket(entrants []BracketEntrant) []EntrantID {
	sorted := slices.Clone(entrants)
	slices.SortStableFunc(sorted, func(a, b BracketEntrant) int { return a.Rank - b.Rank })
	b := newSeeding(sorted)
	for start := 0; start < len(sorted); {
		end := start
		for end < len(sorted) && sorted[end].Rank == sorted[start].Rank {
			end++
		}
		b.placeTier(start, end)
		start = end
	}
	b.repair()
	slots := make([]EntrantID, b.size)
	for i, slot := range b.slotOf {
		slots[slot] = sorted[i].ID
	}
	return slots
}

// seedCost counts what a placement breaks, compared in order: a pool's two
// best not in opposite halves, pool mates in round 1, then how early pool
// mates meet (each pair 2^(rounds before the final) - 1, so one early
// meeting weighs more than several late ones).
type seedCost [3]int

func (c seedCost) add(o seedCost) seedCost {
	return seedCost{c[0] + o[0], c[1] + o[1], c[2] + o[2]}
}

func (c seedCost) less(o seedCost) bool {
	for k := range c {
		if c[k] != o[k] {
			return c[k] < o[k]
		}
	}
	return false
}

// broken reports whether rule 1 or 2 is broken.
func (c seedCost) broken() bool { return c[0] > 0 || c[1] > 0 }

// seeding places the entrants of a bracket, best first.
type seeding struct {
	entrants []BracketEntrant
	size     int
	final    int   // round of the final
	order    []int // seed number of each slot
	slotOf   []int // slot of each entrant, -1 until placed
	at       []int // entrant in each slot, -1 if none
	pair     []int // the other one of the two best of its pool, -1 if none
	mates    [][]int
}

func newSeeding(sorted []BracketEntrant) *seeding {
	size := NextPowerOfTwo(len(sorted))
	b := &seeding{entrants: sorted, size: size, final: MeetRound(0, size-1), order: StandardOrder(size)}
	b.slotOf = slices.Repeat([]int{-1}, len(sorted))
	b.at = slices.Repeat([]int{-1}, size)
	b.pair = slices.Repeat([]int{-1}, len(sorted))
	b.mates = make([][]int, len(sorted))
	byPool := map[int][]int{}
	for i, e := range sorted {
		for _, j := range byPool[e.Pool] {
			b.mates[i] = append(b.mates[i], j)
			b.mates[j] = append(b.mates[j], i)
		}
		byPool[e.Pool] = append(byPool[e.Pool], i)
	}
	for _, all := range byPool {
		if len(all) >= 2 {
			b.pair[all[0]], b.pair[all[1]] = all[1], all[0]
		}
	}
	return b
}

// cost is what entrant i in a slot breaks against its placed pool mates.
func (b *seeding) cost(i, slot int) seedCost {
	var c seedCost
	for _, j := range b.mates[i] {
		other := b.slotOf[j]
		if other < 0 {
			continue
		}
		meet := MeetRound(slot, other)
		if j == b.pair[i] && meet < b.final {
			c[0]++
		}
		if meet == 1 {
			c[1]++
		}
		c[2] += 1<<(b.final-meet) - 1
	}
	return c
}

func (b *seeding) place(i, slot int) {
	b.slotOf[i], b.at[slot] = slot, i
}

func (b *seeding) unplace(i int) {
	b.at[b.slotOf[i]], b.slotOf[i] = -1, -1
}

// placeTier gives the entrants start..end-1 (one rank) the slots of their
// seed numbers at the lowest cost; among equal ways, the better entrant
// takes the better slot (the first way found).
func (b *seeding) placeTier(start, end int) {
	slotOfSeed := make([]int, b.size+1)
	for slot, seed := range b.order {
		slotOfSeed[seed] = slot
	}
	var free []int
	for seed := start + 1; seed <= end; seed++ {
		free = append(free, slotOfSeed[seed])
	}
	used := make([]bool, len(free))
	var best []int
	bestCost := seedCost{1 << 30}
	cur := make([]int, 0, len(free))
	budget := 200000 // large ranks: keep the best way found
	var dfs func(i int, c seedCost)
	dfs = func(i int, c seedCost) {
		budget--
		if !c.less(bestCost) || budget < 0 {
			return
		}
		if i == end {
			best, bestCost = slices.Clone(cur), c
			return
		}
		for k, slot := range free {
			if used[k] {
				continue
			}
			used[k] = true
			cost := b.cost(i, slot)
			b.place(i, slot)
			cur = append(cur, slot)
			dfs(i+1, c.add(cost))
			cur = cur[:len(cur)-1]
			b.unplace(i)
			used[k] = false
		}
	}
	dfs(start, seedCost{})
	for k, slot := range best {
		b.place(start+k, slot)
	}
}

// total is the cost of the whole placement (each pair counted twice).
func (b *seeding) total() seedCost {
	var t seedCost
	for i, slot := range b.slotOf {
		t = t.add(b.cost(i, slot))
	}
	return t
}

// repair swaps entrants of different ranks while rules 1 or 2 are broken
// and a swap breaks them less: the swap meeting pool mates the latest, then
// between the closest seeds, the weaker ones first.
func (b *seeding) repair() {
	for range len(b.entrants) {
		now := b.total()
		if !now.broken() {
			return
		}
		bx, by, bestCost, bestDist, bestLow := -1, -1, seedCost{}, 0, 0
		for x := range b.entrants {
			for y := x + 1; y < len(b.entrants); y++ {
				if b.entrants[x].Rank == b.entrants[y].Rank {
					continue
				}
				b.swap(x, y)
				c := b.total()
				b.swap(x, y)
				if !(seedCost{c[0], c[1]}).less(seedCost{now[0], now[1]}) {
					continue
				}
				sx, sy := b.order[b.slotOf[x]], b.order[b.slotOf[y]]
				dist, low := max(sx-sy, sy-sx), min(sx, sy)
				if bx < 0 || c.less(bestCost) || c == bestCost && (dist < bestDist || dist == bestDist && low > bestLow) {
					bx, by, bestCost, bestDist, bestLow = x, y, c, dist, low
				}
			}
		}
		if bx < 0 {
			return
		}
		b.swap(bx, by)
	}
}

func (b *seeding) swap(x, y int) {
	sx, sy := b.slotOf[x], b.slotOf[y]
	b.slotOf[x], b.slotOf[y] = sy, sx
	b.at[sx], b.at[sy] = y, x
}
