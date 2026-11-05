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
// given order (their results compared across pools), so the best of each rank
// get the slots facing the weakest opponents. Entrants of one rank share the
// slots of their seed numbers and, by order of priority:
//
//  1. the two best entrants of a pool (its winner and runner-up) sit in
//     opposite halves: they can only meet in the final;
//  2. pool mates never meet in the first round;
//  3. the better entrant takes the better slot.
//
// When a rank cannot keep rules 1 and 2 among its own slots (a single best
// fourth facing the top seed from its pool), an entrant swaps with the
// closest seed of another rank, the weaker seeds moving first.
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

// seeding places the entrants of a bracket, best first.
type seeding struct {
	entrants []BracketEntrant
	size     int
	order    []int // seed number of each slot
	slotOf   []int // slot of each entrant, -1 until placed
	at       []int // entrant in each slot, -1 if none
	pair     []int // the other one of the two best of its pool, -1 if none
}

func newSeeding(sorted []BracketEntrant) *seeding {
	size := NextPowerOfTwo(len(sorted))
	b := &seeding{entrants: sorted, size: size, order: StandardOrder(size)}
	b.slotOf = slices.Repeat([]int{-1}, len(sorted))
	b.at = slices.Repeat([]int{-1}, size)
	b.pair = slices.Repeat([]int{-1}, len(sorted))
	best := map[int][]int{} // pool → its two best entrants
	for i, e := range sorted {
		if len(best[e.Pool]) < 2 {
			best[e.Pool] = append(best[e.Pool], i)
		}
	}
	for _, two := range best {
		if len(two) == 2 {
			b.pair[two[0]], b.pair[two[1]] = two[1], two[0]
		}
	}
	return b
}

// cost counts the rules entrant i breaks in a slot, against the entrants
// placed: a pool's two best not in opposite halves, pool mates in round 1.
func (b *seeding) cost(i, slot int) (halves, firstRound int) {
	if j := b.pair[i]; j >= 0 && b.slotOf[j] >= 0 && b.size > 2 && MeetRound(slot, b.slotOf[j]) < MeetRound(0, b.size-1) {
		halves++
	}
	if slot^1 >= b.size {
		return halves, firstRound
	}
	if j := b.at[slot^1]; j >= 0 && j != i && b.entrants[j].Pool == b.entrants[i].Pool {
		firstRound++
	}
	return halves, firstRound
}

func (b *seeding) place(i, slot int) {
	b.slotOf[i], b.at[slot] = slot, i
}

func (b *seeding) unplace(i int) {
	b.at[b.slotOf[i]], b.slotOf[i] = -1, -1
}

// placeTier gives the entrants start..end-1 (one rank) the slots of their
// seed numbers, breaking as few rules as possible; among the best ways, the
// better entrant takes the better slot.
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
	best, bestCost := []int(nil), [2]int{1 << 30, 1 << 30}
	cur := make([]int, 0, len(free))
	budget := 200000 // large ranks: keep the best way found
	var dfs func(i int, c [2]int)
	dfs = func(i int, c [2]int) {
		budget--
		if !less(c, bestCost) || budget < 0 {
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
			h, f := b.cost(i, slot)
			used[k] = true
			b.place(i, slot)
			cur = append(cur, slot)
			dfs(i+1, [2]int{c[0] + h, c[1] + f})
			cur = cur[:len(cur)-1]
			b.unplace(i)
			used[k] = false
		}
	}
	dfs(start, [2]int{})
	for k, slot := range best {
		b.place(start+k, slot)
	}
}

// total counts the rules broken by the whole placement.
func (b *seeding) total() [2]int {
	var t [2]int
	for i, slot := range b.slotOf {
		h, f := b.cost(i, slot)
		t[0] += h
		t[1] += f
	}
	return t
}

// repair swaps entrants of different ranks while it breaks fewer rules:
// the swap between the closest seeds, the weaker ones first.
func (b *seeding) repair() {
	for range len(b.entrants) {
		now := b.total()
		if now == [2]int{} {
			return
		}
		bx, by, bestCost, bestDist, bestMin := -1, -1, now, 0, 0
		for x := range b.entrants {
			for y := x + 1; y < len(b.entrants); y++ {
				if b.entrants[x].Rank == b.entrants[y].Rank {
					continue
				}
				b.swap(x, y)
				c := b.total()
				b.swap(x, y)
				sx, sy := b.order[b.slotOf[x]], b.order[b.slotOf[y]]
				dist, low := max(sx-sy, sy-sx), min(sx, sy)
				if less(c, bestCost) || (c == bestCost && bx >= 0 && (dist < bestDist || dist == bestDist && low > bestMin)) {
					bx, by, bestCost, bestDist, bestMin = x, y, c, dist, low
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

// less compares rule counts: opposite halves first, then round 1.
func less(a, c [2]int) bool {
	if a[0] != c[0] {
		return a[0] < c[0]
	}
	return a[1] < c[1]
}
