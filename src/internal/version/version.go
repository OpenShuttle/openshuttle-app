// Package version holds the versioning rules shared by every OpenShuttle app
// (desktop, relay, web). Versions are MAJOR.MINOR.PATCH:
//
//   - MAJOR is the compatibility generation, the same for every app: apps
//     with the same MAJOR read the same tournament files and speak the same
//     relay protocol. It changes only when that compatibility breaks, for
//     every app at once.
//   - MINOR is the version of the app itself: features and fixes.
//   - PATCH is for security fixes and dependency updates only.
package version

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// Compat is the current compatibility generation: the MAJOR of every app.
const Compat = 1

// Dev is the version of builds made outside a release.
const Dev = "dev"

// Version is a parsed MAJOR.MINOR.PATCH.
type Version struct {
	Major, Minor, Patch int
}

// CodeInvalidVersion reports a version not in the MAJOR.MINOR.PATCH form.
const CodeInvalidVersion engine.Code = "INVALID_VERSION"

// Parse reads "1.0.1". Leading zeros, signs and suffixes are refused: release
// tags must be exact.
func Parse(s string) (Version, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, engine.NewError(CodeInvalidVersion)
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 || p != strconv.Itoa(v) {
			return Version{}, engine.NewError(CodeInvalidVersion)
		}
		n[i] = v
	}
	return Version{n[0], n[1], n[2]}, nil
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compatible reports whether two apps can work together: same MAJOR.
func (v Version) Compatible(other Version) bool {
	return v.Major == other.Major
}
