package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/updates"
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

func TestCheckForUpdate(t *testing.T) {
	f := newBareFixture(t)
	releases := `[{"tag_name": "relay-v1.9.0"}, {"tag_name": "desktop-v1.3.0"}, {"tag_name": "desktop-v1.2.1"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(releases))
	}))
	defer srv.Close()
	svc := NewAppService(f.app, f.rec.emit)
	svc.client, svc.url = srv.Client(), srv.URL
	saved := versionFile
	t.Cleanup(func() { versionFile = saved })

	tests := []struct {
		running string
		want    UpdateInfo
	}{
		{"1.2.0", UpdateInfo{Checked: true, Latest: "1.3.0", Available: true}},
		{"1.3.0", UpdateInfo{Checked: true, Latest: "1.3.0"}},
		{"1.4.0", UpdateInfo{Checked: true, Latest: "1.3.0"}},
		{"dev", UpdateInfo{Checked: true, Latest: "1.3.0"}},
	}
	for _, tt := range tests {
		versionFile = tt.running + "\n"
		got, err := svc.CheckForUpdate(f.ctx)
		if err != nil || got != tt.want || svc.GetUpdate() != tt.want {
			t.Errorf("running %s: %+v, %v; want %+v", tt.running, got, err, tt.want)
		}
		if last := f.rec.last(); last.name != EventUpdateChecked || last.data != tt.want {
			t.Errorf("running %s: emitted %+v", tt.running, last)
		}
	}

	// No network: an error, and what was found before stays.
	srv.Close()
	f.rec.events = nil
	if _, err := svc.CheckForUpdate(f.ctx); err == nil || svc.GetUpdate().Latest != "1.3.0" || len(f.rec.events) != 0 {
		t.Errorf("offline check: %v, %+v, %d events", err, svc.GetUpdate(), len(f.rec.events))
	}
	wantCode(t, func() error { _, err := svc.CheckForUpdate(f.ctx); return err }(), updates.CodeCheckFailed)
}

func TestAutoCheckFollowsTheSetting(t *testing.T) {
	f := newBareFixture(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`[{"tag_name": "desktop-v1.3.0"}]`))
	}))
	defer srv.Close()
	svc := NewAppService(f.app, f.rec.emit)
	svc.client, svc.url = srv.Client(), srv.URL
	saved := versionFile
	t.Cleanup(func() { versionFile = saved })
	ctx := context.Background()

	if s, _ := f.settings.GetSettings(ctx); !s.CheckUpdates {
		t.Fatal("the check is off by default")
	}
	versionFile = "dev"
	svc.autoCheck(ctx)
	if calls != 0 {
		t.Error("a dev build checked")
	}
	versionFile = "1.2.0"
	svc.autoCheck(ctx)
	if calls != 1 || !svc.GetUpdate().Available {
		t.Errorf("release build: %d calls, %+v", calls, svc.GetUpdate())
	}
	if err := f.settings.SetCheckUpdates(ctx, false); err != nil {
		t.Fatal(err)
	}
	svc.autoCheck(ctx)
	if calls != 1 {
		t.Error("checked with the setting off")
	}
}
