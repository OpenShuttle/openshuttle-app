package main

import (
	"net/http"
	"path"
	"strings"
)

// spaHandler serves index.html when the webview navigates to an Angular
// route (such as /display), and passes every other request through. The
// Wails asset server only falls back to index.html for "/".
//
// Only page navigations are rewritten: in dev mode the Angular dev server
// also serves extension-less modules (/@vite/client) that must reach it
// untouched.
func spaHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAppRoute(r) {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		next.ServeHTTP(w, r)
	})
}

func isAppRoute(r *http.Request) bool {
	p := r.URL.Path
	if p == "" || p == "/" || strings.HasPrefix(p, "/wails/") || strings.HasPrefix(p, "/@") {
		return false
	}
	if path.Ext(p) != "" {
		return false
	}
	return isNavigation(r)
}

// isNavigation reports whether the webview is loading a page rather than a
// script, style or data.
func isNavigation(r *http.Request) bool {
	if dest := r.Header.Get("Sec-Fetch-Dest"); dest != "" {
		return dest == "document"
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}
