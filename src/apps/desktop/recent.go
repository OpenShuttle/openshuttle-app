package main

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

const (
	settingRecentTournaments = "recentTournaments"
	maxRecentTournaments     = 10
)

// RecentTournament is an entry of the home screen list.
type RecentTournament struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	OpenedAt time.Time `json:"openedAt"`
	// Exists is false when the file was moved or deleted since.
	Exists bool `json:"exists"`
	// Archived is set once the tournament was archived from this app.
	Archived bool `json:"archived"`
	// Summary is the tournament as it was when last closed; nil for
	// entries saved by older versions.
	Summary *TournamentSummary `json:"summary"`
}

func readRecent(ctx context.Context, db *store.AppDB) ([]RecentTournament, error) {
	raw, ok, err := db.Setting(ctx, settingRecentTournaments)
	if err != nil || !ok {
		return []RecentTournament{}, err
	}
	var list []RecentTournament
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		// A corrupt list is not worth blocking the user: start over.
		return []RecentTournament{}, nil
	}
	for i := range list {
		_, statErr := os.Stat(list[i].Path)
		list[i].Exists = statErr == nil
	}
	return list, nil
}

// touchRecent moves a tournament to the top of the list, most recent first.
func touchRecent(list []RecentTournament, path, name string, now time.Time) []RecentTournament {
	archived := slices.ContainsFunc(list, func(r RecentTournament) bool { return r.Path == path && r.Archived })
	var summary *TournamentSummary
	if i := slices.IndexFunc(list, func(r RecentTournament) bool { return r.Path == path }); i >= 0 {
		summary = list[i].Summary
	}
	list = slices.DeleteFunc(slices.Clone(list), func(r RecentTournament) bool { return r.Path == path })
	list = slices.Insert(list, 0, RecentTournament{Path: path, Name: name, OpenedAt: now.UTC(), Exists: true, Archived: archived, Summary: summary})
	if len(list) > maxRecentTournaments {
		list = list[:maxRecentTournaments]
	}
	return list
}

// renameRecent updates the name shown for a path, keeping its position.
func renameRecent(list []RecentTournament, path, name string) []RecentTournament {
	list = slices.Clone(list)
	for i := range list {
		if list[i].Path == path {
			list[i].Name = name
		}
	}
	return list
}

func writeRecent(ctx context.Context, db *store.AppDB, list []RecentTournament) error {
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return db.SetSetting(ctx, settingRecentTournaments, string(raw))
}

// markArchived flags a tournament of the list as archived.
func markArchived(list []RecentTournament, path string) []RecentTournament {
	list = slices.Clone(list)
	for i := range list {
		if list[i].Path == path {
			list[i].Archived = true
		}
	}
	return list
}

// setSummary updates the summary of a tournament of the list.
func setSummary(list []RecentTournament, path string, summary *TournamentSummary) []RecentTournament {
	list = slices.Clone(list)
	for i := range list {
		if list[i].Path == path {
			list[i].Summary = summary
		}
	}
	return list
}

// lastEventAt is the time of the latest event of a log.
func lastEventAt(log []events.Event) time.Time {
	if len(log) == 0 {
		return time.Time{}
	}
	return log[len(log)-1].At
}
