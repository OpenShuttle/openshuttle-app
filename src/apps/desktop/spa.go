package main

import (
	"net/http"
	"path"
	"strings"
)

// spaHandler serves index.html for Angular routes (paths whose last segment
// has no file extension, such as /display), and real files otherwise. The
// Wails asset server only falls back to index.html for "/".
func spaHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAppRoute(r.URL.Path) {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		next.ServeHTTP(w, r)
	})
}

func isAppRoute(p string) bool {
	if p == "" || p == "/" || strings.HasPrefix(p, "/wails/") {
		return false
	}
	return path.Ext(p) == ""
}
