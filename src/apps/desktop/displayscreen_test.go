package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestPickScreen(t *testing.T) {
	laptop := &application.Screen{ID: "laptop", IsPrimary: true}
	projector := &application.Screen{ID: "projector"}
	tv := &application.Screen{ID: "tv"}

	tests := []struct {
		name      string
		screens   []*application.Screen
		monitorID string
		want      *application.Screen
	}{
		{"requested screen", []*application.Screen{laptop, projector, tv}, "tv", tv},
		{"empty id picks first secondary", []*application.Screen{laptop, projector, tv}, "", projector},
		{"unknown id picks first secondary", []*application.Screen{laptop, projector}, "gone", projector},
		{"single screen falls back to primary", []*application.Screen{laptop}, "", laptop},
		{"no screen", nil, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickScreen(tt.screens, tt.monitorID); got != tt.want {
				t.Errorf("pickScreen() = %v, want %v", got, tt.want)
			}
		})
	}
}
