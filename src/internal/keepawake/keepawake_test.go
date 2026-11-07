package keepawake

import "testing"

// The hold needs a desktop session (a session bus on Linux): where there is
// none, as in CI, the test only checks that Start fails cleanly.
func TestStartStop(t *testing.T) {
	stop, err := Start("OpenShuttle test")
	if err != nil {
		if stop != nil {
			t.Error("a failed Start returns no stop")
		}
		t.Skipf("no way to stay awake here: %v", err)
	}
	stop()
}
