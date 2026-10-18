package tournament

import (
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// RouteKind says which entrants of a stage a routing rule sends on.
type RouteKind string

const (
	// RouteRanks: every entrant at one of Ranks (pool position, bracket
	// place).
	RouteRanks RouteKind = "ranks"
	// RouteBestOfRank: the Count best entrants at Rank, compared across pools.
	RouteBestOfRank RouteKind = "bestOfRank"
	// RouteBest: the Count best entrants not routed yet, rank by rank, then
	// compared across pools.
	RouteBest RouteKind = "best"
	// RouteRest: every entrant not routed yet.
	RouteRest RouteKind = "rest"
)

// Limits of routing.
const (
	MaxRules = 16
	MaxRank  = 64
)

// RouteRule sends some entrants of a stage to a later stage. Rules apply in
// order, each to the entrants earlier rules left; entrants no rule takes
// keep their final rank in the stage. Stored in events and presets: keep the
// JSON names.
type RouteRule struct {
	Kind  RouteKind `json:"kind"`
	To    string    `json:"to"`
	Ranks []int     `json:"ranks,omitempty"`
	Rank  int       `json:"rank,omitempty"`
	Count int       `json:"count,omitempty"`
}

// Targets lists the stages a stage routes to, in rule order.
func (st StageConfig) Targets() []string {
	var out []string
	for _, r := range st.Routing {
		if !slices.Contains(out, r.To) {
			out = append(out, r.To)
		}
	}
	return out
}

// RanksRules turns "ranks 1 and 2 to S2, rank 3 to S3" into rules: one
// RouteRanks rule per run of ranks going to the same stage. "" keeps a rank.
func RanksRules(routing ...string) []RouteRule {
	var out []RouteRule
	for i, to := range routing {
		if to == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].To == to && out[n-1].Ranks[len(out[n-1].Ranks)-1] == i {
			out[n-1].Ranks = append(out[n-1].Ranks, i+1)
			continue
		}
		out = append(out, RouteRule{Kind: RouteRanks, To: to, Ranks: []int{i + 1}})
	}
	return out
}

// validRule checks a rule: its parameters, and a target later in the format.
func validRule(r RouteRule, position map[string]int, from int) (RouteRule, error) {
	bad := engine.NewError(engine.CodeInvalidRouting)
	if to := position[r.To]; to <= from {
		return r, bad
	}
	switch r.Kind {
	case RouteRanks:
		if len(r.Ranks) == 0 || len(r.Ranks) > MaxRank {
			return r, bad
		}
		ranks := slices.Clone(r.Ranks)
		slices.Sort(ranks)
		for i, rank := range ranks {
			if rank < 1 || rank > MaxRank || (i > 0 && ranks[i-1] == rank) {
				return r, bad
			}
		}
		return RouteRule{Kind: r.Kind, To: r.To, Ranks: ranks}, nil
	case RouteBestOfRank:
		if r.Rank < 1 || r.Rank > MaxRank || r.Count < 1 || r.Count > MaxRank {
			return r, bad
		}
		return RouteRule{Kind: r.Kind, To: r.To, Rank: r.Rank, Count: r.Count}, nil
	case RouteBest:
		if r.Count < 1 || r.Count > 2*MaxRank*MaxPools {
			return r, bad
		}
		return RouteRule{Kind: r.Kind, To: r.To, Count: r.Count}, nil
	case RouteRest:
		return RouteRule{Kind: r.Kind, To: r.To}, nil
	}
	return r, bad
}

// Ranked entrants of a stage, best first: rank by rank, then compared
// across pools. Assign sends them through the rules.
type rankedEntrant struct {
	ID   engine.EntrantID
	Rank int // 0 while unknown (bracket place not decided)
}

// assign applies the routing rules to the ranked entrants of a stage. It
// returns the stage of each routed entrant; the others keep their rank.
func assign(rules []RouteRule, ranked []rankedEntrant) map[engine.EntrantID]string {
	out := map[engine.EntrantID]string{}
	for id, i := range assignRules(rules, ranked) {
		out[id] = rules[i].To
	}
	return out
}

// assignRules returns, for each routed entrant, the index of its rule.
func assignRules(rules []RouteRule, ranked []rankedEntrant) map[engine.EntrantID]int {
	out := map[engine.EntrantID]int{}
	free := func(e rankedEntrant) bool { _, taken := out[e.ID]; return !taken && e.Rank > 0 }
	for i, r := range rules {
		taken := 0
		for _, e := range ranked {
			if !free(e) {
				continue
			}
			switch r.Kind {
			case RouteRanks:
				if !slices.Contains(r.Ranks, e.Rank) {
					continue
				}
			case RouteBestOfRank:
				if e.Rank != r.Rank || taken >= r.Count {
					continue
				}
			case RouteBest:
				if taken >= r.Count {
					continue
				}
			}
			out[e.ID] = i
			taken++
		}
	}
	return out
}

// PoolSizesFor returns the sizes of the pools of a stage for n entrants:
// the fixed number of pools split evenly, or the split from the size
// priority list. Nil when no split exists.
func (st StageConfig) PoolSizesFor(n int) []int {
	if len(st.PoolSizes) > 0 {
		return engine.PlanPools(n, st.PoolSizes)
	}
	if st.Pools < 1 || n < 2*st.Pools {
		return nil
	}
	return engine.PoolSizes(n, st.Pools)
}

// validPoolSizes accepts a priority list of distinct sizes from 2 to 16.
func validPoolSizes(sizes []int) bool {
	seen := map[int]bool{}
	for _, s := range sizes {
		if s < 2 || s > 16 || seen[s] {
			return false
		}
		seen[s] = true
	}
	return len(sizes) > 0 && len(sizes) <= 6
}
