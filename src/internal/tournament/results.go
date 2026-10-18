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
		for _, g := range s.StageRanks(c, st.ID) {
			to := st.RouteOf(g.Rank)
			if g.Rank == 0 || to == "" {
				add(g.Entrants, st.ID, g.Rank, g.Decided && g.Rank > 0)
				continue
			}
			if next := s.stage(c, to); next != nil && !expanded[to] {
				walk(next)
			}
			// Withdrawn entrants are not routed: they stay ranked here, last.
			var stay []engine.EntrantID
			for _, id := range g.Entrants {
				if s.withdrawnBy(id, c, s.stageIndex(c, st.ID)) {
					stay = append(stay, id)
				}
			}
			add(stay, st.ID, g.Rank, g.Decided)
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
