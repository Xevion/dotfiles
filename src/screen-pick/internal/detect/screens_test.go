package detect

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"screenpick/internal/fakefs"
	"screenpick/internal/resolve"
)

var update = flag.Bool("update", false, "rewrite testdata/screens/*.golden")

var screenWidths = []int{0, 40, 120}

var screensDisk = fakefs.New(
	"/tmp/build-cache/5b090f6779e3.log",
	"/tmp/out.bin",
	"/tmp/naïve.txt",
	"/tmp/日本.txt",
	"/etc/hosts",
	"/var/log/syslog",
	"/usr/lib/python3.12/os.py",
	"/usr/include/stdio.h",
	"/home/u/.bashrc",
	"/home/u/.npm/_logs/2026-10-27T12_01_44_221Z-debug-0.log",
	"/home/u/.ssh/id_ed25519",
	"/home/u/.config/app/app.conf",
	"/home/u/x.html",
	"/home/u/My Documents/notes.txt",
	"/home/u/proj/src/main.rs",
	"/home/u/proj/src/lib.rs",
	"/home/u/proj/src/main.c",
	"/home/u/proj/src/util.c",
	"/home/u/proj/src/server/mod.rs",
	"/home/u/proj/src/server/handler.rs",
	"/home/u/proj/src/main/kotlin/App.kt",
	"/home/u/proj/app/main.py",
	"/home/u/proj/app/models.py",
	"/home/u/proj/app/config.py",
	"/home/u/proj/tests/test_api.py",
	"/home/u/proj/tests/test_models.py",
	"/home/u/proj/tests/test_util.py",
	"/home/u/proj/web/src/App.tsx",
	"/home/u/proj/web/src/index.ts",
	"/home/u/proj/web/src/index.js",
	"/home/u/proj/web/package.json",
	"/home/u/proj/cmd/app/main.go",
	"/home/u/proj/internal/db/db.go",
	"/home/u/proj/internal/db/db_test.go",
	"/home/u/proj/internal/util/util.go",
	"/home/u/proj/docs/guide.md",
	"/home/u/proj/docs/notes/2026-10-27.md",
	"/home/u/proj/Cargo.toml",
	"/home/u/proj/Cargo.lock",
	"/home/u/proj/.gitignore",
	"/home/u/proj/.env",
	"/home/u/proj/Makefile",
	"/home/u/proj/Dockerfile",
	"/home/u/proj/LICENSE",
	"/home/u/proj/run.sh",
	"/home/u/proj/run.ps1",
	"/home/u/proj/package.json",
	"/home/u/proj/README.md",
	"/home/u/proj/a.tar.gz",
	"/home/u/proj/b.d.ts",
	"/home/u/proj/file with spaces.txt",
	"/home/u/proj/été.txt",
	"/home/u/proj/ä.txt",
	"/home/u/proj/app.properties",
	"/home/u/proj/config/app/app.conf",
	"/home/u/proj/current -> src/main.rs",
	"/home/u/proj/node_modules/.bin/tsc",
	"/home/u/proj/.git/",
	"/home/u/proj/sub/deep/",
	"/home/u/other/README.md",
	"/media/d/Data/file.bin",

	winePfx+"/dosdevices/c: -> ../drive_c",
	winePfx+"/dosdevices/z: -> /",
	winePfx+"/dosdevices/d: -> /media/d",
	winePfx+"/drive_c/users/u/Desktop/notes.txt",
	winePfx+"/drive_c/users/u/AppData/Local/Vendor/run.log",
	winePfx+"/drive_c/Program Files/App/app.log",
	winePfx+"/drive_c/windows/win.ini",
)

func screensEnv() resolve.Env {
	env := testEnv()
	env.Lstat, env.Readlink, env.ReadDir = screensDisk.Lstat, screensDisk.Readlink, screensDisk.ReadDir
	env.WinePrefixes = []string{winePfx}
	return env
}

func renderGolden(text string, width int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# width %d\n", width)
	for _, m := range ScanWidth(text, resolve.New(screensEnv()), width) {
		fmt.Fprintf(&b, "%s %q", m.Kind, m.Text)
		if m.Line > 0 {
			fmt.Fprintf(&b, " @%d:%d", m.Line, m.Col)
		}
		fmt.Fprintf(&b, " [%d-%d]", m.Start, m.End)
		if m.Kind == KindPath {
			state := "missing"
			switch {
			case m.Exists && m.IsDir:
				state = "dir"
			case m.Exists:
				state = "file"
			}
			fmt.Fprintf(&b, " -> %q %s %s +%d", m.Path, m.Source, state, len(m.Alternatives))
			if m.Expanded {
				b.WriteString(" expanded")
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func screenFiles(t testing.TB) []string {
	t.Helper()
	files, err := filepath.Glob("testdata/screens/*.txt")
	if err != nil || len(files) == 0 {
		t.Fatalf("no screens: %v", err)
	}
	return files
}

func TestScreens(t *testing.T) {
	for _, file := range screenFiles(t) {
		name := strings.TrimSuffix(filepath.Base(file), ".txt")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			for _, w := range screenWidths {
				b.WriteString(renderGolden(string(raw), w))
			}
			golden := strings.TrimSuffix(file, ".txt") + ".golden"
			if *update {
				if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			if diff := cmp.Diff(string(want), b.String()); diff != "" {
				t.Errorf("golden mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
