package resolve

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"screenpick/internal/fakefs"
)

const (
	winePfx   = "/home/u/.wine"
	protonPfx = "/home/u/.local/share/Steam/steamapps/compatdata/100/pfx"
)

func TestRankWinePrefixes(t *testing.T) {
	found := []string{winePfx, protonPfx}
	cases := []struct {
		name  string
		procs []map[string]string
		want  []string
	}{
		{"none running", nil, []string{winePfx, protonPfx}},
		{"proton app running", []map[string]string{
			{"STEAM_COMPAT_DATA_PATH": "/home/u/.local/share/Steam/steamapps/compatdata/100"},
		}, []string{protonPfx, winePfx}},
		{"explicit prefix", []map[string]string{{"WINEPREFIX": "/opt/pfx"}}, []string{"/opt/pfx", winePfx, protonPfx}},
		{"WINEPREFIX beats compat path in one process", []map[string]string{{
			"STEAM_COMPAT_DATA_PATH": "/home/u/.local/share/Steam/steamapps/compatdata/100",
			"WINEPREFIX":             "/opt/pfx",
		}}, []string{"/opt/pfx", protonPfx, winePfx}},
		{"trailing slash deduped", []map[string]string{{"WINEPREFIX": winePfx + "/"}}, []string{winePfx, protonPfx}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, RankWinePrefixes(tc.procs, found)); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

var disk = fakefs.New(
	"/home/u/proj/.git/",
	"/home/u/proj/src/main.rs",
	"/home/u/proj/sub/deep/",
	"/home/u/proj/README.md",
	"/home/u/old/Cargo.toml",
	"/home/u/old/stale.cpp",
	"/home/u/crate/Cargo.toml",
	"/home/u/crate/src/",
	"/home/u/.local/share/notes/src/tool/go.mod",
	"/home/u/.local/share/notes/src/tool/main.go",
	"/home/u/.local/state/x/",
	"/home/u/.local/share/x/",
	"/home/u/.config/fish/config.fish",
	winePfx+"/dosdevices/c: -> ../drive_c",
	winePfx+"/drive_c/users/u/notes.txt",
)

func newResolver(bases ...Base) *Resolver {
	return New(Env{
		Bases:        bases,
		Home:         "/home/u",
		Getenv:       func(k string) string { return map[string]string{"HOME": "/home/u"}[k] },
		Lstat:        disk.Lstat,
		Readlink:     disk.Readlink,
		ReadDir:      disk.ReadDir,
		WinePrefixes: []string{winePfx},
	})
}

func TestBases(t *testing.T) {
	cases := []struct {
		name  string
		bases []Base
		want  []Base
	}{
		{"process then its root then home", []Base{{"/home/u/proj/sub/deep", SourceProcess}}, []Base{
			{"/home/u/proj/sub/deep", SourceProcess},
			{"/home/u/proj", SourceRoot},
			{"/home/u", SourceHome},
		}},
		{"window ranks after roots", []Base{{"/home/u/old", SourceWindow}, {"/home/u/proj/src", SourceProcess}}, []Base{
			{"/home/u/proj/src", SourceProcess},
			{"/home/u/proj", SourceRoot},
			{"/home/u/old", SourceWindow},
			{"/home/u", SourceHome},
		}},
		{"manifest root without git", []Base{{"/home/u/crate/src", SourceProcess}}, []Base{
			{"/home/u/crate/src", SourceProcess},
			{"/home/u/crate", SourceRoot},
			{"/home/u", SourceHome},
		}},
		{"duplicates collapse", []Base{{"/home/u/proj/", SourceProcess}, {"/home/u/proj", SourceWindow}}, []Base{
			{"/home/u/proj", SourceProcess},
			{"/home/u", SourceHome},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, newResolver(tc.bases...).Bases()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	proc := Base{"/home/u/proj", SourceProcess}
	cases := []struct {
		name   string
		token  string
		screen []string
		want   Resolution
	}{
		{"relative", "src/main.rs", nil, Resolution{Path: "/home/u/proj/src/main.rs", Source: SourceProcess, Exists: true}},
		{"missing keeps the first base", "nope.rs", nil, Resolution{Path: "/home/u/proj/nope.rs", Source: SourceProcess}},
		{"screen outranks process", "Cargo.toml", []string{"/home/u/old"}, Resolution{Path: "/home/u/old/Cargo.toml", Source: SourceScreen, Exists: true}},
		{"directory flagged", "src", nil, Resolution{Path: "/home/u/proj/src", Source: SourceProcess, Exists: true, IsDir: true}},
		{"unset variable cannot be placed", "$NOPE/x", nil, Resolution{}},
		{"unc share cannot be placed", `\\srv\share\x`, nil, Resolution{}},
		{"windows env var cannot be placed", `%APPDATA%\x`, nil, Resolution{}},
		{"wine missing file keeps prefix path", `C:\users\u\nope.txt`, nil, Resolution{Path: winePfx + "/drive_c/users/u/nope.txt", Source: SourceWine}},
		{"wine with no matching drive", `Q:\x.txt`, nil, Resolution{}},

		// Abbreviated components, as a shortened prompt or statusline prints.
		{"abbreviated home path", "~/.l/s/n/s/tool", nil, Resolution{
			Path: "/home/u/.local/share/notes/src/tool", Source: SourceHome, Exists: true, IsDir: true, Expanded: true,
		}},
		{"abbreviated with file", "~/.c/f/config.fish", nil, Resolution{
			Path: "/home/u/.config/fish/config.fish", Source: SourceHome, Exists: true, Expanded: true,
		}},
		{"ambiguous abbreviation keeps every branch", "~/.l/s/x", nil, Resolution{
			Path: "/home/u/.local/share/x", Source: SourceHome, Exists: true, IsDir: true, Expanded: true,
			Alternatives: []Candidate{{"/home/u/.local/state/x", SourceHome}},
		}},
		{"abbreviation narrows to the branch that exists", "~/.l/s/n/s/t/main.go", nil, Resolution{
			Path: "/home/u/.local/share/notes/src/tool/main.go", Source: SourceHome, Exists: true, Expanded: true,
		}},
		{"last component must match exactly", "~/.c/f/config", nil, Resolution{Path: "/home/u/.c/f/config", Source: SourceHome}},
		{"long components are not abbreviations", "/home/u/projx/src/main.rs", nil, Resolution{Path: "/home/u/projx/src/main.rs", Source: SourceAbsolute}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := newResolver(proc).Resolve(tc.token, tc.screen)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Resolve(%q) (-want +got):\n%s", tc.token, diff)
			}
		})
	}
}
