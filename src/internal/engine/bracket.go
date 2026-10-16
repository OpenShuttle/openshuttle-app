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
// Entrants are seeded by rank in their pool, then by pool (A1, B1, C1, A2…).
// Entrants of equal rank share a tier of slots; within a tier, each takes the
// slot where it would meet entrants from its own pool as late as possible, so
// pool mates never meet in the first round when it can be avoided, and the
// winner and runner-up of a pool sit in opposite halves.
func SeedBracket(entrants []BracketEntrant) []EntrantID {
	sorted := slices.Clone(entrants)
	slices.SortStableFunc(sorted, func(a, b BracketEntrant) int {
		if a.Rank != b.Rank {
			return a.Rank - b.Rank
		}
		return a.Pool - b.Pool
	})
	size := NextPowerOfTwo(len(sorted))
	order := StandardOrder(size)
	slotOfSeed := make([]int, size+1)
	for slot, seed := range order {
		slotOfSeed[seed] = slot
	}

	slots := make([]EntrantID, size)
	placedPool := make([]int, size) // pool of the entrant in each slot, -1 if none
	for i := range placedPool {
		placedPool[i] = -1
	}
	for start := 0; start < len(sorted); {
		end := start
		for end < len(sorted) && sorted[end].Rank == sorted[start].Rank {
			end++
		}
		tier := sorted[start:end]
		free := make([]int, 0, len(tier))
		for seed := start + 1; seed <= end; seed++ {
			free = append(free, slotOfSeed[seed])
		}
		for _, e := range tier {
			best, bestScore := -1, -1
			for i, slot := range free {
				score := MeetRound(0, size-1) + 1 // nobody from this pool yet
				for other, pool := range placedPool {
					if pool == e.Pool {
						score = min(score, MeetRound(slot, other))
					}
				}
				if score > bestScore {
					best, bestScore = i, score
				}
			}
			slot := free[best]
			free = slices.Delete(free, best, best+1)
			slots[slot] = e.ID
			placedPool[slot] = e.Pool
		}
		start = end
	}
	return slots
}
