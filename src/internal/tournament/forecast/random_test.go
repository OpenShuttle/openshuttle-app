package forecast

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// randomInput builds a random tournament to forecast: two categories, each
// with pools then a bracket waiting for them, breaks, a match on court and
// random rules.
func randomInput(r *rand.Rand) Input {
	in := Input{Now: t0, Courts: 1 + r.IntN(6), Rest: time.Duration(r.IntN(30)) * time.Minute, Rules: map[int][]Rule{}}
	cats := []engine.Category{md, wd}
	var scopes []Scope
	for _, c := range cats {
		scopes = append(scopes, Scope{Category: c}, Scope{Category: c, Stage: "S1"}, Scope{Category: c, Stage: "S2"})
		pools := 1 + r.IntN(3)
		for p := range pools {
			scopes = append(scopes, Scope{Category: c, Stage: "S1", Pool: p + 1})
			size := 2 + r.IntN(3)
			for a := range size {
				for b := a + 1; b < size; b++ {
					in.Items = append(in.Items, Item{
						Key: fmt.Sprintf("%s/S1/%d/%d", c, p, a), Category: c, Stage: "S1", Pool: p, Level: a,
						Match: fmt.Sprintf("%s-%d-%d-%d", c, p, a, b), Minutes: 10 + r.IntN(20),
						A: engine.EntrantID(fmt.Sprintf("%s%d.%d", c, p, a)), B: engine.EntrantID(fmt.Sprintf("%s%d.%d", c, p, b)),
					})
				}
			}
		}
		for level, size := range []int{4, 2} {
			scopes = append(scopes, Scope{Category: c, Stage: "S2", RoundSize: size, PlaceFrom: 1})
			for range size / 2 {
				in.Items = append(in.Items, Item{
					Key: fmt.Sprintf("%s/S2/R%d", c, level), Category: c, Stage: "S2", Pool: -1, Level: level,
					RoundSize: size, PlaceFrom: 1, After: []string{"S1"}, Minutes: 20, Pause: time.Duration(r.IntN(2)*30) * time.Minute,
				})
			}
		}
	}
	r.Shuffle(len(in.Items), func(i, j int) { in.Items[i], in.Items[j] = in.Items[j], in.Items[i] })
	for range r.IntN(3) {
		from := t0.Add(time.Duration(r.IntN(300)) * time.Minute)
		in.Breaks = append(in.Breaks, Break{From: from, To: from.Add(time.Duration(1+r.IntN(90)) * time.Minute)})
	}
	if r.IntN(3) == 0 {
		in.OnCourt = append(in.OnCourt, OnCourt{Court: 1 + r.IntN(in.Courts), Category: md, Stage: "S1", End: t0.Add(15 * time.Minute)})
	}
	kinds := []RuleKind{RulePriority, RuleOnly, RuleNever}
	for c := 1; c <= in.Courts; c++ {
		for range r.IntN(4) {
			in.Rules[c] = append(in.Rules[c], Rule{Kind: kinds[r.IntN(3)], Level: 1 + r.IntN(3), Scope: scopes[r.IntN(len(scopes))]})
		}
	}
	return in
}

// TestRunAlwaysReturns runs the forecast on random inputs: it always ends,
// and places every match.
func TestRunAlwaysReturns(t *testing.T) {
	for seed := range uint64(5000) {
		in := randomInput(rand.New(rand.NewPCG(seed, 7)))
		done := make(chan Result, 1)
		go func() { done <- Run(in) }()
		select {
		case res := <-done:
			for _, sl := range res.Slots {
				if sl.Start.After(t0.Add(7 * 24 * time.Hour)) {
					t.Fatalf("seed %d: %s placed at %v", seed, sl.Match, sl.Start)
				}
			}
			if len(res.Slots) != len(in.Items) {
				t.Fatalf("seed %d: %d matches placed out of %d", seed, len(res.Slots), len(in.Items))
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("seed %d: the forecast does not end", seed)
		}
	}
}
