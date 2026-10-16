package main

import (
	_ "embed"
	"strings"

	"github.com/OpenShuttle/openshuttle-app/src/internal/version"
)

// versionFile is "dev" in the repository; the release workflow writes the
// tag's version (MAJOR.MINOR.PATCH) into it before building.
//
//go:embed version.txt
var versionFile string

// AppInfo describes the running app.
type AppInfo struct {
	// Version is MAJOR.MINOR.PATCH, or "dev" outside a release.
	Version string `json:"version"`
	// Compat is the compatibility generation: apps with the same Compat
	// work together. It equals the MAJOR of the version.
	Compat int `json:"compat"`
}

// AppService tells the frontend about the app itself.
type AppService struct{}

// GetAppInfo returns the version of the running app.
func (AppService) GetAppInfo() AppInfo {
	return appInfo()
}

func appInfo() AppInfo {
	return AppInfo{Version: strings.TrimSpace(versionFile), Compat: version.Compat}
}
