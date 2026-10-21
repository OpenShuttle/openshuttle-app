// Package blocks orders the schedule's blocks: a block is one round of a
// pool, or one round of a part of a bracket. The tournament cuts its waiting
// matches into waves of blocks; this package merges the waves of every
// category into one order, applies the order the desk set, and keeps the
// rounds of a pool or a bracket in their order. Pure: no State, no clock.
package blocks

import (
	"fmt"
	"slices"
	"strings"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// Block is a round of a pool, or a round of a part of a bracket.
type Block struct {
	Key string
	// Group ranks the block within its wave by default: its pool
	// ("MD/S1/A"), or its bracket round. Older files ordered these.
	Group string
	// Series is the pool, or the bracket, whose rounds keep their order;
	// Round is the block's round in it.
	Series string
	Round  int
	// Place is, in a bracket, the first place its part plays for; in pools,
	// the pool.
	Place int
	// Matches are the block's waiting matches, none for a stand-in block.
	Matches []string
}

// PoolKey is the key of a round of a pool: "MD/S1/A/2" (round from 0).
func PoolKey(c engine.Category, stage string, pool, round int) string {
	return fmt.Sprintf("%s/%s/%s/%d", c, stage, engine.PoolName(pool), round+1)
}

// PoolGroup is the group of the rounds of a pool: "MD/S1/A".
func PoolGroup(c engine.Category, stage string, pool int) string {
	return fmt.Sprintf("%s/%s/%s", c, stage, engine.PoolName(pool))
}

// BracketKey is the key of a bracket round (level from 0): "MD/S2/R1" for
// the main draw, "MD/S2/R2/P5" for the round of places 5 and below starting
// at 5 (full placement).
func BracketKey(c engine.Category, stage string, level, placeFrom int) string {
	if placeFrom > 1 {
		return fmt.Sprintf("%s/%s/R%d/P%d", c, stage, level+1, placeFrom)
	}
	return fmt.Sprintf("%s/%s/R%d", c, stage, level+1)
}

// IsRound tells a round of a pool ("MD/S1/A/2") or of a part of a bracket
// ("MD/S2/R2/P5"), in an order set by the desk, from a pool of older files
// ("MD/S1/A"), whose order applies within each wave. Main bracket rounds
// ("MD/S2/R1") are in both.
func IsRound(key string) bool {
	return strings.Count(key, "/") == 3
}

// SortByPlace orders the blocks of each wave by place: pool order, or the
// main draw before the places below.
func SortByPlace(waves [][]Block) {
	for _, w := range waves {
		slices.SortStableFunc(w, func(a, b Block) int { return a.Place - b.Place })
	}
}

// Order merges the waves of every category into the order the blocks are
// played. Waves go side by side (wave 1 of each category, then wave 2…);
// within a wave the categories follow their opening order (the categories
// argument), then each category's groups as its waves bring them; groups an
// older file ordered (stored pools) come first. Once the desk set a block
// order (stored rounds), it holds, blocks added since following in default
// order, and the rounds of each series keep their order.
func Order(waves map[engine.Category][][]Block, categories []engine.Category, stored []string) []Block {
	rank := groupRanks(waves, categories, stored)
	var def []Block
	for k := 0; ; k++ {
		var step []Block
		for _, c := range categories {
			if k < len(waves[c]) {
				step = append(step, waves[c][k]...)
			}
		}
		if step == nil {
			break
		}
		slices.SortStableFunc(step, func(a, b Block) int { return rank[a.Group] - rank[b.Group] })
		def = append(def, step...)
	}
	if !slices.ContainsFunc(stored, IsRound) {
		return def
	}
	at := map[string]int{}
	for i, k := range stored {
		at[k] = i
	}
	out := slices.Clone(def)
	slices.SortStableFunc(out, func(a, b Block) int {
		ia, oka := at[a.Key]
		ib, okb := at[b.Key]
		switch {
		case oka && okb:
			return ia - ib
		case oka:
			return -1
		case okb:
			return 1
		}
		return 0
	})
	return InRoundOrder(out)
}

// groupRanks ranks the groups within a wave: those older files ordered
// first, in their order, then the others in default order.
func groupRanks(waves map[engine.Category][][]Block, categories []engine.Category, stored []string) map[string]int {
	rank := map[string]int{}
	n := 0
	for _, k := range stored {
		if !IsRound(k) {
			rank[k] = n
			n++
		}
	}
	for _, c := range categories {
		for _, wave := range waves[c] {
			for _, b := range wave {
				if _, ok := rank[b.Group]; !ok {
					rank[b.Group] = n
					n++
				}
			}
		}
	}
	return rank
}

// InRoundOrder keeps the rounds of each series in order: the places its
// blocks hold in the order are given to its rounds, first round first.
func InRoundOrder(order []Block) []Block {
	places := map[string][]int{}
	for i, b := range order {
		places[b.Series] = append(places[b.Series], i)
	}
	out := slices.Clone(order)
	for _, at := range places {
		series := make([]Block, len(at))
		for j, i := range at {
			series[j] = order[i]
		}
		slices.SortStableFunc(series, func(a, b Block) int { return a.Round - b.Round })
		for j, i := range at {
			out[i] = series[j]
		}
	}
	return out
}

// Move puts a block just before another one ("" at the end) and returns the
// new order of the keys. The rounds of the moved block's series keep their
// order. ok is false when key, or before, is not in the order.
func Move(order []Block, key, before string) (keys []string, ok bool) {
	from := slices.IndexFunc(order, func(b Block) bool { return b.Key == key })
	if from < 0 || (before != "" && !slices.ContainsFunc(order, func(b Block) bool { return b.Key == before })) {
		return nil, false
	}
	moved := order[from]
	rest := slices.Delete(slices.Clone(order), from, from+1)
	at := len(rest)
	if before != "" {
		at = slices.IndexFunc(rest, func(b Block) bool { return b.Key == before })
	}
	if key == before {
		at = from
	}
	for _, b := range InRoundOrder(slices.Insert(rest, at, moved)) {
		keys = append(keys, b.Key)
	}
	return keys, true
}
