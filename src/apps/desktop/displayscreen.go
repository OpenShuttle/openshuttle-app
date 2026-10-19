package main

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	MainWindowName    = "main"
	DisplayWindowName = "display"

	// EventDisplayScreenChanged is emitted when the display screen opens or
	// closes. Its payload is true when the window is open.
	EventDisplayScreenChanged = "displayScreen.changed"
	// EventDisplayFullscreenChanged is emitted when the display screen enters
	// or leaves full screen. Its payload is true in full screen.
	EventDisplayFullscreenChanged = "displayScreen.fullscreen"
)

// Monitor describes a screen the display window can be placed on.
type Monitor struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	IsPrimary bool   `json:"isPrimary"`
}

// DisplayScreenService opens and closes the display screen: the second
// window shown on the hall monitor or projector. It opens in a window, the
// desk moves it if needed, then puts it in full screen.
type DisplayScreenService struct {
	mu         sync.Mutex
	window     *application.WebviewWindow
	fullscreen bool
}

func NewDisplayScreenService() *DisplayScreenService {
	return &DisplayScreenService{}
}

// ListMonitors returns the connected screens.
func (s *DisplayScreenService) ListMonitors() []Monitor {
	screens := application.Get().Screen.GetAll()
	monitors := make([]Monitor, 0, len(screens))
	for _, sc := range screens {
		monitors = append(monitors, Monitor{
			ID:        sc.ID,
			Name:      sc.Name,
			Width:     sc.Size.Width,
			Height:    sc.Size.Height,
			IsPrimary: sc.IsPrimary,
		})
	}
	return monitors
}

// IsOpen reports whether the display screen is open.
func (s *DisplayScreenService) IsOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.window != nil
}

// Open shows the display screen in a window, centred on the given monitor.
// An empty or unknown monitorID picks the first non-primary screen, or the
// primary one when there is a single screen. If the window is already open,
// it comes to the front.
func (s *DisplayScreenService) Open(monitorID string) {
	app := application.Get()
	screen := pickScreen(app.Screen.GetAll(), monitorID)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.window != nil {
		s.window.Show()
		s.window.Focus()
		return
	}

	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             DisplayWindowName,
		Title:            "OpenShuttle",
		Screen:           screen,
		Width:            1280,
		Height:           720,
		InitialPosition:  application.WindowCentered,
		BackgroundColour: application.NewRGB(0x0B, 0x0D, 0x12),
		URL:              "/display",
	})
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		s.mu.Lock()
		s.window = nil
		s.fullscreen = false
		s.mu.Unlock()
		app.Event.Emit(EventDisplayScreenChanged, false)
		app.Event.Emit(EventDisplayFullscreenChanged, false)
	})
	s.window = w
	s.fullscreen = false
	app.Event.Emit(EventDisplayScreenChanged, true)
	app.Event.Emit(EventDisplayFullscreenChanged, false)
}

// IsFullscreen reports whether the display screen is open in full screen.
func (s *DisplayScreenService) IsFullscreen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.window != nil && s.fullscreen
}

// SetFullscreen puts the open display screen in full screen, or back in a
// window (to move it, check it, or reach the desktop behind).
func (s *DisplayScreenService) SetFullscreen(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.window == nil {
		return
	}
	if on {
		s.window.Fullscreen()
	} else {
		s.window.UnFullscreen()
	}
	s.setFullscreenLocked(on)
}

func (s *DisplayScreenService) setFullscreenLocked(on bool) {
	if s.fullscreen != on {
		s.fullscreen = on
		application.Get().Event.Emit(EventDisplayFullscreenChanged, on)
	}
}

// Close closes the display screen if it is open.
func (s *DisplayScreenService) Close() {
	s.mu.Lock()
	w := s.window
	s.mu.Unlock()
	if w != nil {
		w.Close()
	}
}

func pickScreen(screens []*application.Screen, monitorID string) *application.Screen {
	for _, sc := range screens {
		if sc.ID == monitorID {
			return sc
		}
	}
	for _, sc := range screens {
		if !sc.IsPrimary {
			return sc
		}
	}
	if len(screens) > 0 {
		return screens[0]
	}
	return nil
}
