package tournament

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestStagePauseValidation(t *testing.T) {
	pools := StageConfig{ID: "S1", Kind: StagePools, Win: sets21, Pools: 2, Tiebreaks: engine.DefaultTiebreaks,
		Routing: RanksRules("S2", "S2")}
	bracket := StageConfig{ID: "S2", Kind: StageBracket, Win: sets21}
	tests := []struct {
		name         string
		entry, later int
		want         engine.Code
		wantEntry    int
	}{
		{"none", 0, 0, "", 0},
		{"an hour before the bracket", 0, 60, "", 0},
		{"the entry stage has no pause", 30, 60, "", 0},
		{"negative", 0, -15, engine.CodeInvalidFormat, 0},
		{"longest", 0, MaxStagePause, "", 0},
		{"too long", 0, MaxStagePause + 1, engine.CodeInvalidFormat, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := pools, bracket
			a.Pause, b.Pause = tt.entry, tt.later
			got, err := ValidateFormat([]StageConfig{a, b})
			var e *engine.Error
			code := engine.Code("")
			if errors.As(err, &e) {
				code = e.Code
			}
			if code != tt.want {
				t.Fatalf("code = %q, want %q", code, tt.want)
			}
			if err == nil && (got[0].Pause != tt.wantEntry || got[1].Pause != tt.later) {
				t.Errorf("pauses = %d, %d", got[0].Pause, got[1].Pause)
			}
		})
	}
}

// An older FormatSet has no pause: the stages read 0.
func TestStagePauseOlderFiles(t *testing.T) {
	var fs FormatSet
	if err := json.Unmarshal([]byte(`{"category":"MS","stages":[{"id":"S2","name":"","kind":"bracket","win":{"kind":"sets","setsToWin":2,"pointsPerSet":21,"cap":30}}]}`), &fs); err != nil {
		t.Fatal(err)
	}
	if fs.Stages[0].Pause != 0 {
		t.Errorf("pause = %d", fs.Stages[0].Pause)
	}
	if b, _ := json.Marshal(StageConfig{ID: "S1"}); strings.Contains(string(b), "pause") {
		t.Errorf("no pause written when 0: %s", b)
	}
}

// stageTimes gives the first start and the last end of each stage.
func stageTimes(sc Schedule) (first, end map[string]time.Time) {
	first, end = map[string]time.Time{}, map[string]time.Time{}
	for _, sl := range sc.Slots {
		k := string(sl.Category) + "/" + sl.Stage
		end[k] = later(end[k], sl.End)
		if f, ok := first[k]; !ok || sl.Start.Before(f) {
			first[k] = sl.Start
		}
	}
	return first, end
}

func TestStagePauseSchedule(t *testing.T) {
	const ms = engine.MensSingles
	tests := []struct {
		name  string
		pause int
	}{{"no pause", 0}, {"an hour", 60}, {"a quarter", 15}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGame(t, ms)
			g.do(func(s State) (Change, error) { return s.SetCourts(4) })
			g.confirmedPools(ms, 2, 8)
			main := g.addStage(ms, StageBracket, "Main", true)
			g.editStage(ms, main, func(st *StageConfig) { st.Pause = tt.pause })
			g.route(ms, "S1", main, main)
			pause := time.Duration(tt.pause) * time.Minute

			// Forecast: the bracket starts its pause after the pools.
			first, end := stageTimes(g.state().Schedule(t0))
			if got, want := first["MS/"+main], end["MS/S1"].Add(pause); !got.Equal(want) {
				t.Errorf("forecast: bracket starts %v, want %v", got, want)
			}

			// Played: the pause counts from the real end of the pools.
			g.now = t0.Add(2 * time.Hour)
			g.finishPools(ms)
			g.fill(ms, main)
			now := g.now.Add(10 * time.Minute)
			first, _ = stageTimes(g.state().Schedule(now))
			want := later(now, g.now.Add(pause))
			if got := first["MS/"+main]; !got.Equal(want) {
				t.Errorf("played: bracket starts %v, want %v", got, want)
			}
			// No proposal before the pause is over; the match can still be called.
			if got := len(g.state().Proposals(now)); (got == 0) != (tt.pause > 10) {
				t.Errorf("proposals at %v: %d", now, got)
			}
			if got := len(g.state().Proposals(want)); got == 0 {
				t.Errorf("no proposal once the pause is over")
			}
			m := g.bracketMatches(ms, main)[0]
			g.do(func(s State) (Change, error) { return s.CallMatch(m.ID, 1) })
		})
	}
}

// With two categories, the other one keeps playing during the pause.
func TestStagePauseOtherCategoryPlays(t *testing.T) {
	const ms, ws = engine.MensSingles, engine.WomensSingles
	g := newGame(t, ms, ws)
	g.do(func(s State) (Change, error) { return s.SetCourts(4) })
	g.confirmedPools(ms, 1, 4)
	g.confirmedPools(ws, 2, 12)
	main := g.addStage(ms, StageBracket, "Main", true)
	g.editStage(ms, main, func(st *StageConfig) { st.Pause = 60 })
	g.route(ms, "S1", main, main)

	sc := g.state().Schedule(t0)
	first, end := stageTimes(sc)
	from, to := end["MS/S1"], first["MS/"+main]
	if !to.Equal(from.Add(time.Hour)) {
		t.Fatalf("bracket starts %v, pools end %v", to, from)
	}
	played := false
	for _, sl := range sc.Slots {
		if sl.Category == ws && !sl.Start.Before(from) && sl.Start.Before(to) {
			played = true
		}
	}
	if !played {
		t.Errorf("no WS match during the MS pause (%v-%v)", from, to)
	}
}
