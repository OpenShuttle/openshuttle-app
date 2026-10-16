package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const pageAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

func TestSPAHandler(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		headers map[string]string
		want    string
	}{
		{"root", "/", map[string]string{"Accept": pageAccept}, "/"},
		{"display window", "/display", map[string]string{"Accept": pageAccept}, "/"},
		{"reload on a deep route", "/tournament/control-desk", map[string]string{"Sec-Fetch-Dest": "document"}, "/"},
		{"bundle", "/main-XXEJNCO4.js", map[string]string{"Accept": "*/*"}, "/main-XXEJNCO4.js"},
		{"translations", "/i18n/fr.json", map[string]string{"Accept": "application/json"}, "/i18n/fr.json"},
		{"font", "/media/font.woff2", nil, "/media/font.woff2"},
		{"wails runtime", "/wails/runtime", map[string]string{"Accept": pageAccept}, "/wails/runtime"},
		// Dev mode: modules of the Angular dev server have no extension.
		{"vite client as module", "/@vite/client", map[string]string{"Accept": "*/*", "Sec-Fetch-Dest": "script"}, "/@vite/client"},
		{"vite client even if html is accepted", "/@vite/client", map[string]string{"Accept": pageAccept}, "/@vite/client"},
		{"extension-less module", "/chunk-abc", map[string]string{"Sec-Fetch-Dest": "script", "Accept": pageAccept}, "/chunk-abc"},
		{"fetch without html", "/display", map[string]string{"Accept": "*/*"}, "/display"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := spaHandler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.URL.Path
			}))
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tt.want {
				t.Errorf("path %q served as %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
