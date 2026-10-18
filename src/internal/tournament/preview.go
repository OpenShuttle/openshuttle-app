package tournament

import (
	"fmt"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// RankCount is a rank of a stage and how many entrants hold it: one per pool
// at a pool position, one per bracket place (several when tied).
type RankCount struct {
	Rank  int `json:"rank"`
	Count int `json:"count"`
}

// StagePreview is what a stage gives for a number of entrants, in the
// order of the format's stages.
type StagePreview struct {
	Phase    int `json:"phase"`
	Entrants int `json:"entrants"`
	// Pools: their sizes; NoSplit when the allowed sizes cannot split the
	// entrants.
	Sizes   []int `json:"sizes"`
	NoSplit bool  `json:"noSplit"`
	// Brackets: byes in the first round.
	Byes    int         `json:"byes"`
	Matches int         `json:"matches"`
	Rounds  int         `json:"rounds"`
	Ranks   []RankCount `json:"ranks"`
	// Rules counts the entrants each routing rule sends on; Kept those
	// keeping their final rank here, at places PlaceFrom..PlaceTo.
	Rules     []int `json:"rules"`
	Kept      int   `json:"kept"`
	PlaceFrom int   `json:"placeFrom"`
	PlaceTo   int   `json:"placeTo"`
}

// FormatPreview is what a format gives for a number of entrants: each stage,
// then how many entrants the full standings place and whether some share a
// rank.
type FormatPreview struct {
	Stages []StagePreview `json:"stages"`
	Ranked int            `json:"ranked"`
	Ties   bool           `json:"ties"`
}

// PreviewFormat computes what a format gives for n entrants, before anything
// is played: the pools or brackets of each stage, the routing, the matches
// and the final places. actual gives the real group sizes of stages already
// filled (pools, or {entrants} for a bracket).
func PreviewFormat(stages []StageConfig, n int, actual map[string][]int) FormatPreview {
	phases := Phases(stages)
	reach := map[string]int{}
	out := FormatPreview{Stages: make([]StagePreview, len(stages))}
	order := map[string][]previewEntrant{}
	for i, st := range stages {
		p := StagePreview{Phase: phases[st.ID], Entrants: reach[st.ID], Sizes: []int{}, Ranks: []RankCount{}, Rules: make([]int, len(st.Routing))}
		if i == 0 {
			p.Entrants = n
		}
		sizes, filled := actual[st.ID]
		if st.Kind == StagePools {
			if filled {
				p.Sizes, p.Entrants = sizes, sum(sizes)
			} else if p.Entrants > 0 {
				p.Sizes = st.PoolSizesFor(p.Entrants)
				p.NoSplit = p.Sizes == nil
				if p.Sizes == nil {
					p.Sizes = []int{}
				}
			}
			for _, size := range p.Sizes {
				p.Matches += engine.RoundRobinMatches(size)
				p.Rounds = max(p.Rounds, engine.RoundRobinRounds(size))
			}
			for r := 1; ; r++ {
				count := 0
				for _, size := range p.Sizes {
					if size >= r {
						count++
					}
				}
				if count == 0 {
					break
				}
				p.Ranks = append(p.Ranks, RankCount{Rank: r, Count: count})
			}
		} else {
			if filled && len(sizes) > 0 {
				p.Entrants = sizes[0]
			}
			if p.Entrants > 0 {
				p.Byes = engine.NextPowerOfTwo(p.Entrants) - p.Entrants
				p.Matches = engine.BracketMatches(p.Entrants, st.FullPlacement)
				p.Rounds = engine.BracketRounds(p.Entrants)
				p.Ranks = bracketRanks(p.Entrants, st.FullPlacement)
			}
		}

		// Routing, on stand-in entrants: rank by rank, as they come.
		var ranked []rankedEntrant
		for _, rc := range p.Ranks {
			for k := range rc.Count {
				ranked = append(ranked, rankedEntrant{ID: engine.EntrantID(fmt.Sprintf("%d.%d", rc.Rank, k)), Rank: rc.Rank})
			}
		}
		rules := assignRules(st.Routing, ranked)
		var entrants []previewEntrant
		for _, e := range ranked {
			pe := previewEntrant{rank: e.Rank}
			if r, ok := rules[e.ID]; ok {
				p.Rules[r]++
				pe.to = st.Routing[r].To
				reach[pe.to]++
			} else {
				p.Kept++
			}
			entrants = append(entrants, pe)
		}
		order[st.ID] = entrants
		out.Stages[i] = p
	}

	// Final places, following the routing from the entry stage.
	index := map[string]int{}
	for i, st := range stages {
		index[st.ID] = i
	}
	expanded := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		expanded[id] = true
		p := &out.Stages[index[id]]
		entrants := order[id]
		for i := 0; i < len(entrants); {
			j := i
			kept := 0
			for j < len(entrants) && entrants[j].rank == entrants[i].rank {
				if to := entrants[j].to; to != "" {
					if _, ok := index[to]; ok && !expanded[to] {
						walk(to)
					}
				} else {
					kept++
				}
				j++
			}
			if kept > 0 {
				if p.PlaceFrom == 0 {
					p.PlaceFrom = out.Ranked + 1
				}
				out.Ranked += kept
				p.PlaceTo = out.Ranked
				out.Ties = out.Ties || kept > 1
			}
			i = j
		}
	}
	if len(stages) > 0 {
		walk(stages[0].ID)
	}
	return out
}

type previewEntrant struct {
	rank int
	to   string
}

// bracketRanks lists the places of a bracket of n entrants: every place with
// full placement; otherwise 1, 2, then the losers of each round tied (3–4,
// 5–8…).
func bracketRanks(n int, full bool) []RankCount {
	var out []RankCount
	if full {
		for r := 1; r <= n; r++ {
			out = append(out, RankCount{Rank: r, Count: 1})
		}
		return out
	}
	out = append(out, RankCount{Rank: 1, Count: 1})
	if n >= 2 {
		out = append(out, RankCount{Rank: 2, Count: 1})
	}
	for p := 2; p < n; p *= 2 {
		out = append(out, RankCount{Rank: p + 1, Count: min(p, n-p)})
	}
	return out
}

func sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}
