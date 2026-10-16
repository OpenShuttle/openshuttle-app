package main

import (
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/version"
)

// A release build embeds a version whose MAJOR is the compatibility
// generation; the release workflow also checks it before building.
func TestEmbeddedVersion(t *testing.T) {
	info := appInfo()
	if info.Version == version.Dev {
		return
	}
	v, err := version.Parse(info.Version)
	if err != nil || v.Major != version.Compat || info.Compat != version.Compat {
		t.Errorf("embedded version %q does not match compat %d", info.Version, version.Compat)
	}
}
