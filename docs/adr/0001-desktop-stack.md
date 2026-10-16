# 1. Desktop stack: Wails v3, Go, Angular, SQLite

- Status: accepted
- Date: 2026-10-16

## Context

OpenShuttle v1 is a desktop app that runs a badminton tournament on one PC with
no network at all. It needs two native windows (control desk and display
screen), a web-based UI, a crash-safe local database and an English and French
interface. A later v2 adds a Go relay server and an Angular website.

## Decision

- **Shell**: Wails v3, pinned to an exact beta in `src/go.mod` and
  `frontend/package.json` (`@wailsio/runtime`). It gives native windows,
  multi-window and multi-monitor support, and Go ↔ TypeScript bindings
  generated from Go services.
- **Backend**: Go, one module in `src/`. All business logic lives in Go.
- **Frontend**: Angular, in `src/apps/desktop/frontend`. The same framework will
  serve the v2 website.
- **Database**: SQLite through `modernc.org/sqlite` (pure Go, no CGO for the
  database layer).
- **i18n**: Transloco, not `@angular/localize`:
  - the language switches at runtime, in both windows, without a reload;
  - Go returns stable codes (`INVALID_SCORE`, event types, court states), and
    translating a dynamic key (`errors.` + code) is direct with Transloco,
    while `$localize` needs a hand-written mapping per enum;
  - translations are plain JSON, easy for contributors and reusable by v2.

  Transloco does not check keys at build time. CI runs `npm run i18n:check`,
  which fails when `en.json` and `fr.json` do not share the exact same keys.
- **Fonts and assets** are bundled. Nothing is loaded from the network.
- **Routing**: the main window loads `/`, the display screen loads `/display`.
  The Wails asset server only falls back to `index.html` for `/`, so the
  desktop app wraps it in `spaHandler`.

## Consequences

- Wails v3 is still in beta: upgrades are deliberate, one exact version at a
  time, with the Go module and the npm runtime bumped together.
- On Linux, Wails v3 builds against GTK 4 and WebKitGTK 6
  (`libgtk-4-dev`, `libwebkitgtk-6.0-dev`). Windows uses WebView2, macOS WKWebView.
- The desktop Go package embeds `frontend/dist`: the frontend must be built
  before `go vet` or `go test` on that package.
