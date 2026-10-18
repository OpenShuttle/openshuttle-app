package engine

import (
	"math/rand/v2"
	"slices"
)

// FillPools places entrants coming from earlier stages into pools. Ranks are
// spread in a snake (the best of each rank in A, B, C, then C, B, A…), and
// each entrant goes, within its row of the snake, to the first pool with room
// where nobody comes from the same group (Pool) as it, when there is one.
// The result only depends on the input: no randomness.
func FillPools(entrants []BracketEntrant, pools int) [][]EntrantID {
	if pools < 1 {
		return nil
	}
	return FillPoolsSized(entrants, PoolSizes(len(entrants), pools))
}

// FillPoolsSized is FillPools with the size of each pool given, larger
// pools first.
func FillPoolsSized(entrants []BracketEntrant, sizes []int) [][]EntrantID {
	pools := len(sizes)
	if pools < 1 {
		return nil
	}
	sorted := slices.Clone(entrants)
	slices.SortStableFunc(sorted, func(a, b BracketEntrant) int {
		if a.Rank != b.Rank {
			return a.Rank - b.Rank
		}
		return a.Pool - b.Pool
	})
	out := make([][]EntrantID, pools)
	groups := make([]map[int]bool, pools)
	for i := range groups {
		groups[i] = map[int]bool{}
	}
	for row := 0; row*pools < len(sorted); row++ {
		order := make([]int, pools)
		for i := range order {
			order[i] = i
			if row%2 == 1 {
				order[i] = pools - 1 - i
			}
		}
		end := min(len(sorted), (row+1)*pools)
		for _, e := range sorted[row*pools : end] {
			pick := -1
			for _, p := range order {
				if len(out[p]) <= row && len(out[p]) < sizes[p] && !groups[p][e.Pool] {
					pick = p
					break
				}
			}
			if pick < 0 {
				for _, p := range order {
					if len(out[p]) <= row && len(out[p]) < sizes[p] {
						pick = p
						break
					}
				}
			}
			out[pick] = append(out[pick], e.ID)
			groups[pick][e.Pool] = true
		}
	}
	return out
}

// DrawBracket places entrants straight into the first-round slots of a
// bracket, for a format starting with a bracket. Seeds take the slots of
// their seed number (standard order), the others the next numbers at random;
// the missing numbers are byes, so they face the top seeds. Then unseeded
// entrants meeting a club mate in the first round are swapped with another
// unseeded entrant when that removes the conflict.
func DrawBracket(entrants []DrawEntrant, rng *rand.Rand) []EntrantID {
	var seeded, others []DrawEntrant
	for _, e := range entrants {
		if e.Seed > 0 {
			seeded = append(seeded, e)
		} else {
			others = append(others, e)
		}
	}
	slices.SortStableFunc(seeded, func(a, b DrawEntrant) int { return a.Seed - b.Seed })
	rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	ordered := append(seeded, others...)

	size := NextPowerOfTwo(len(ordered))
	slots := make([]EntrantID, size)
	byID := map[EntrantID]DrawEntrant{}
	for slot, seed := range StandardOrder(size) {
		if seed <= len(ordered) {
			slots[slot] = ordered[seed-1].ID
			byID[ordered[seed-1].ID] = ordered[seed-1]
		}
	}

	conflict := func(slot int) bool {
		a, b := slots[slot], slots[slot^1]
		return a != "" && b != "" && sharesClub(byID[a], byID[b])
	}
	movable := func(slot int) bool { return slots[slot] != "" && byID[slots[slot]].Seed == 0 }
	for i := range slots {
		if !conflict(i) || !movable(i) {
			continue
		}
		for j := range slots {
			if j/2 == i/2 || !movable(j) {
				continue
			}
			slots[i], slots[j] = slots[j], slots[i]
			if !conflict(i) && !conflict(j) {
				break
			}
			slots[i], slots[j] = slots[j], slots[i]
		}
	}
	return slots
}

func sharesClub(a, b DrawEntrant) bool {
	for _, c := range a.Clubs {
		if slices.Contains(b.Clubs, c) {
			return true
		}
	}
	return false
}
