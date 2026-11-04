package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
)

// settingTrashedTournaments holds the tournaments moved to the trash from the
// home screen. It is a list of its own, not capped like the recent list, so
// nothing leaves the trash unless the user says so.
const settingTrashedTournaments = "trashedTournaments"

func readTrash(ctx context.Context, db store.Settings) ([]RecentTournament, error) {
	raw, ok, err := db.Setting(ctx, settingTrashedTournaments)
	if err != nil || !ok {
		return []RecentTournament{}, err
	}
	var list []RecentTournament
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		// A corrupt trash is not worth blocking the user: start over.
		return []RecentTournament{}, nil
	}
	for i := range list {
		_, statErr := os.Stat(list[i].Path)
		list[i].Exists = statErr == nil
	}
	return list, nil
}

func writeTrash(ctx context.Context, db store.Settings, list []RecentTournament) error {
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return db.SetSetting(ctx, settingTrashedTournaments, string(raw))
}

func withoutPath(list []RecentTournament, path string) []RecentTournament {
	return slices.DeleteFunc(slices.Clone(list), func(r RecentTournament) bool { return r.Path == path })
}

// ListTrashedTournaments returns the tournaments in the trash, most recently
// trashed first.
func (s *TournamentService) ListTrashedTournaments(ctx context.Context) ([]RecentTournament, error) {
	return readTrash(ctx, s.app)
}

// TrashTournament moves a tournament of the recent list to the trash. Its file
// stays on disk. The open tournament is closed first.
func (s *TournamentService) TrashTournament(ctx context.Context, path string) error {
	s.closeIfOpen(path)
	return s.app.Update(ctx, func(tx store.Settings) error {
		recent, err := readRecent(ctx, tx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(recent, func(r RecentTournament) bool { return r.Path == path })
		if i < 0 {
			return nil // already out of the list
		}
		entry := recent[i]
		at := s.now().UTC()
		entry.TrashedAt = &at
		trash, err := readTrash(ctx, tx)
		if err != nil {
			return err
		}
		if err := writeTrash(ctx, tx, slices.Insert(withoutPath(trash, path), 0, entry)); err != nil {
			return err
		}
		return writeRecent(ctx, tx, withoutPath(recent, path))
	})
}

// RestoreTournament puts a tournament of the trash back at the top of the
// recent list.
func (s *TournamentService) RestoreTournament(ctx context.Context, path string) error {
	return s.app.Update(ctx, func(tx store.Settings) error {
		trash, err := readTrash(ctx, tx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(trash, func(r RecentTournament) bool { return r.Path == path })
		if i < 0 {
			return engine.NewError(engine.CodeNotInTrash)
		}
		entry := trash[i]
		entry.TrashedAt = nil
		recent, err := readRecent(ctx, tx)
		if err != nil {
			return err
		}
		recent = slices.Insert(withoutPath(recent, path), 0, entry)
		if len(recent) > maxRecentTournaments {
			recent = recent[:maxRecentTournaments]
		}
		if err := writeRecent(ctx, tx, recent); err != nil {
			return err
		}
		return writeTrash(ctx, tx, withoutPath(trash, path))
	})
}

// DeleteTournament removes a tournament from the trash for good. With
// deleteFile, its file is deleted from disk too, with its -wal and -shm
// siblings; backups are kept. A file open in another instance of the app is
// kept (FILE_IN_USE), and so is one that cannot be deleted (DELETE_FAILED):
// the tournament then stays in the trash.
func (s *TournamentService) DeleteTournament(ctx context.Context, path string, deleteFile bool) error {
	trash, err := readTrash(ctx, s.app)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(trash, func(r RecentTournament) bool { return r.Path == path }) {
		return engine.NewError(engine.CodeNotInTrash)
	}
	if deleteFile {
		s.closeIfOpen(path)
		if err := store.RemoveTournament(path); err != nil {
			var e *engine.Error
			if errors.As(err, &e) {
				return err
			}
			slog.Error("delete tournament file", "path", path, "err", err)
			return engine.NewError(engine.CodeDeleteFailed)
		}
	}
	return s.app.Update(ctx, func(tx store.Settings) error {
		trash, err := readTrash(ctx, tx)
		if err != nil {
			return err
		}
		return writeTrash(ctx, tx, withoutPath(trash, path))
	})
}

// untrash takes a tournament out of the trash when it is opened again.
func (s *TournamentService) untrash(ctx context.Context, path string) {
	_ = s.app.Update(ctx, func(tx store.Settings) error {
		trash, err := readTrash(ctx, tx)
		if err != nil || !slices.ContainsFunc(trash, func(r RecentTournament) bool { return r.Path == path }) {
			return err
		}
		return writeTrash(ctx, tx, withoutPath(trash, path))
	})
}

// closeIfOpen closes the tournament when it is the open one.
func (s *TournamentService) closeIfOpen(path string) {
	s.mu.Lock()
	open := s.file != nil && sameFile(s.file.Path(), path)
	s.mu.Unlock()
	if open {
		s.CloseTournament()
	}
}
