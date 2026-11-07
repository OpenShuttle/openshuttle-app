//go:build windows

package keepawake

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	powerCreateRequest = kernel32.NewProc("PowerCreateRequest")
	powerSetRequest    = kernel32.NewProc("PowerSetRequest")
	powerClearRequest  = kernel32.NewProc("PowerClearRequest")
)

// Power request types (POWER_REQUEST_TYPE).
const (
	powerRequestDisplayRequired = 0
	powerRequestSystemRequired  = 1
)

// reasonContext is REASON_CONTEXT with a simple string.
type reasonContext struct {
	version uint32
	flags   uint32
	reason  *uint16
}

const (
	powerRequestContextVersion      = 0
	powerRequestContextSimpleString = 0x1
)

// start holds a power request: unlike SetThreadExecutionState it belongs to
// the process, not to a thread (goroutines move between threads). The display
// request keeps the screen on, so the idle lock never comes; the system
// request keeps the computer from sleeping.
func start(reason string) (func(), error) {
	text, err := windows.UTF16PtrFromString(reason)
	if err != nil {
		return nil, err
	}
	ctx := reasonContext{version: powerRequestContextVersion, flags: powerRequestContextSimpleString, reason: text}
	h, _, callErr := powerCreateRequest.Call(uintptr(unsafe.Pointer(&ctx)))
	if windows.Handle(h) == windows.InvalidHandle {
		return nil, fmt.Errorf("PowerCreateRequest: %w", callErr)
	}
	handle := windows.Handle(h)
	types := []uintptr{powerRequestDisplayRequired, powerRequestSystemRequired}
	for i, t := range types {
		if ok, _, callErr := powerSetRequest.Call(h, t); ok == 0 {
			for _, set := range types[:i] {
				_, _, _ = powerClearRequest.Call(h, set)
			}
			_ = windows.CloseHandle(handle)
			return nil, fmt.Errorf("PowerSetRequest: %w", callErr)
		}
	}
	return func() {
		for _, t := range types {
			_, _, _ = powerClearRequest.Call(h, t)
		}
		_ = windows.CloseHandle(handle)
	}, nil
}
