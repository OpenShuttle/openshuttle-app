package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

const (
	settingLanguage        = "language"
	settingOperator        = "operator"
	settingMonitor         = "monitor"
	settingRotationSeconds = "rotationSeconds"
	settingDefaultCourts   = "defaultCourts"
	settingDefaultMinRest  = "defaultMinRest"
	settingDefaultPoolWin  = "defaultPoolWin"
	settingTournamentsDir  = "tournamentsDir"

	// EventSettingsChanged is emitted after a setting changes, so every
	// window applies it. Its payload is the full Settings.
	EventSettingsChanged = "settings.changed"
)

// Limits and defaults of the settings.
const (
	DefaultRotationSeconds = 8
	MinRotationSeconds     = 4
	MaxRotationSeconds     = 30
)

// Languages are the UI languages.
var Languages = []string{"en", "fr"}

// Settings are the app settings stored in app.db, for every tournament.
type Settings struct {
	// Language is empty until the user picks one; the frontend then follows
	// the system language.
	Language string `json:"language"`
	// Operator is the name recorded with each tournament action.
	Operator string `json:"operator"`
	// Monitor is the screen of the display window, "" for automatic.
	Monitor string `json:"monitor"`
	// RotationSeconds is how long the display screen shows each standings
	// view before the next one.
	RotationSeconds int `json:"rotationSeconds"`
	// Defaults of new tournaments.
	DefaultCourts  int    `json:"defaultCourts"`
	DefaultMinRest int    `json:"defaultMinRest"`
	DefaultPoolWin string `json:"defaultPoolWin"` // "sets" or "timed"
	// TournamentsDir is where the file dialogs open.
	TournamentsDir string `json:"tournamentsDir"`
}

// SettingsService reads and writes the app settings.
type SettingsService struct {
	db      *store.AppDB
	emit    func(name string, data any)
	dialogs fileDialogs
	mu      sync.Mutex
}

func NewSettingsService(db *store.AppDB, emit func(string, any), dialogs fileDialogs) *SettingsService {
	return &SettingsService{db: db, emit: emit, dialogs: dialogs}
}

// GetSettings returns the current settings, defaults filled in.
func (s *SettingsService) GetSettings(ctx context.Context) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return loadSettings(ctx, s.db)
}

// SetLanguage sets the UI language ("en" or "fr").
func (s *SettingsService) SetLanguage(ctx context.Context, lang string) error {
	valid := false
	for _, l := range Languages {
		valid = valid || l == lang
	}
	if !valid {
		return engine.NewError(engine.CodeInvalidSetting)
	}
	return s.set(ctx, settingLanguage, lang)
}

// SetOperator sets the name recorded with each action.
func (s *SettingsService) SetOperator(ctx context.Context, name string) error {
	name = strings.Join(strings.Fields(name), " ")
	if len([]rune(name)) > 50 {
		return engine.NewError(engine.CodeInvalidSetting)
	}
	return s.set(ctx, settingOperator, name)
}

// SetMonitor sets the screen of the display window, "" for automatic.
func (s *SettingsService) SetMonitor(ctx context.Context, id string) error {
	if len(id) > 200 {
		return engine.NewError(engine.CodeInvalidSetting)
	}
	return s.set(ctx, settingMonitor, id)
}

// SetRotationSeconds sets how long each standings view stays on the display screen.
func (s *SettingsService) SetRotationSeconds(ctx context.Context, seconds int) error {
	return s.setInt(ctx, settingRotationSeconds, seconds, MinRotationSeconds, MaxRotationSeconds)
}

// SetDefaultCourts sets the number of courts of new tournaments.
func (s *SettingsService) SetDefaultCourts(ctx context.Context, courts int) error {
	return s.setInt(ctx, settingDefaultCourts, courts, 1, tournament.MaxCourts)
}

// SetDefaultMinRest sets the minimum rest of new tournaments, in minutes.
func (s *SettingsService) SetDefaultMinRest(ctx context.Context, minutes int) error {
	return s.setInt(ctx, settingDefaultMinRest, minutes, 0, tournament.MaxMinRest)
}

// SetDefaultPoolWin sets the pool win condition of new tournaments: "sets"
// or "timed" (timed applies while a single category plays).
func (s *SettingsService) SetDefaultPoolWin(ctx context.Context, kind string) error {
	if kind != string(engine.WinBySets) && kind != string(engine.WinByTime) {
		return engine.NewError(engine.CodeInvalidSetting)
	}
	return s.set(ctx, settingDefaultPoolWin, kind)
}

// ChooseTournamentsDir asks for the folder where file dialogs open. It
// returns the folder, or "" when the user cancelled.
func (s *SettingsService) ChooseTournamentsDir(ctx context.Context) (string, error) {
	current, err := s.GetSettings(ctx)
	if err != nil {
		return "", err
	}
	dir, err := s.dialogs.ChooseFolder(current.TournamentsDir)
	if err != nil || dir == "" {
		return "", err
	}
	return dir, s.set(ctx, settingTournamentsDir, dir)
}

func (s *SettingsService) setInt(ctx context.Context, key string, value, lo, hi int) error {
	if value < lo || value > hi {
		return engine.NewError(engine.CodeInvalidSetting)
	}
	return s.set(ctx, key, strconv.Itoa(value))
}

func (s *SettingsService) set(ctx context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.SetSetting(ctx, key, value); err != nil {
		return err
	}
	settings, err := loadSettings(ctx, s.db)
	if err != nil {
		return err
	}
	s.emit(EventSettingsChanged, settings)
	return nil
}

// loadSettings reads every setting, defaults filled in.
func loadSettings(ctx context.Context, db *store.AppDB) (Settings, error) {
	raw := map[string]string{}
	for _, key := range []string{
		settingLanguage, settingOperator, settingMonitor, settingRotationSeconds,
		settingDefaultCourts, settingDefaultMinRest, settingDefaultPoolWin, settingTournamentsDir,
	} {
		v, _, err := db.Setting(ctx, key)
		if err != nil {
			return Settings{}, err
		}
		raw[key] = v
	}
	number := func(key string, def int) int {
		if n, err := strconv.Atoi(raw[key]); err == nil {
			return n
		}
		return def
	}
	s := Settings{
		Language:        raw[settingLanguage],
		Operator:        raw[settingOperator],
		Monitor:         raw[settingMonitor],
		RotationSeconds: number(settingRotationSeconds, DefaultRotationSeconds),
		DefaultCourts:   number(settingDefaultCourts, tournament.DefaultCourts),
		DefaultMinRest:  number(settingDefaultMinRest, tournament.DefaultMinRest),
		DefaultPoolWin:  raw[settingDefaultPoolWin],
		TournamentsDir:  raw[settingTournamentsDir],
	}
	if s.DefaultPoolWin == "" {
		s.DefaultPoolWin = string(engine.WinBySets)
	}
	if info, err := os.Stat(s.TournamentsDir); s.TournamentsDir == "" || err != nil || !info.IsDir() {
		s.TournamentsDir = documentsDir()
	}
	return s, nil
}

// documentsDir is the user's Documents folder, or their home folder.
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
