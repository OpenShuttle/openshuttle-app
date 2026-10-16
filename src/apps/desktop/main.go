package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

// EventTournamentUpdated is emitted after each appended event. Its payload is
// the new tournament version; every window refetches the views it shows.
const EventTournamentUpdated = "tournament.updated"

// EventLanguageChanged is emitted by a window when the user switches language,
// so the other window follows. Its payload is the language code ("en", "fr").
const EventLanguageChanged = "language.changed"

func init() {
	application.RegisterEvent[uint64](EventTournamentUpdated)
	application.RegisterEvent[bool](EventDisplayScreenChanged)
	application.RegisterEvent[string](EventLanguageChanged)
}

func main() {
	app := application.New(application.Options{
		Name:        "OpenShuttle",
		Description: "Run badminton club tournaments, fully offline",
		Services: []application.Service{
			application.NewService(NewDisplayScreenService()),
		},
		Assets: application.AssetOptions{
			Handler: spaHandler(application.AssetFileServerFS(assets)),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      MainWindowName,
		Title:     "OpenShuttle",
		Width:     1280,
		Height:    800,
		MinWidth:  1180,
		MinHeight: 680,
		// Matches the --color-bg token so there is no flash before Angular renders.
		BackgroundColour: application.NewRGB(0xF3, 0xF4, 0xF6),
		URL:              "/",
	})
	// Closing the control desk quits the app, display screen included.
	mainWindow.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		app.Quit()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
