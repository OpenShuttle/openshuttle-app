package main

import (
	"log/slog"
	"sync"
)

// awakeReason is what the system shows as the reason it stays awake.
const awakeReason = "OpenShuttle: a badminton tournament is running"

// keepAwake holds the computer awake (no sleep, no screen off or lock) while
// the keepAwake setting is on. A system refusing it is only logged: its own
// power settings then apply.
type keepAwake struct {
	start func(reason string) (stop func(), err error)
	mu    sync.Mutex
	stop  func()
}

// apply takes or releases the hold.
func (k *keepAwake) apply(on bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	switch {
	case on && k.stop == nil:
		stop, err := k.start(awakeReason)
		if err != nil {
			slog.Warn("cannot keep the computer awake", "err", err)
			return
		}
		k.stop = stop
	case !on && k.stop != nil:
		k.stop()
		k.stop = nil
	}
}

// follow wraps emit so that a settings change takes or releases the hold.
func (k *keepAwake) follow(emit func(string, any)) func(string, any) {
	return func(name string, data any) {
		emit(name, data)
		if s, ok := data.(Settings); ok && name == EventSettingsChanged {
			k.apply(s.KeepAwake)
		}
	}
}
