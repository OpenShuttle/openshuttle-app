package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// Draw statuses.
const (
	DrawNone      = "none"
	DrawDraft     = "draft"
	DrawConfirmed = "confirmed"
)

// DrawEntrantView is a card of the draw screen.
type DrawEntrantView struct {
	ID      engine.EntrantID `json:"id"`
	Bib     int              `json:"bib"`
	Seed    int              `json:"seed"`
	Players []PlayerView     `json:"players"`
	Clubs   []string         `json:"clubs"`
	// SameClub lists the bibs of pool mates sharing a club.
	SameClub []int `json:"sameClub"`
	// Absent is set when a drawn entrant is no longer checked in.
	Absent bool `json:"absent"`
}

// DrawPoolView is a column of the draw screen.
type DrawPoolView struct {
	Name       string            `json:"name"`
	Entrants   []DrawEntrantView `json:"entrants"`
	Unbalanced bool              `json:"unbalanced"`
	Seeds      int               `json:"seeds"`
}

// DrawCategory is one category of the draw screen: its entry stage.
type DrawCategory struct {
	CategoryRef
	// Kind is the entry stage's: pools, or a bracket whose single "pool"
	// lists the first-round slots (a bye has an empty ID).
	Kind    tournament.StageKind `json:"kind"`
	Stage   string               `json:"stage"`
	Status  string               `json:"status"`
	Pools   []DrawPoolView       `json:"pools"`
	Planned int                  `json:"planned"` // pools in the format
	Present int                  `json:"present"`
	// CanDraw is false below two present entrants, or when no pool split
	// fits the present entrants (NoSplit, with the allowed PoolSizes).
	CanDraw   bool  `json:"canDraw"`
	NoSplit   bool  `json:"noSplit"`
	PoolSizes []int `json:"poolSizes"`
	// Unplaced are present entrants missing from the draw.
	Unplaced []DrawEntrantView `json:"unplaced"`
	// Ranking lists the seeds, top seed first; Unseeded the present
	// entrants not seeded, to pick seeds from.
	Ranking  []DrawEntrantView `json:"ranking"`
	Unseeded []DrawEntrantView `json:"unseeded"`
}

// DrawView is the draw screen.
type DrawView struct {
	Categories []DrawCategory `json:"categories"`
}

// SetSeeds ranks the seeds of a category, top seed first; the entrants
// left out are not seeded.
func (s *TournamentService) SetSeeds(ctx context.Context, c engine.Category, ranking []engine.EntrantID) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.SetSeeds(c, ranking) })
}

// DrawEntry draws (or redraws) the entry stage of a category: its pools, or
// the slots of its bracket.
func (s *TournamentService) DrawEntry(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.DrawEntry(c, s.rng()) })
}

// MoveEntrant moves an entrant to another pool (0 is A).
func (s *TournamentService) MoveEntrant(ctx context.Context, id engine.EntrantID, pool int) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.MoveEntrant(id, pool) })
}

// ConfirmDraw locks the pools of a category.
func (s *TournamentService) ConfirmDraw(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ConfirmDraw(c) })
}

// ReopenDraw unlocks a confirmed draw.
func (s *TournamentService) ReopenDraw(ctx context.Context, c engine.Category) error {
	return s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.ReopenDraw(c) })
}

// GetDrawView returns the draw screen.
func (s *TournamentService) GetDrawView() DrawView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	dup := st.DuplicatePlayers()
	card := func(id engine.EntrantID) DrawEntrantView {
		if id == "" {
			return DrawEntrantView{Players: []PlayerView{}, Clubs: []string{}, SameClub: []int{}}
		}
		e, _ := st.Entrant(id)
		v := DrawEntrantView{ID: id, Bib: e.Bib, Seed: e.Seed, Clubs: e.Clubs(), SameClub: []int{}, Absent: !e.Complete()}
		for _, p := range e.Players {
			v.Players = append(v.Players, playerView(p, dup[p.ID]))
		}
		if v.Clubs == nil {
			v.Clubs = []string{}
		}
		return v
	}

	v := DrawView{Categories: []DrawCategory{}}
	for _, ref := range categoryRefs(st) {
		present := st.Present(ref.Category)
		entry := st.Stages(ref.Category)[0]
		sizes := entry.PoolSizesFor(len(present))
		dc := DrawCategory{
			CategoryRef: ref, Kind: entry.Kind, Stage: entry.ID, Status: DrawNone, Planned: len(sizes),
			Present: len(present), CanDraw: len(present) >= 2 && (entry.Kind == tournament.StageBracket || sizes != nil),
			Pools: []DrawPoolView{}, Unplaced: []DrawEntrantView{}, Ranking: []DrawEntrantView{}, Unseeded: []DrawEntrantView{},
			NoSplit:   entry.Kind == tournament.StagePools && len(present) >= 2 && sizes == nil,
			PoolSizes: append([]int{}, entry.PoolSizes...),
		}
		for _, id := range st.Seeds(ref.Category) {
			dc.Ranking = append(dc.Ranking, card(id))
		}
		for _, e := range present {
			if e.Seed == 0 {
				dc.Unseeded = append(dc.Unseeded, card(e.ID))
			}
		}
		if d, ok := st.Draw(ref.Category); ok {
			dc.Status = DrawDraft
			if d.Confirmed {
				dc.Status = DrawConfirmed
			}
			check := st.CheckDraw(ref.Category)
			for i, pool := range d.Pools {
				pv := DrawPoolView{
					Name: engine.PoolName(i), Entrants: []DrawEntrantView{},
					Unbalanced: check.Pools[i].Unbalanced, Seeds: check.Pools[i].Seeds,
				}
				for _, id := range pool {
					c := card(id)
					for _, mate := range check.Pools[i].SameClub[id] {
						if m, ok := st.Entrant(mate); ok {
							c.SameClub = append(c.SameClub, m.Bib)
						}
					}
					pv.Entrants = append(pv.Entrants, c)
				}
				dc.Pools = append(dc.Pools, pv)
			}
			for _, id := range check.Unplaced {
				dc.Unplaced = append(dc.Unplaced, card(id))
			}
		}
		v.Categories = append(v.Categories, dc)
	}
	return v
}

// cryptoRand returns a generator seeded from the OS. The seed is not stored:
// the draw event records the resulting pools.
func cryptoRand() *rand.Rand {
	var b [16]byte
	_, _ = cryptorand.Read(b[:])
	return rand.New(rand.NewPCG(binary.LittleEndian.Uint64(b[:8]), binary.LittleEndian.Uint64(b[8:])))
}
