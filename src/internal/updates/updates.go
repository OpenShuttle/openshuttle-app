// Package updates finds out whether a newer desktop release is published.
// It only reads the list of GitHub releases: nothing is downloaded or
// installed, and a failed check (no network in the hall) is only an
// unanswered question, never an obstacle.
package updates

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/version"
)

// ReleasesURL lists the releases of the app's repository, newest first.
// Desktop releases are tagged desktop-vX.Y.Z; other apps share the list.
const ReleasesURL = "https://api.github.com/repos/OpenShuttle/openshuttle-app/releases?per_page=30"

// CodeCheckFailed reports a check that got no usable answer.
const CodeCheckFailed engine.Code = "UPDATE_CHECK_FAILED"

const (
	tagPrefix = "desktop-v"
	// maxBody bounds the answer read: the list is a few tens of kilobytes.
	maxBody = 2 << 20
)

type release struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// Latest returns the newest published desktop version listed at url, and
// false when there is none. Drafts, prereleases and other apps' tags are
// left out.
func Latest(ctx context.Context, client *http.Client, url, userAgent string) (version.Version, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return version.Version{}, false, engine.NewError(CodeCheckFailed)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return version.Version{}, false, engine.NewError(CodeCheckFailed)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return version.Version{}, false, engine.NewError(CodeCheckFailed)
	}
	var releases []release
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&releases); err != nil {
		return version.Version{}, false, engine.NewError(CodeCheckFailed)
	}
	return newest(releases)
}

// newest picks the highest desktop version among published releases.
func newest(releases []release) (version.Version, bool, error) {
	var best version.Version
	found := false
	for _, r := range releases {
		if r.Draft || r.Prerelease || !strings.HasPrefix(r.Tag, tagPrefix) {
			continue
		}
		v, err := version.Parse(strings.TrimPrefix(r.Tag, tagPrefix))
		if err != nil {
			continue
		}
		if !found || best.Less(v) {
			best, found = v, true
		}
	}
	return best, found, nil
}
