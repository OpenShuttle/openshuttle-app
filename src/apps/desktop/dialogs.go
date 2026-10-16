package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

const tournamentFilter = "*" + store.TournamentExtension

// nativeDialogs shows the OS file dialogs, attached to the main window and
// opened in the given folder.
type nativeDialogs struct{}

func (nativeDialogs) SaveTournament(dir, suggestedName string) (string, error) {
	return saveFile(dir, suggestedName, "OpenShuttle", tournamentFilter)
}

func (nativeDialogs) SaveCSV(dir, suggestedName string) (string, error) {
	return saveFile(dir, suggestedName, "CSV", "*.csv")
}

func (nativeDialogs) OpenTournament(dir string) (string, error) {
	app := application.Get()
	d := app.Dialog.OpenFile().
		CanChooseFiles(true).
		AddFilter("OpenShuttle", tournamentFilter)
	if dir != "" {
		d.SetDirectory(dir)
	}
	if w, ok := app.Window.GetByName(MainWindowName); ok {
		d.AttachToWindow(w)
	}
	return d.PromptForSingleSelection()
}

func (nativeDialogs) ChooseFolder(dir string) (string, error) {
	app := application.Get()
	d := app.Dialog.OpenFile().
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true)
	if dir != "" {
		d.SetDirectory(dir)
	}
	if w, ok := app.Window.GetByName(MainWindowName); ok {
		d.AttachToWindow(w)
	}
	return d.PromptForSingleSelection()
}

func saveFile(dir, suggestedName, filterName, pattern string) (string, error) {
	app := application.Get()
	d := app.Dialog.SaveFile().
		SetFilename(suggestedName).
		AddFilter(filterName, pattern).
		CanCreateDirectories(true)
	if dir != "" {
		d.SetDirectory(dir)
	}
	if w, ok := app.Window.GetByName(MainWindowName); ok {
		d.AttachToWindow(w)
	}
	return d.PromptForSingleSelection()
}
