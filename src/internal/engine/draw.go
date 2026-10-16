package engine

import (
	"math/rand/v2"
	"slices"
)

// PoolSizes splits n entrants into pools as evenly as possible, larger pools
// first: 14 entrants in 4 pools gives 4, 4, 3, 3.
func PoolSizes(n, pools int) []int {
	if pools < 1 || n < 0 {
		return nil
	}
	sizes := make([]int, pools)
	for i := range sizes {
		sizes[i] = n / pools
		if i < n%pools {
			sizes[i]++
		}
	}
	return sizes
}

// RoundRobinMatches is the number of matches of a pool of size n.
func RoundRobinMatches(n int) int {
	return n * (n - 1) / 2
}

// DrawEntrant is an entrant as the draw sees it.
type DrawEntrant struct {
	ID EntrantID
	// Seed is 1 for the top seed, 0 when not seeded.
	Seed  int
	Clubs []string
}

// DrawPools places entrants into pools:
//
//   - pool sizes follow PoolSizes;
//   - seeds go first, in seed order, in a snake (A, B, C, C, B, A…), so the
//     strongest are spread out;
//   - the others come in a random order; each goes to the pool where it
//     meets the fewest players of its own clubs, then the emptiest pool.
//
// rng makes the draw reproducible: the caller seeds it, and records the
// result rather than the seed. The engine never reads a clock or global
// randomness.
func DrawPools(entrants []DrawEntrant, pools int, rng *rand.Rand) [][]EntrantID {
	sizes := PoolSizes(len(entrants), pools)
	out := make([][]EntrantID, pools)
	clubs := make([]map[string]int, pools)
	for i := range clubs {
		clubs[i] = map[string]int{}
	}
	place := func(e DrawEntrant, pool int) {
		out[pool] = append(out[pool], e.ID)
		for _, c := range e.Clubs {
			clubs[pool][c]++
		}
	}

	var seeded, others []DrawEntrant
	for _, e := range entrants {
		if e.Seed > 0 {
			seeded = append(seeded, e)
		} else {
			others = append(others, e)
		}
	}
	slices.SortStableFunc(seeded, func(a, b DrawEntrant) int { return a.Seed - b.Seed })

	snake := snakeOrder(pools)
	next := 0
	for _, e := range seeded {
		// Skip full pools: more seeds than pools wrap around the snake.
		for tries := 0; tries < len(snake) && len(out[snake[next%len(snake)]]) >= sizes[snake[next%len(snake)]]; tries++ {
			next++
		}
		pool := snake[next%len(snake)]
		if len(out[pool]) >= sizes[pool] {
			others = append(others, e) // no room left for seeds: draw it like the others
			continue
		}
		place(e, pool)
		next++
	}

	rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	for _, e := range others {
		best, bestConflicts, bestRoom := -1, 0, 0
		for p := range pools {
			room := sizes[p] - len(out[p])
			if room <= 0 {
				continue
			}
			conflicts := 0
			for _, c := range e.Clubs {
				conflicts += clubs[p][c]
			}
			if best < 0 || conflicts < bestConflicts || (conflicts == bestConflicts && room > bestRoom) {
				best, bestConflicts, bestRoom = p, conflicts, room
			}
		}
		place(e, best)
	}
	return out
}

// snakeOrder returns pool indexes 0..n-1 then n-1..0.
func snakeOrder(n int) []int {
	order := make([]int, 0, 2*n)
	for i := range n {
		order = append(order, i)
	}
	for i := n - 1; i >= 0; i-- {
		order = append(order, i)
	}
	return order
}

// PoolName is the letter of a pool: 0 is A, 25 is Z, 26 is AA.
func PoolName(i int) string {
	name := ""
	for i++; i > 0; i = (i - 1) / 26 {
		name = string(rune('A'+(i-1)%26)) + name
	}
	return name
}
