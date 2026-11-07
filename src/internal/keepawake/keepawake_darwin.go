//go:build darwin

package keepawake

import (
	"os"
	"os/exec"
	"strconv"
)

// start runs caffeinate, which holds the power assertions while it lives:
// -d the display (so the screen saver and its lock never come), -i idle
// sleep. -w makes it leave with this process, even after a crash.
func start(reason string) (func(), error) {
	cmd := exec.Command("/usr/bin/caffeinate", "-d", "-i", "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}, nil
}
