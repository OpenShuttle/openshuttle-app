package main

import (
	"errors"
	"testing"
)

func TestKeepAwakeFollowsTheSetting(t *testing.T) {
	held, starts := 0, 0
	fail := false
	k := &keepAwake{start: func(string) (func(), error) {
		starts++
		if fail {
			return nil, errors.New("refused")
		}
		held++
		return func() { held-- }, nil
	}}
	var emitted []string
	emit := k.follow(func(name string, _ any) { emitted = append(emitted, name) })

	steps := []struct {
		name               string
		event              string
		on                 bool
		wantHeld, wantRuns int
	}{
		{"on", EventSettingsChanged, true, 1, 1},
		{"on again holds once", EventSettingsChanged, true, 1, 1},
		{"other event ignored", EventPresetsChanged, false, 1, 1},
		{"off releases", EventSettingsChanged, false, 0, 1},
		{"off again", EventSettingsChanged, false, 0, 1},
		{"on again", EventSettingsChanged, true, 1, 2},
	}
	for _, s := range steps {
		emit(s.event, Settings{KeepAwake: s.on})
		if held != s.wantHeld || starts != s.wantRuns {
			t.Fatalf("%s: held %d, starts %d; want %d, %d", s.name, held, starts, s.wantHeld, s.wantRuns)
		}
	}
	if len(emitted) != len(steps) {
		t.Errorf("emitted %d events, want %d", len(emitted), len(steps))
	}

	// A refusal is only logged, and the next change tries again.
	k.apply(false)
	fail = true
	k.apply(true)
	if k.stop != nil || held != 0 {
		t.Error("a refused hold is not kept")
	}
	fail = false
	k.apply(true)
	if held != 1 {
		t.Error("the next change tries again")
	}
}
