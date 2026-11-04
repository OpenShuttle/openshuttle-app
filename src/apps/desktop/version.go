package main

import (
	"context"
	_ "embed"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/updates"
	"github.com/OpenShuttle/openshuttle-app/src/internal/version"
)

// versionFile is "dev" in the repository; the release workflow writes the
// tag's version (MAJOR.MINOR.PATCH) into it before building.
//
//go:embed version.txt
var versionFile string

// EventUpdateChecked is emitted when a check for a newer release got an
// answer. Its payload is the UpdateInfo.
const EventUpdateChecked = "update.checked"

// updateCheckTimeout bounds a check: without network it gives up quickly.
const updateCheckTimeout = 10 * time.Second

// AppInfo describes the running app.
type AppInfo struct {
	// Version is MAJOR.MINOR.PATCH, or "dev" outside a release.
	Version string `json:"version"`
	// Compat is the compatibility generation: apps with the same Compat
	// work together. It equals the MAJOR of the version.
	Compat int `json:"compat"`
}

// UpdateInfo says whether a newer release is published. The app never
// downloads it: the home screen links to the website's download page.
type UpdateInfo struct {
	// Checked: a check got an answer since the app started.
	Checked bool `json:"checked"`
	// Latest is the newest desktop version published ("" when none).
	Latest string `json:"latest"`
	// Available: Latest is newer than the running version (never for a
	// dev build).
	Available bool `json:"available"`
}

// AppService tells the frontend about the app itself, and whether a newer
// release is published.
type AppService struct {
	db     *store.AppDB
	emit   func(name string, data any)
	client *http.Client
	url    string

	mu     sync.Mutex
	update UpdateInfo
}

func NewAppService(db *store.AppDB, emit func(string, any)) *AppService {
	return &AppService{db: db, emit: emit, client: &http.Client{Timeout: updateCheckTimeout}, url: updates.ReleasesURL}
}

// GetAppInfo returns the version of the running app.
func (*AppService) GetAppInfo() AppInfo {
	return appInfo()
}

func appInfo() AppInfo {
	return AppInfo{Version: strings.TrimSpace(versionFile), Compat: version.Compat}
}

// ServiceStartup checks for a newer release in the background, when the
// setting allows it and the app is a release build. A failure is silent.
func (s *AppService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	go s.autoCheck(context.WithoutCancel(ctx))
	return nil
}

func (s *AppService) autoCheck(ctx context.Context) {
	if _, err := version.Parse(appInfo().Version); err != nil {
		return // dev build
	}
	settings, err := loadSettings(ctx, s.db)
	if err != nil || !settings.CheckUpdates {
		return
	}
	if _, err := s.CheckForUpdate(ctx); err != nil {
		slog.Info("update check", "err", err)
	}
}

// GetUpdate returns what the last check found.
func (s *AppService) GetUpdate() UpdateInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.update
}

// CheckForUpdate asks the GitHub releases list for the newest desktop
// version, now. It works whatever the setting (the user asked), and fails
// with UPDATE_CHECK_FAILED without network.
func (s *AppService) CheckForUpdate(ctx context.Context) (UpdateInfo, error) {
	current := appInfo().Version
	ctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()
	latest, found, err := updates.Latest(ctx, s.client, s.url, "OpenShuttle/"+current)
	if err != nil {
		return s.GetUpdate(), err
	}
	info := UpdateInfo{Checked: true}
	if found {
		info.Latest = latest.String()
		if v, err := version.Parse(current); err == nil {
			info.Available = v.Less(latest)
		}
	}
	s.mu.Lock()
	s.update = info
	s.mu.Unlock()
	s.emit(EventUpdateChecked, info)
	return info, nil
}
