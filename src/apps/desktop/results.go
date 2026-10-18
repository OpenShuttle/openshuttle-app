package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/store"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament"
)

// utf8BOM starts CSV files so spreadsheets detect UTF-8.
const utf8BOM = "\xef\xbb\xbf"

// ResultRow is a line of the full standings.
type ResultRow struct {
	Rank    int      `json:"rank"` // 0 while not decided
	Tied    bool     `json:"tied"`
	Entrant SideView `json:"entrant"`
	// Where the rank was earned: the stage, and the rank there (pool
	// position or bracket place), 0 while the stage is not filled.
	Stage      string               `json:"stage"`
	StageName  string               `json:"stageName"`
	StageKind  tournament.StageKind `json:"stageKind"`
	EntryStage bool                 `json:"entryStage"`
	StageRank  int                  `json:"stageRank"`
}

// ResultCategory is the full standings of a category.
type ResultCategory struct {
	CategoryRef
	Started  bool        `json:"started"`
	Complete bool        `json:"complete"`
	Rows     []ResultRow `json:"rows"`
}

// ResultsView is the results screen.
type ResultsView struct {
	Name       string           `json:"name"`
	Archived   bool             `json:"archived"`
	Categories []ResultCategory `json:"categories"`
}

// GetResultsView returns the full standings of every category.
func (s *TournamentService) GetResultsView() ResultsView {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	side := sideViewer(st)
	v := ResultsView{Name: st.Name, Archived: st.Archived, Categories: []ResultCategory{}}
	for _, ref := range categoryRefs(st) {
		rows, complete := st.FinalStandings(ref.Category)
		rc := ResultCategory{CategoryRef: ref, Started: rows != nil, Complete: complete, Rows: []ResultRow{}}
		for _, r := range rows {
			stage, _ := st.Stage(ref.Category, r.Stage)
			rc.Rows = append(rc.Rows, ResultRow{
				Rank: r.Rank, Tied: r.Tied, Entrant: side(r.Entrant),
				Stage: r.Stage, StageName: stage.Name, StageKind: stage.Kind,
				EntryStage: st.Stages(ref.Category)[0].ID == r.Stage, StageRank: r.StageRank,
			})
		}
		v.Categories = append(v.Categories, rc)
	}
	return v
}

// ExportCSV asks where to save a CSV file and writes content to it, with a
// UTF-8 byte order mark so spreadsheets read accents right. The frontend
// builds the content: it holds the translated column names. It returns the
// path, or "" when the user cancelled.
func (s *TournamentService) ExportCSV(suggestedName, content string) (string, error) {
	path, err := s.dialogs.SaveCSV(s.tournamentsDir(context.Background()), suggestedName)
	if err != nil || path == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(path), ".csv") {
		path += ".csv"
	}
	if err := os.WriteFile(path, append([]byte(utf8BOM), content...), 0o644); err != nil {
		slog.Error("export csv", "err", err)
		return "", engine.NewError(engine.CodeWriteFailed)
	}
	return path, nil
}

// ArchiveTournament closes the tournament: it records the archive, writes a
// copy where the user chooses, then closes the file. It returns the copy's
// path, or "" when the user cancelled.
func (s *TournamentService) ArchiveTournament(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.file == nil {
		s.mu.Unlock()
		return "", engine.NewError(engine.CodeNoTournament)
	}
	suggested := fileNameFor(s.state.Name + " " + s.now().Format("2006-01-02"))
	source := s.file.Path()
	s.mu.Unlock()

	dest, err := s.dialogs.SaveTournament(s.tournamentsDir(ctx), suggested)
	if err != nil || dest == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(dest), store.TournamentExtension) {
		dest += store.TournamentExtension
	}
	if filepath.Clean(dest) == filepath.Clean(source) {
		// The copy cannot replace the open file.
		return "", engine.NewError(engine.CodeFileExists)
	}
	if err := s.execute(ctx, func(st tournament.State) (tournament.Change, error) { return st.Archive() }); err != nil {
		return "", err
	}

	s.mu.Lock()
	err = s.file.Backup(ctx, dest)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	if list, err := readRecent(ctx, s.app); err == nil {
		_ = writeRecent(ctx, s.app, markArchived(list, source))
	}
	s.CloseTournament()
	return dest, nil
}
