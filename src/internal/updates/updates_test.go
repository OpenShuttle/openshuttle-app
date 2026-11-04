package updates

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestLatest(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string // "" when no release is found
		fails  bool
	}{
		{"newest desktop release, other apps and drafts ignored", 200, `[
			{"tag_name": "relay-v1.9.0"},
			{"tag_name": "desktop-v1.4.0", "draft": true},
			{"tag_name": "desktop-v1.5.0-rc1", "prerelease": true},
			{"tag_name": "desktop-v1.3.0"},
			{"tag_name": "desktop-v1.10.0"},
			{"tag_name": "desktop-v1.2.1"}
		]`, "1.10.0", false},
		{"no desktop release", 200, `[{"tag_name": "relay-v1.0.0"}]`, "", false},
		{"empty list", 200, `[]`, "", false},
		{"rate limited", 403, `{"message": "API rate limit exceeded"}`, "", true},
		{"not JSON", 200, `<html>portal</html>`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") != "OpenShuttle/1.2.0" {
					t.Errorf("user agent = %q", r.Header.Get("User-Agent"))
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			v, found, err := Latest(context.Background(), srv.Client(), srv.URL, "OpenShuttle/1.2.0")
			if tt.fails {
				var e *engine.Error
				if !errors.As(err, &e) || e.Code != CodeCheckFailed {
					t.Fatalf("err = %v, want %s", err, CodeCheckFailed)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := ""; found {
				got = v.String()
				if got != tt.want {
					t.Errorf("latest = %s, want %s", got, tt.want)
				}
			} else if tt.want != "" {
				t.Errorf("no release found, want %s", tt.want)
			}
		})
	}
}

func TestLatestUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens any more: the hall has no network
	if _, _, err := Latest(context.Background(), http.DefaultClient, url, "OpenShuttle/dev"); err == nil {
		t.Fatal("no error without network")
	}
}
