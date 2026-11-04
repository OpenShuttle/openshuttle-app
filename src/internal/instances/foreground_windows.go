//go:build windows

package instances

import "golang.org/x/sys/windows"

var allowSetForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// asfwAny lets any process take the foreground.
const asfwAny = ^uintptr(0)

// allowForeground lets the holder take the foreground: Windows refuses
// SetForegroundWindow to a background process unless the foreground one
// (the one just launched by a double-click, or the instance the user clicked
// in) allows it.
func allowForeground() {
	_, _, _ = allowSetForegroundWindow.Call(asfwAny)
}
