package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// CreateSampleTournament creates and opens a tournament to discover the app
// with: two categories of fictional pairs from fictional clubs, pools then a
// bracket, everybody checked in, the draws confirmed, two rounds played and
// the courts busy, a score pending. It is saved in the tournaments folder
// under name (translated by the frontend), numbered when the name is taken,
// and returns its path.
func (s *TournamentService) CreateSampleTournament(ctx context.Context, name string) (string, error) {
	name, err := tournament.NormalizeName(name)
	if err != nil {
		return "", err
	}
	dir := s.tournamentsDir(ctx)
	if dir == "" {
		dir = documentsDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := freePath(dir, name)

	// Built with its own clock, an hour and a half back, so the matches
	// played have real durations and those on court started minutes ago.
	clock := s.now().Add(-95 * time.Minute)
	builder := NewTournamentService(s.app, func(string, any) {}, func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}, s.backupDir, nil)
	builder.rng = s.rng
	b := &sampleBuilder{ctx: ctx, svc: builder, clock: &clock}
	b.build(path, name)
	builder.CloseTournament()
	if b.err != nil {
		_ = os.Remove(path)
		return "", b.err
	}
	return path, s.OpenTournament(ctx, path)
}

// freePath is dir/<name>.openshuttle, or "<name> (2)…" when taken.
func freePath(dir, name string) string {
	base := strings.TrimSuffix(fileNameFor(name), store.TournamentExtension)
	path := filepath.Join(dir, base+store.TournamentExtension)
	for n := 2; ; n++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return path
		}
		path = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, n, store.TournamentExtension))
	}
}

// sampleBuilder plays a tournament through the service's commands, its
// clock moved by hand between matches. The first error stops it.
type sampleBuilder struct {
	ctx   context.Context
	svc   *TournamentService
	clock *time.Time
	err   error
}

// build makes the sample tournament: a men's and a women's doubles.
func (b *sampleBuilder) build(path, name string) {
	b.do(b.svc.CreateTournamentAt(b.ctx, path, name))
	b.do(b.svc.SetCourts(b.ctx, 7))
	b.entrants(engine.MensDoubles, 12)
	b.entrants(engine.WomensDoubles, 8)
	b.format(engine.MensDoubles, 3, true)
	b.format(engine.WomensDoubles, 2, true)
	b.checkInAll()
	b.draw(engine.MensDoubles, engine.WomensDoubles)
	b.play(2)
	b.callAll()
	b.endOne()
}

func (b *sampleBuilder) do(err error) {
	if b.err == nil {
		b.err = err
	}
}

var (
	sampleClubs = []string{"Volant des Monts", "BC Rive Droite", "Plumes du Lac", "AS Belvédère"}
	sampleFirst = []string{"Lucas", "Hugo", "Paul", "Jules", "Louis", "Théo", "Nathan", "Simon", "Victor", "Kevin", "Mathis", "Félix",
		"Camille", "Léa", "Jade", "Manon", "Inès", "Chloé", "Emma", "Clara", "Alice", "Océane", "Élise", "Lou"}
	sampleLast = []string{"Perrin", "Faure", "Blanc", "Masson", "Caron", "Mathieu", "Lopez", "Vidal", "Gauthier", "Brun", "Fontaine", "Simon",
		"Moreau", "Roux", "Fournier", "Morin", "Guérin", "Henry", "Lambert", "Bertrand", "Laurent", "Dumont", "Clément", "André"}
	sampleScores = []engine.Score{
		{Sets: []engine.SetScore{{A: 15, B: 11}, {A: 15, B: 9}}},
		{Sets: []engine.SetScore{{A: 13, B: 15}, {A: 15, B: 12}, {A: 15, B: 10}}},
		{Sets: []engine.SetScore{{A: 9, B: 15}, {A: 12, B: 15}}},
		{Sets: []engine.SetScore{{A: 17, B: 15}, {A: 15, B: 8}}},
	}
)

// entrants registers n pairs of a category, men's or women's first names
// to match it.
func (b *sampleBuilder) entrants(c engine.Category, n int) {
	b.do(b.svc.AddCategory(b.ctx, c))
	women := c == engine.WomensDoubles || c == engine.WomensSingles
	for i := range n {
		k := len(b.svc.GetCheckInView().Teams) * 2 % 12
		if women {
			k += 12
		}
		player := func(j int) tournament.PlayerInput {
			return tournament.PlayerInput{
				Name: fmt.Sprintf("%s %s", sampleFirst[(k+j)%len(sampleFirst)], sampleLast[(k+j*5+i)%len(sampleLast)]),
				Club: sampleClubs[i%len(sampleClubs)],
			}
		}
		b.do(b.svc.RegisterEntrant(b.ctx, c, []tournament.PlayerInput{player(0), player(1)}))
	}
}

// stages is the format of a category.
func (b *sampleBuilder) stages(c engine.Category) []tournament.StageConfig {
	var stages []tournament.StageConfig
	for _, fc := range b.svc.GetFormatView().Categories {
		if fc.Category == c {
			for _, st := range fc.Stages {
				stages = append(stages, st.StageConfig)
			}
		}
	}
	return stages
}

// format sets pools, then a full-placement bracket for the first two of each.
func (b *sampleBuilder) format(c engine.Category, pools int, bracket bool) {
	if bracket {
		b.do(b.svc.AddStage(b.ctx, c, tournament.StageBracket))
	}
	stages := b.stages(c)
	if len(stages) == 0 {
		b.do(engine.NewError(engine.CodeCategoryNotFound))
		return
	}
	win := engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 2, PointsPerSet: 15, Cap: 21}
	stages[0].Pools, stages[0].PoolSizes, stages[0].Win = pools, nil, win
	if bracket && len(stages) > 1 {
		stages[0].Routing = tournament.RanksRules("S2", "S2")
		stages[1].Name, stages[1].Win, stages[1].FullPlacement = "Tableau principal", win, true
	}
	b.do(b.svc.SetFormat(b.ctx, c, stages))
}

func (b *sampleBuilder) checkInAll() {
	for _, team := range b.svc.GetCheckInView().Teams {
		b.do(b.svc.CheckInEntrant(b.ctx, team.EntrantID))
	}
}

func (b *sampleBuilder) draw(categories ...engine.Category) {
	for _, c := range categories {
		b.do(b.svc.DrawEntry(b.ctx, c))
		b.do(b.svc.ConfirmDraw(b.ctx, c))
	}
}

// play runs up to n rounds of calls: every proposal called, ended and
// scored; brackets are filled once their pools are over.
func (b *sampleBuilder) play(n int) {
	scored := 0
	for range n {
		called := b.callAll()
		if len(called) == 0 {
			filled := false
			for _, c := range b.svc.GetTournamentView().Categories {
				if b.svc.FillStage(b.ctx, c.Category, "S2") == nil {
					filled = true
				}
			}
			if !filled {
				return
			}
			continue
		}
		*b.clock = b.clock.Add(22 * time.Minute)
		for _, id := range called {
			b.do(b.svc.EndMatch(b.ctx, id))
			b.do(b.svc.RecordScore(b.ctx, id, sampleScores[scored%len(sampleScores)]))
			scored++
		}
		*b.clock = b.clock.Add(3 * time.Minute)
	}
}

// callAll calls every proposal of the desk.
func (b *sampleBuilder) callAll() []tournament.MatchID {
	var called []tournament.MatchID
	for _, c := range b.svc.GetControlDeskView().Courts {
		if c.Proposal != nil && b.svc.CallMatch(b.ctx, c.Proposal.ID, c.Number) == nil {
			called = append(called, c.Proposal.ID)
		}
	}
	*b.clock = b.clock.Add(9 * time.Minute)
	return called
}

// endOne ends the first match on court, its score left pending.
func (b *sampleBuilder) endOne() {
	for _, c := range b.svc.GetControlDeskView().Courts {
		if c.Match != nil {
			b.do(b.svc.EndMatch(b.ctx, c.Match.ID))
			return
		}
	}
}
