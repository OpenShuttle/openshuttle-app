//go:build !windows

package instances

// allowForeground has nothing to do outside Windows.
func allowForeground() {}
