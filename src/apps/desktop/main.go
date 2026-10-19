package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

// EventTournamentUpdated is emitted after each appended event, and when a
// tournament is opened or closed. Its payload is the tournament version (0
// when none is open); every window refetches the views it shows.
const EventTournamentUpdated = "tournament.updated"

func init() {
	application.RegisterEvent[uint64](EventTournamentUpdated)
	application.RegisterEvent[bool](EventDisplayScreenChanged)
	application.RegisterEvent[Settings](EventSettingsChanged)
	application.RegisterEvent[Presets](EventPresetsChanged)
}

func main() {
	dataDir, err := appDataDir()
	if err != nil {
		log.Fatal(err)
	}
	appDB, err := store.OpenAppDB(context.Background(), filepath.Join(dataDir, "app.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer appDB.Close()

	emit := func(name string, data any) {
		if app := application.Get(); app != nil {
			app.Event.Emit(name, data)
		}
	}

	app := application.New(application.Options{
		Name:        "OpenShuttle",
		Description: "Run badminton club tournaments, fully offline",
		Services: []application.Service{
			service(&AppService{}),
			service(NewDisplayScreenService()),
			service(NewSettingsService(appDB, emit, nativeDialogs{})),
			service(NewPresetService(appDB, emit, time.Now)),
			service(NewTournamentService(
				appDB, emit, time.Now, filepath.Join(dataDir, "backups"), nativeDialogs{},
			)),
		},
		Assets: application.AssetOptions{
			Handler: spaHandler(application.AssetFileServerFS(assets)),
		},
		MarshalError: marshalError,
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
		// The app's top bar is the title bar: frameless on Windows and Linux
		// (it draws its own window buttons), native traffic lights over it on
		// macOS. It drags the window and maximises on double-click.
		Frameless: runtime.GOOS != "darwin",
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 64,
		},
	})
	// Closing the control desk quits the app, display screen included.
	mainWindow.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		app.Quit()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// service binds a service with our error marshaler. Wails v3 beta.30 ignores
// the app-level MarshalError for bound methods: it must be set per service.
func service[T any](instance *T) application.Service {
	return application.NewServiceWithOptions(instance, application.ServiceOptions{MarshalError: marshalError})
}

// appDataDir is where app.db and backups live: the user config folder
// (%AppData% on Windows, ~/Library/Application Support on macOS,
// ~/.config on Linux).
func appDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "OpenShuttle")
	return dir, os.MkdirAll(dir, 0o755)
}
