package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

func TestFirstExistingDir(t *testing.T) {
	type test struct {
		name     string
		dir      string
		existing []string
		want     string
	}
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = `C:\`
	}
	p := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	tests := []test{
		{"exists", p("clubs", "lists"), []string{p("clubs", "lists"), p("clubs")}, p("clubs", "lists")},
		{"parent", p("clubs", "lists", "2026"), []string{p("clubs")}, p("clubs")},
		{"root only", p("gone", "deeper"), []string{root}, root},
		{"none", p("gone"), nil, ""},
		{"not cleaned", p("clubs") + string(filepath.Separator) + "." + string(filepath.Separator), []string{p("clubs")}, p("clubs")},
		{"empty", "", []string{root}, ""},
		{"relative", filepath.Join("clubs", "lists"), []string{"clubs", "."}, ""},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests,
			test{"windows drive", `D:\Tournois\2026\Printemps`, []string{`D:\Tournois`}, `D:\Tournois`},
			test{"windows forward slashes", `D:/Tournois/2026`, []string{`D:\Tournois`}, `D:\Tournois`},
			test{"windows drive gone", `E:\Tournois`, []string{`C:\`}, ""},
			test{"windows share", `\\club\partage\inscrits\2026`, []string{`\\club\partage\inscrits`}, `\\club\partage\inscrits`},
			test{"windows share gone", `\\club\partage\inscrits`, nil, ""},
			test{"windows no drive", `Tournois\2026`, []string{`Tournois`}, ""},
		)
	} else {
		// A folder remembered on Windows is not a path here.
		tests = append(tests, test{"windows path elsewhere", `C:\Users\julie\Documents`, []string{root}, ""})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := map[string]bool{}
			for _, d := range tt.existing {
				exists[d] = true
			}
			if got := firstExistingDir(tt.dir, func(d string) bool { return exists[d] }); got != tt.want {
				t.Errorf("firstExistingDir(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

// TestDialogDirs checks, for each context, that its dialog opens in the
// default folder, then in the folder last used for it (not another
// context's), then in its first existing parent once it is gone.
func TestDialogDirs(t *testing.T) {
	f := newBareFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	name := func(ext string) string { n++; return fmt.Sprintf("f%d%s", n, ext) }

	// Each context: run its dialog, which returns a file in dir.
	contexts := []struct {
		c   dialogContext
		run func(dir string)
	}{
		{dialogImport, func(dir string) {
			path := filepath.Join(dir, name(".csv"))
			must(os.WriteFile(path, []byte("Joueur 1;Joueur 2\nZola;Mia\n"), 0o644))
			f.dialogs.openCSV = path
			_, err := f.svc.ChooseImportFile(f.ctx, engine.MensDoubles)
			must(err)
		}},
		{dialogExport, func(dir string) {
			f.dialogs.csv = filepath.Join(dir, name(".csv"))
			_, err := f.svc.ExportCSV("plume.csv", "a;b\n")
			must(err)
		}},
		{dialogOpen, func(dir string) {
			path := filepath.Join(dir, name(".openshuttle"))
			must(f.svc.CreateTournamentAt(f.ctx, path, "Plume"))
			f.svc.CloseTournament()
			f.dialogs.open = path
			_, err := f.svc.OpenTournamentDialog(f.ctx)
			must(err)
		}},
		{dialogArchive, func(dir string) {
			must(f.svc.CreateTournamentAt(f.ctx, filepath.Join(f.dir, name(".openshuttle")), "Plume"))
			f.dialogs.save = filepath.Join(dir, name(""))
			_, err := f.svc.ArchiveTournament(f.ctx)
			must(err)
		}},
		{dialogNew, func(dir string) {
			f.dialogs.save = filepath.Join(dir, name(""))
			_, err := f.svc.CreateTournament(f.ctx, "Plume")
			must(err)
		}},
	}
	def := f.svc.tournamentsDir(f.ctx)
	for _, tc := range contexts {
		t.Run(string(tc.c), func(t *testing.T) {
			wantDir := func(want string) {
				t.Helper()
				if f.dialogs.dir != want {
					t.Errorf("dialog opened in %q, want %q", f.dialogs.dir, want)
				}
			}
			base := filepath.Join(f.dir, string(tc.c))
			used := filepath.Join(base, "used")
			must(os.MkdirAll(used, 0o755))

			tc.run(used) // first time: the default folder
			wantDir(def)
			tc.run(used)
			wantDir(used)

			must(os.RemoveAll(used)) // gone: its parent
			must(os.MkdirAll(used, 0o755))
			tc.run(used)
			must(os.RemoveAll(used))
			tc.run(base)
			wantDir(base)

			// Nothing left (or a path from another system): the default.
			must(f.app.SetSetting(f.ctx, settingLastDir(tc.c), filepath.Join(f.dir, "gone", "deeper")))
			tc.run(base)
			wantDir(f.dir)
			must(f.app.SetSetting(f.ctx, settingLastDir(tc.c), "relative"))
			tc.run(base)
			wantDir(def)
		})
	}

	// Each context kept its own folder.
	for _, tc := range contexts {
		if got, _, _ := f.app.Setting(f.ctx, settingLastDir(tc.c)); got != filepath.Join(f.dir, string(tc.c)) {
			t.Errorf("%s remembers %q", tc.c, got)
		}
	}
}

// TestNewTournamentDialogDir: the tournaments folder of the settings wins
// over the folder last used for a new tournament.
func TestNewTournamentDialogDir(t *testing.T) {
	f := newBareFixture(t)
	elsewhere := filepath.Join(f.dir, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	f.dialogs.save = filepath.Join(elsewhere, "a")
	if _, err := f.svc.CreateTournament(f.ctx, "Plume"); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(f.dir, "Tournois")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	f.dialogs.folder = folder
	if _, err := f.settings.ChooseTournamentsDir(f.ctx); err != nil {
		t.Fatal(err)
	}
	f.dialogs.save = filepath.Join(elsewhere, "b")
	if _, err := f.svc.CreateTournament(f.ctx, "Plume"); err != nil {
		t.Fatal(err)
	}
	if f.dialogs.dir != folder {
		t.Errorf("dialog opened in %q, want the tournaments folder %q", f.dialogs.dir, folder)
	}
	// Once that folder is gone, the folder last used.
	if err := os.RemoveAll(folder); err != nil {
		t.Fatal(err)
	}
	f.dialogs.save = filepath.Join(elsewhere, "c")
	if _, err := f.svc.CreateTournament(f.ctx, "Plume"); err != nil {
		t.Fatal(err)
	}
	if f.dialogs.dir != elsewhere {
		t.Errorf("dialog opened in %q, want %q", f.dialogs.dir, elsewhere)
	}
}
