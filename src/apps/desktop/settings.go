package main

import (
	"context"
	"strings"
	"sync"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

const (
	settingLanguage = "language"
	settingOperator = "operator"

	// EventSettingsChanged is emitted after a setting changes, so every
	// window applies it. Its payload is the full Settings.
	EventSettingsChanged = "settings.changed"
)

// Languages are the UI languages.
var Languages = []string{"en", "fr"}

// Settings are the app settings stored in app.db.
type Settings struct {
	// Language is empty until the user picks one; the frontend then follows
	// the system language.
	Language string `json:"language"`
	// Operator is the name recorded with each tournament action.
	Operator string `json:"operator"`
}

// SettingsService reads and writes the app settings.
type SettingsService struct {
	db   *store.AppDB
	emit func(name string, data any)
	mu   sync.Mutex
}

func NewSettingsService(db *store.AppDB, emit func(string, any)) *SettingsService {
	return &SettingsService{db: db, emit: emit}
}

// GetSettings returns the current settings.
func (s *SettingsService) GetSettings(ctx context.Context) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(ctx)
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

func (s *SettingsService) set(ctx context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.SetSetting(ctx, key, value); err != nil {
		return err
	}
	settings, err := s.read(ctx)
	if err != nil {
		return err
	}
	s.emit(EventSettingsChanged, settings)
	return nil
}

func (s *SettingsService) read(ctx context.Context) (Settings, error) {
	lang, _, err := s.db.Setting(ctx, settingLanguage)
	if err != nil {
		return Settings{}, err
	}
	op, _, err := s.db.Setting(ctx, settingOperator)
	if err != nil {
		return Settings{}, err
	}
	return Settings{Language: lang, Operator: op}, nil
}
