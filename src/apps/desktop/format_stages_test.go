package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// While the pools of the sample are played, its bracket can still change:
// running late, the desk shortens its matches. The pools stay as they are.
func TestLaterStageChangesWhilePoolsPlay(t *testing.T) {
	f := newBareFixture(t)
	dir := filepath.Join(f.dir, "Tournois")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f.dialogs.folder = dir
	if _, err := f.settings.ChooseTournamentsDir(f.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateSampleTournament(f.ctx, "Exemple"); err != nil {
		t.Fatal(err)
	}

	fc := f.svc.GetFormatView().Categories[0]
	stages := reg(fc).Stages
	if !stages[0].Started || stages[len(stages)-1].Started {
		t.Fatalf("started = %v / %v, want pools started, bracket not", stages[0].Started, stages[len(stages)-1].Started)
	}
	configs := func() []tournament.StageConfig {
		out := []tournament.StageConfig{}
		for _, s := range reg(f.svc.GetFormatView().Categories[0]).Stages {
			out = append(out, s.StageConfig)
		}
		return out
	}
	short := engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 1, PointsPerSet: 21, Cap: 30}

	next := configs()
	next[len(next)-1].Win = short
	if err := f.svc.SetFormat(f.ctx, fc.Category, next); err != nil {
		t.Fatal(err)
	}
	if got := configs(); got[len(got)-1].Win != short {
		t.Fatalf("bracket win = %+v", got[len(got)-1].Win)
	}

	next = configs()
	next[0].Win = short
	wantCode(t, f.svc.SetFormat(f.ctx, fc.Category, next), engine.CodeStageStarted)
}
