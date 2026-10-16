package main

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

const tournamentFilter = "*" + store.TournamentExtension

// nativeDialogs shows the OS file dialogs, attached to the main window.
type nativeDialogs struct{}

func (nativeDialogs) SaveTournament(suggestedName string) (string, error) {
	app := application.Get()
	d := app.Dialog.SaveFile().
		SetFilename(suggestedName).
		AddFilter("OpenShuttle", tournamentFilter).
		CanCreateDirectories(true)
	if dir := documentsDir(); dir != "" {
		d.SetDirectory(dir)
	}
	if w, ok := app.Window.GetByName(MainWindowName); ok {
		d.AttachToWindow(w)
	}
	return d.PromptForSingleSelection()
}

func (nativeDialogs) OpenTournament() (string, error) {
	app := application.Get()
	d := app.Dialog.OpenFile().
		CanChooseFiles(true).
		AddFilter("OpenShuttle", tournamentFilter)
	if dir := documentsDir(); dir != "" {
		d.SetDirectory(dir)
	}
	if w, ok := app.Window.GetByName(MainWindowName); ok {
		d.AttachToWindow(w)
	}
	return d.PromptForSingleSelection()
}

func documentsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, "Documents")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return home
}
