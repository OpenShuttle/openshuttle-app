package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSPAHandler(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/", "/"},
		{"/display", "/"},
		{"/tournament/control-desk", "/"},
		{"/main-XXEJNCO4.js", "/main-XXEJNCO4.js"},
		{"/i18n/fr.json", "/i18n/fr.json"},
		{"/media/font.woff2", "/media/font.woff2"},
		{"/wails/runtime", "/wails/runtime"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var got string
			h := spaHandler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.URL.Path
			}))
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.path, nil))
			if got != tt.want {
				t.Errorf("path %q served as %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
