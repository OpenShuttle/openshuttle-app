// Package keepawake keeps the computer awake: while held, the system does not
// sleep and the screen neither turns off nor locks, so the control desk and
// the hall display stay up during a tournament.
//
// Each system has its own call: a power request on Windows, caffeinate on
// macOS, the desktop portal (or the screensaver service) on Linux. It never
// needs the network, and a failure only means the system's own settings apply.
package keepawake

import "errors"

// ErrUnsupported is returned where the system offers no way to stay awake.
var ErrUnsupported = errors.New("keepawake: unsupported on this system")

// Start keeps the computer awake until the returned stop is called, giving
// reason to the system (shown by tools listing what blocks sleep). The hold
// also ends when the process exits, even after a crash.
func Start(reason string) (stop func(), err error) {
	return start(reason)
}
