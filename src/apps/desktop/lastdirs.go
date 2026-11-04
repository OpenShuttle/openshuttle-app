package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
)

// dialogContext names what a file dialog is for. Each context remembers the
// last folder used, per PC, in app.db (setting "lastDir.<context>").
type dialogContext string

const (
	dialogImport  dialogContext = "import"  // player list import
	dialogExport  dialogContext = "export"  // CSV exports (entrants, standings)
	dialogOpen    dialogContext = "open"    // opening a tournament
	dialogArchive dialogContext = "archive" // archive copy
	dialogNew     dialogContext = "new"     // new or duplicated tournament
)

func settingLastDir(c dialogContext) string { return "lastDir." + string(c) }

// firstExistingDir is dir, or its first existing parent, or "" when none
// exists or dir is not an absolute path (a folder remembered on another
// system, for instance).
func firstExistingDir(dir string, isDir func(string) bool) string {
	if dir == "" || !filepath.IsAbs(dir) {
		return ""
	}
	for d := filepath.Clean(dir); ; {
		if isDir(d) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// dialogFallbacks are the contexts whose folder a dialog takes, in order,
// when its own context has none yet: CSV files go together, then follow
// the tournaments.
var dialogFallbacks = map[dialogContext][]dialogContext{
	dialogImport:  {dialogImport, dialogExport, dialogOpen, dialogNew},
	dialogExport:  {dialogExport, dialogImport, dialogOpen, dialogNew},
	dialogOpen:    {dialogOpen, dialogNew, dialogArchive},
	dialogNew:     {dialogNew, dialogOpen, dialogArchive},
	dialogArchive: {dialogArchive, dialogOpen, dialogNew},
}

// dialogDir is where a file dialog for c opens: the first folder remembered
// along its fallbacks (or that folder's first existing parent), else the
// tournaments folder (Documents when unset). A new tournament opens in the
// tournaments folder when one is set in the settings.
func (s *TournamentService) dialogDir(ctx context.Context, c dialogContext) string {
	if c == dialogNew {
		if dir, _, err := s.app.Setting(ctx, settingTournamentsDir); err == nil && dir != "" && isDir(dir) {
			return dir
		}
	}
	for _, fc := range dialogFallbacks[c] {
		last, _, err := s.app.Setting(ctx, settingLastDir(fc))
		if err != nil {
			break
		}
		if dir := firstExistingDir(last, isDir); dir != "" {
			return dir
		}
	}
	return s.tournamentsDir(ctx)
}

// rememberDir stores the folder of path, chosen in a dialog for c. A failure
// is only logged: the dialog then opens in the default folder next time.
func (s *TournamentService) rememberDir(ctx context.Context, c dialogContext, path string) {
	if path == "" {
		return
	}
	if err := s.app.SetSetting(ctx, settingLastDir(c), filepath.Dir(path)); err != nil {
		slog.Error("remember dialog folder", "context", c, "err", err)
	}
}
