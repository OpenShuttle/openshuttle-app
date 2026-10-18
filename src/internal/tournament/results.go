package tournament

import (
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// Ranked is a line of a category's full standings.
type Ranked struct {
	// Rank is the final position, 0 while not decided yet.
	Rank int
	// Tied is set when other entrants share this rank (entrants losing in
	// the same round of a bracket without full placement, or kept at the
	// same pool position).
	Tied    bool
	Entrant engine.EntrantID
	// Stage is where the rank was earned, StageRank the rank there (pool
	// position or bracket place); 0 when the stage is not filled yet.
	Stage     string
	StageRank int
}

// FinalStandings ranks every drawn entrant of a category by following the
// routing from the entry stage: rank by rank, a rank routed to another stage
// brings that stage's standings (once), a kept rank is the entrants at that
// rank, tied across pools. complete is set when every rank is final.
func (s State) FinalStandings(c engine.Category) (rows []Ranked, complete bool) {
	entry := s.entry(c)
	if entry == nil || !entry.Confirmed {
		return nil, false
	}
	complete = true
	offset := 0
	expanded := map[string]bool{}
	add := func(ids []engine.EntrantID, stage string, stageRank int, decided bool) {
		for _, id := range ids {
			r := Ranked{Tied: len(ids) > 1, Entrant: id, Stage: stage, StageRank: stageRank}
			if decided {
				r.Rank = offset + 1
			} else {
				complete = false
			}
			rows = append(rows, r)
		}
		offset += len(ids)
	}

	var walk func(st *Stage)
	walk = func(st *Stage) {
		expanded[st.ID] = true
		if !st.Confirmed {
			// Not filled yet: who goes there is known, not their rank.
			var ids []engine.EntrantID
			for _, e := range s.Routed(c, st.ID) {
				ids = append(ids, e.ID)
			}
			for _, id := range ids {
				rows = append(rows, Ranked{Entrant: id, Stage: st.ID})
			}
			offset += len(ids)
			complete = false
			return
		}
		to := s.Assignment(c, st.ID)
		ranked := s.rankedEntrants(c, st)
		decided := map[int]bool{}
		for _, g := range s.StageRanks(c, st.ID) {
			decided[g.Rank] = g.Decided
		}
		for i := 0; i < len(ranked); {
			j := i
			for j < len(ranked) && ranked[j].Rank == ranked[i].Rank {
				j++
			}
			// Entrants routed on bring their stage's standings, in order;
			// the others share this rank.
			var kept []engine.EntrantID
			for _, e := range ranked[i:j] {
				if dest := to[e.ID]; dest != "" {
					if next := s.stage(c, dest); next != nil && !expanded[dest] {
						walk(next)
					}
					continue
				}
				kept = append(kept, e.ID)
			}
			rank := ranked[i].Rank
			add(kept, st.ID, rank, rank > 0 && decided[rank])
			i = j
		}
	}
	walk(entry)

	slices.SortStableFunc(rows, func(a, b Ranked) int {
		switch {
		case a.Rank == b.Rank:
			return 0
		case a.Rank == 0:
			return 1
		case b.Rank == 0:
			return -1
		}
		return a.Rank - b.Rank
	})
	return rows, complete
}
