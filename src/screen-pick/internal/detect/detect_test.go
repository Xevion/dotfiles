package detect

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"screenpick/internal/fakefs"
	"screenpick/internal/resolve"
)

const (
	winePfx   = "/home/u/.wine"
	protonPfx = "/home/u/.local/share/Steam/steamapps/compatdata/100/pfx"
)

var disk = fakefs.New(
	"/tmp/build-cache/5b090f6779e3.log",
	"/tmp/out.bin",
	"/tmp/foo(1).txt",
	"/tmp/naïve.txt",
	"/tmp/日本.txt",
	"/etc/hosts",
	"/var/log/syslog",
	"/usr/lib/python3.12/os.py",
	"/home/u/.bashrc",
	"/home/u/.config/app/app.conf",
	"/home/u/x.html",
	"/home/u/My Documents/notes.txt",
	"/home/u/proj/src/main.rs",
	"/home/u/proj/src/lib.rs",
	"/home/u/proj/foo/bar.py",
	"/home/u/proj/Cargo.toml",
	"/home/u/proj/.gitignore",
	"/home/u/proj/.env",
	"/home/u/proj/Makefile",
	"/home/u/proj/Dockerfile",
	"/home/u/proj/LICENSE",
	"/home/u/proj/node_modules/.bin/tsc",
	"/home/u/proj/config.local.json",
	"/home/u/proj/package.json",
	"/home/u/proj/a.tar.gz",
	"/home/u/proj/b.d.ts",
	"/home/u/proj/file with spaces.txt",
	"/home/u/proj/config/app/app.conf",
	"/home/u/proj/README.md",
	"/home/u/proj/ä.txt",
	"/home/u/proj/été.txt",
	"/home/u/proj/a\nb",
	"/home/u/proj/.git/",
	"/home/u/proj/sub/deep/",
	"/home/u/other/README.md",
	"/home/u/old/stale.cpp",
	"/home/u/old/Cargo.toml",
	"/media/d/Data/file.bin",

	winePfx+"/dosdevices/c: -> ../drive_c",
	winePfx+"/dosdevices/z: -> /",
	winePfx+"/dosdevices/d: -> /media/d",
	winePfx+"/drive_c/users/u/Desktop/notes.txt",
	winePfx+"/drive_c/Program Files/App/app.log",
	winePfx+"/drive_c/windows/win.ini",

	protonPfx+"/dosdevices/c: -> ../drive_c",
	protonPfx+"/dosdevices/z: -> /",
	protonPfx+"/drive_c/users/steamuser/AppData/Local/Vendor/run.log",
	protonPfx+"/drive_c/windows/win.ini",
)

func testEnv() resolve.Env {
	vars := map[string]string{"HOME": "/home/u"}
	return resolve.Env{
		Bases:        []resolve.Base{{Dir: "/home/u/proj", Source: resolve.SourceProcess}},
		Home:         "/home/u",
		Getenv:       func(k string) string { return vars[k] },
		Lstat:        disk.Lstat,
		Readlink:     disk.Readlink,
		ReadDir:      disk.ReadDir,
		WinePrefixes: []string{winePfx, protonPfx},
	}
}

// render prints a match as kind:text, with @line[:col] when a location was given.
func render(m Match) string {
	s := m.Kind.String() + ":" + m.Text
	if m.Line > 0 {
		s += "@" + strconv.Itoa(m.Line)
		if m.Col > 0 {
			s += ":" + strconv.Itoa(m.Col)
		}
	}
	return s
}

type scanCase struct {
	name string
	in   string
	want []string
}

var scanCases = []scanCase{
	// The reported failures: a statusline glyph glued to the path made it
	// "not found", which also made two copy actions identical.
	{"glyph after path", "Wrote /tmp/build-cache/5b090f6779e3.log∴ done", []string{"path:/tmp/build-cache/5b090f6779e3.log"}},
	{"ellipsis after path", "/tmp/build-cache/5b090f6779e3.log… more", []string{"path:/tmp/build-cache/5b090f6779e3.log"}},
	{"hex filename is not a sha", "/tmp/build-cache/5b090f6779e3.log", []string{"path:/tmp/build-cache/5b090f6779e3.log"}},

	// Wrappers and trailing punctuation.
	{"parens with location", "see (src/main.rs:12:4) for details", []string{"path:src/main.rs@12:4"}},
	{"tool call", "● Read(src/main.rs)", []string{"path:src/main.rs"}},
	{"square brackets", "[src/main.rs]", []string{"path:src/main.rs"}},
	{"braces", "{/etc/hosts}", []string{"path:/etc/hosts"}},
	{"angle brackets", "</etc/hosts>", []string{"path:/etc/hosts"}},
	{"single quotes", "'/etc/hosts'", []string{"path:/etc/hosts"}},
	{"double quotes", `"/etc/hosts"`, []string{"path:/etc/hosts"}},
	{"backticks", "`/etc/hosts`", []string{"path:/etc/hosts"}},
	{"smart double quotes", "“/etc/hosts”", []string{"path:/etc/hosts"}},
	{"smart single quotes", "‘/etc/hosts’", []string{"path:/etc/hosts"}},
	{"trailing period", "See /etc/hosts.", []string{"path:/etc/hosts"}},
	{"trailing comma", "/etc/hosts, then", []string{"path:/etc/hosts"}},
	{"trailing semicolon", "/etc/hosts;", []string{"path:/etc/hosts"}},
	{"trailing bang and question", "/etc/hosts!?", []string{"path:/etc/hosts"}},
	{"trailing three dots", "/etc/hosts...", []string{"path:/etc/hosts"}},
	{"parens inside an existing name", "wrote /tmp/foo(1).txt", []string{"path:/tmp/foo(1).txt"}},
	{"markdown link", "[notes](src/main.rs)", []string{"path:src/main.rs"}},
	{"markdown bold", "**src/main.rs**", []string{"path:src/main.rs"}},
	{"box border no space", "│/etc/hosts│", []string{"path:/etc/hosts"}},
	{"box border spaced", "│ /etc/hosts │", []string{"path:/etc/hosts"}},
	{"tree branch", "├── src/main.rs", []string{"path:src/main.rs"}},
	{"rounded corner", "╰─/etc/hosts", []string{"path:/etc/hosts"}},
	{"claude result line", "  ⎿  Wrote 12 lines to /tmp/out.bin", []string{"path:/tmp/out.bin"}},

	// Spaces.
	{"unquoted space, exists", "/home/u/My Documents/notes.txt", []string{"path:/home/u/My Documents/notes.txt"}},
	{"single-quoted space, missing", "'/home/u/No Such/x.txt'", []string{"path:/home/u/No Such/x.txt"}},
	{"double-quoted space", `"/home/u/My Documents/notes.txt"`, []string{"path:/home/u/My Documents/notes.txt"}},
	{"backslash-escaped space", `/home/u/My\ Documents/notes.txt`, []string{`path:/home/u/My\ Documents/notes.txt`}},
	{"relative name with spaces", "open file with spaces.txt now", []string{"path:file with spaces.txt"}},
	{"tilde with space", "~/My Documents/notes.txt", []string{"path:~/My Documents/notes.txt"}},
	{"adjacent paths stay separate", "/etc/hosts /var/log/syslog", []string{"path:/etc/hosts", "path:/var/log/syslog"}},

	// Prefixes glued to a path.
	{"long flag", "--output=/tmp/out.bin", []string{"path:/tmp/out.bin"}},
	{"key=value", "path=/var/log/syslog;", []string{"path:/var/log/syslog"}},
	{"env assignment with tilde", "KEY=~/.bashrc", []string{"path:~/.bashrc"}},
	{"diff header", "diff --git a/src/main.rs b/src/main.rs", []string{"path:src/main.rs", "path:src/main.rs"}},
	{"diff old side", "--- a/src/main.rs", []string{"path:src/main.rs"}},
	{"diff new side", "+++ b/src/main.rs", []string{"path:src/main.rs"}},
	{"a/ kept when stripping finds nothing", "a/b.c", []string{"path:a/b.c"}},

	// Locations in compiler, linter and runtime output.
	{"rustc arrow", "  --> src/main.rs:4:9", []string{"path:src/main.rs@4:9"}},
	{"gcc", "main.c:3:5: error: expected ';'", []string{"path:main.c@3:5"}},
	{"go vet", "./main.go:12:3: undefined: x", []string{"path:./main.go@12:3"}},
	{"tsc parens", "src/app.ts(12,5): error TS2322", []string{"path:src/app.ts@12:5"}},
	{"msvc parens", "foo.c(12): error C2065", []string{"path:foo.c@12"}},
	{"python traceback", `File "/usr/lib/python3.12/os.py", line 10, in <module>`, []string{"path:/usr/lib/python3.12/os.py@10"}},
	{"pytest node id", "tests/test_x.py::test_foo PASSED", []string{"path:tests/test_x.py"}},
	{"pytest failure", "tests/test_x.py:42: AssertionError", []string{"path:tests/test_x.py@42"}},
	{"node stack frame", "at Object.<anonymous> (/app/index.js:5:3)", []string{"path:/app/index.js@5:3"}},
	{"trailing colon only", "Cargo.toml:", []string{"path:Cargo.toml"}},
	{"line only", "foo.rs:12", []string{"path:foo.rs@12"}},
	{"line col then message", "src/lib.rs:10:5: warning: unused", []string{"path:src/lib.rs@10:5"}},
	{"ripgrep line then code", "src/main.rs:12:fn main() {", []string{"path:src/main.rs@12"}},

	// Bare names: accepted on a known extension, otherwise only when on disk.
	{"dotted names", "files: config.local.json and package.json", []string{"path:config.local.json", "path:package.json"}},
	{"multi-dot extensions", "a.tar.gz b.d.ts", []string{"path:a.tar.gz", "path:b.d.ts"}},
	{"existing dotfiles", ".gitignore and .env", []string{"path:.gitignore", "path:.env"}},
	{"missing dotfile", "add a .foorc", nil},
	{"existing extensionless", "Makefile Dockerfile LICENSE", []string{"path:Makefile", "path:Dockerfile", "path:LICENSE"}},
	{"missing extensionless", "Rakefile", nil},
	{"bare directory word", "the src folder", nil},
	{"directory with slash", "look in src/", []string{"path:src/"}},
	{"dot directory segment", "node_modules/.bin/tsc", []string{"path:node_modules/.bin/tsc"}},
	{"missing known extension", "README.md", []string{"path:README.md"}},

	// Things that are not targets.
	{"bare domain", "visit example.com today", []string{"host:example.com"}},
	{"latin abbreviations", "e.g. this, i.e. that, etc.", nil},
	{"dotted identifier", "a.b.c.d", nil},
	{"rust path", "std::io::Result", nil},
	{"email", "user@host.com", nil},
	{"versions", "version 1.2.3 and v0.48.2 and 3.14", nil},
	{"statusline numbers", "cost $0.64 at 152.3 tok/s", nil},
	{"slash words", "1/2 tests, 50/50, and/or, I/O, TCP/IP, w/o, n/a, km/h", nil},
	{"dates", "2026/10/27 and 10/27/2026", nil},
	{"globs", "**/*.ts and src/*.rs", nil},
	{"method calls", "os.path.join(cwd, p) console.log(x) fmt.Println(x)", nil},
	{"attribute chain", "self.config.get", nil},
	{"go type names", "returns *exec.Cmd and os.File", nil},
	{"upper-case extension", "see NOTES.TXT", []string{"path:NOTES.TXT"}},
	{"operators", "-> => <-> ... // ./ ../ / ~ +/- ---", nil},
	{"clock time", "12:34:56", nil},
	{"mac address", "aa:bb:cc:dd:ee:ff", nil},
	{"hex literal", "0x1f2e3d4c", nil},
	{"uuid", "550e8400-e29b-41d4-a716-446655440000", nil},
	{"scheme alone", "http://", nil},
	{"go package pattern", "go test ./...", nil},

	// URLs.
	{"url with fragment", "https://github.com/foo/bar/blob/master/src/lib.rs#L10", []string{"url:https://github.com/foo/bar/blob/master/src/lib.rs#L10"}},
	{"url in parens", "(https://example.com/a)", []string{"url:https://example.com/a"}},
	{"url with balanced parens", "https://en.wikipedia.org/wiki/Foo_(bar)", []string{"url:https://en.wikipedia.org/wiki/Foo_(bar)"}},
	{"markdown url", "[docs](https://docs.rs/serde)", []string{"url:https://docs.rs/serde"}},
	{"angle url", "<https://example.com>", []string{"url:https://example.com"}},
	{"url trailing period", "see https://example.com/a.", []string{"url:https://example.com/a"}},
	{"url query and fragment", "https://example.com/p?q=1&r=2#frag.", []string{"url:https://example.com/p?q=1&r=2#frag"}},
	{"scp-style remote", "git@github.com:someone/project.git", []string{"url:git@github.com:someone/project.git"}},
	{"ssh url", "ssh://git@host:22/repo.git", []string{"url:ssh://git@host:22/repo.git"}},
	{"localhost url", "http://localhost:5173/app", []string{"url:http://localhost:5173/app"}},
	{"url in smart quotes", "“https://example.com”", []string{"url:https://example.com"}},
	{"url then glyph", "https://example.com∴", []string{"url:https://example.com"}},
	{"commit url is not a sha", "https://github.com/x/y/commit/3f2a9c1", []string{"url:https://github.com/x/y/commit/3f2a9c1"}},
	{"file url", "file:///home/u/x.html", []string{"path:file:///home/u/x.html"}},

	// Hosts.
	{"ipv4 with and without port", "connect to 192.168.1.10:8080 and 10.0.0.1", []string{"host:192.168.1.10:8080", "host:10.0.0.1"}},
	{"bracketed ipv6", "[::1]:3000", []string{"host:[::1]:3000"}},
	{"bare ipv6", "fe80::1", []string{"host:fe80::1"}},
	{"localhost port", "localhost:5173", []string{"host:localhost:5173"}},
	{"any address", "0.0.0.0:80", []string{"host:0.0.0.0:80"}},
	{"domain with port", "example.com:443", []string{"host:example.com:443"}},
	{"cidr", "10.0.0.0/8", []string{"host:10.0.0.0/8"}},
	{"octet out of range", "999.1.1.1", nil},
	{"five octets", "1.2.3.4.5", nil},
	{"hex words with double colon", "abc::def", nil},
	{"double colon alone", "::", nil},

	// Commit hashes.
	{"short sha", "commit 3f2a9c1", []string{"sha:3f2a9c1"}},
	{"full sha", "a7a5ba1c3d4e5f60718293a4b5c6d7e8f9012345", []string{"sha:a7a5ba1c3d4e5f60718293a4b5c6d7e8f9012345"}},
	{"oneline log", "184c3e9 feat(reconcile): mask", []string{"sha:184c3e9"}},
	{"sha range", "abc1234..def5678", []string{"sha:abc1234", "sha:def5678"}},
	{"all letters", "deadbeef", nil},
	{"all digits", "1234567", nil},
	{"too short", "abc123", nil},
	{"too long", "a7a5ba1c3d4e5f60718293a4b5c6d7e8f90123456", nil},
	{"md5 is not a sha", "id 1b7a526227c06c3f60ab04a7659d5789", nil},
	{"between abbreviated and full", "3f2a9c1d4e5f60718293a4", nil},
	{"sha256 digest", "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", nil},

	// Non-ASCII letters belong to the name; symbols do not.
	{"accented name", "/tmp/naïve.txt", []string{"path:/tmp/naïve.txt"}},
	{"cjk name", "/tmp/日本.txt", []string{"path:/tmp/日本.txt"}},

	// Variables and other roots.
	{"dollar var", "$HOME/.bashrc", []string{"path:$HOME/.bashrc"}},
	{"braced var", "${XDG_CONFIG_HOME}/fish", []string{"path:${XDG_CONFIG_HOME}/fish"}},
	{"tilde", "~/.config/app/app.conf", []string{"path:~/.config/app/app.conf"}},
	{"windows path", `C:\Users\someone\file.txt`, []string{`path:C:\Users\someone\file.txt`}},
	{"claude statusline", "~/proj │ master │ $0.64 │ 152.3 tok/s", []string{"path:~/proj"}},

	// Escaped and encoded forms. The span is what is on screen; Decode and
	// the resolver work out which reading is real.
	{"python PosixPath repr", "PosixPath('/tmp/out.bin')", []string{"path:/tmp/out.bin"}},
	{"python bytes repr", "b'/tmp/out.bin'", []string{"path:/tmp/out.bin"}},
	{"ls tty quoting", "'file with spaces.txt'  Cargo.toml", []string{"path:file with spaces.txt", "path:Cargo.toml"}},
	{"ls ansi-c quoting", `'a'$'\n''b'`, []string{`path:'a'$'\n''b'`}},
	{"git octal quoting", "\tmodified:   \"\\303\\244.txt\"", []string{`path:\303\244.txt`}},
	{"json doubled backslashes", `{"file":"C:\\Users\\x\\a.txt"}`, []string{`path:C:\\Users\\x\\a.txt`}},
	{"json escaped slashes", `{"p":"\/tmp\/out.bin"}`, []string{`path:\/tmp\/out.bin`}},
	{"json unicode escape", `"\u00e9t\u00e9.txt"`, []string{`path:\u00e9t\u00e9.txt`}},
	{"escaped parens", `\(draft\).txt`, []string{`path:\(draft\).txt`}},
	{"quoted var then path", `"$HOME"/.bashrc`, []string{`path:"$HOME"/.bashrc`}},

	// Windows and Wine forms.
	{"forward-slash drive path", "C:/Users/x/a.txt", []string{"path:C:/Users/x/a.txt"}},
	{"unc path", `\\srv\share\x.txt`, []string{`path:\\srv\share\x.txt`}},
	{"extended-length path", `\\?\C:\x.txt`, []string{`path:\\?\C:\x.txt`}},
	{"wsl unc path", `\\wsl$\Ubuntu\home\u\x.html`, []string{`path:\\wsl$\Ubuntu\home\u\x.html`}},
	{"msys path", "/c/Users/x/a.txt", []string{"path:/c/Users/x/a.txt"}},
	{"wine z drive", `Z:\home\u\x.html`, []string{`path:Z:\home\u\x.html`}},
	{"windows env var", `%APPDATA%\App\cfg.ini`, []string{`path:%APPDATA%\App\cfg.ini`}},
	{"unquoted space in wine path", `loaded C:\Program Files\App\app.log ok`, []string{`path:C:\Program Files\App\app.log`}},
	{"drive letter alone", "C:", nil},
	{"trailing backslash is not an escape", `Z:\srv\inst\ -Port 1`, []string{`path:Z:\srv\inst\`}},

	// Terminal plumbing: wrap markers and escapes never split or join a target.
	{"cr wrap marker", "/tmp/build-ca\r\nche/5b090f6779e3.log", []string{"path:/tmp/build-cache/5b090f6779e3.log"}},
	{"nul wrap marker", "/tmp/build-ca\x00\nche/5b090f6779e3.log", []string{"path:/tmp/build-cache/5b090f6779e3.log"}},
	{"real newline separates", "/tmp/out\n.bin", []string{"path:/tmp/out"}},
	{"sgr around path", "\x1b[31m/tmp/out.bin\x1b[0m", []string{"path:/tmp/out.bin"}},
	{"sgr inside path", "\x1b[1;34m/tmp/\x1b[0mout.bin", []string{"path:/tmp/out.bin"}},
	{"osc 8 hyperlink", "\x1b]8;;file:///home/u/x.html\x1b\\click\x1b]8;;\x1b\\", []string{"path:click"}},
	{"url across wrap", "https://example.com/a/very/lo\r\nng/path", []string{"url:https://example.com/a/very/long/path"}},

	// Rows from real tools.
	{"ls -l row", "-rw-r--r-- 1 u u 4.2K Oct 27 12:00 Cargo.toml", []string{"path:Cargo.toml"}},
	{"ls -l symlink", "lrwxrwxrwx 1 u u 10 Oct 27 12:00 hostlink -> /etc/hosts", []string{"path:/etc/hosts"}},
	{"git status long", "\tmodified:   src/main.rs", []string{"path:src/main.rs"}},
	{"git status porcelain", "M  config/app/app.conf", []string{"path:config/app/app.conf"}},
	{"git status untracked", "?? .env", []string{"path:.env"}},
	{"git rename", "renamed: old/a.txt -> new/b.txt", []string{"path:old/a.txt", "path:new/b.txt"}},
	{"diffstat", " src/main.rs | 12 +++---", []string{"path:src/main.rs"}},
	{"leading unicode ellipsis", "…/src/main.rs", []string{"path:src/main.rs"}},
	{"leading ascii ellipsis", ".../src/main.rs", []string{"path:src/main.rs"}},
	{"cargo compiling", "   Compiling demo v0.1.0 (/home/u/proj)", []string{"path:/home/u/proj"}},
	{"claude transcript", strings.Join([]string{
		"● Update(pkg/util/paths.py)",
		"  ⎿  Updated pkg/util/paths.py with 3 additions",
		"● Bash(go test ./...)",
		"  ⎿  ok  \tdemo/detect\t0.012s",
	}, "\n"), []string{
		"path:pkg/util/paths.py",
		"path:pkg/util/paths.py",
	}},
}

func TestScan(t *testing.T) {
	for _, tc := range scanCases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, m := range Scan(tc.in, resolve.New(testEnv())) {
				got = append(got, render(m))
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Scan(%q) (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}

// Offsets must point at exactly the visible text, in order and without
// overlap, or labels land in the wrong cells.
func TestScanOffsets(t *testing.T) {
	for _, tc := range scanCases {
		if strings.ContainsAny(tc.in, "\x1b\r\x00") {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			prev := 0
			for _, m := range Scan(tc.in, resolve.New(testEnv())) {
				if m.Start < prev || m.End <= m.Start || m.End > len(tc.in) {
					t.Fatalf("bad span [%d,%d) after %d for %q", m.Start, m.End, prev, m.Text)
				}
				if span := tc.in[m.Start:m.End]; span != m.Text {
					t.Errorf("span %q != text %q", span, m.Text)
				}
				prev = m.End
			}
		})
	}
}

func TestResolve(t *testing.T) {
	proc := func(dir string) resolve.Base { return resolve.Base{Dir: dir, Source: resolve.SourceProcess} }
	cases := []struct {
		name string
		in   string
		// pick selects the match with this text; empty means the last match.
		pick  string
		bases []resolve.Base // nil keeps testEnv's
		path  string
		// src is checked only when set.
		src    resolve.Source
		exists bool
		alts   int
	}{
		{name: "absolute", in: "/etc/hosts", path: "/etc/hosts", src: resolve.SourceAbsolute, exists: true},
		{name: "relative to process cwd", in: "src/main.rs", path: "/home/u/proj/src/main.rs", src: resolve.SourceProcess, exists: true},
		{name: "missing stays on first base", in: "nope.md", path: "/home/u/proj/nope.md", exists: false},
		{name: "tilde", in: "~/.bashrc", path: "/home/u/.bashrc", exists: true},
		{name: "env var", in: "$HOME/.bashrc", path: "/home/u/.bashrc", exists: true},
		{name: "quoted env var", in: `"$HOME"/.bashrc`, path: "/home/u/.bashrc", exists: true},
		{name: "file url", in: "file:///home/u/x.html", path: "/home/u/x.html", exists: true},
		{name: "percent-encoded file url", in: "file:///home/u/My%20Documents/notes.txt", path: "/home/u/My Documents/notes.txt", exists: true},
		{name: "osc 8 target wins over text", in: "\x1b]8;;file:///home/u/x.html\x1b\\click\x1b]8;;\x1b\\", path: "/home/u/x.html", exists: true},
		{name: "diff prefix stripped", in: "+++ b/src/main.rs", path: "/home/u/proj/src/main.rs", exists: true},
		{name: "truncated prefix", in: "…/src/main.rs", path: "/home/u/proj/src/main.rs", exists: true},

		// Escapes: the reading that exists wins.
		{name: "escaped space", in: `/home/u/My\ Documents/notes.txt`, path: "/home/u/My Documents/notes.txt", exists: true},
		{name: "ls ansi-c newline", in: `'a'$'\n''b'`, path: "/home/u/proj/a\nb", exists: true},
		{name: "git octal", in: "\"\\303\\244.txt\"", path: "/home/u/proj/ä.txt", exists: true},
		{name: "json unicode", in: `"\u00e9t\u00e9.txt"`, path: "/home/u/proj/été.txt", exists: true},
		{name: "json escaped slashes", in: `"\/tmp\/out.bin"`, path: "/tmp/out.bin", exists: true},

		// Several bases: the nearest directory path above the match, then
		// process, root, window. Only existing directories count, whatever
		// text surrounds them.
		{name: "cwd beats a later base", in: "main.rs", bases: []resolve.Base{proc("/home/u/proj/src"), proc("/home/u/proj")}, path: "/home/u/proj/src/main.rs", exists: true},
		{name: "falls back to a later base", in: "src/main.rs", bases: []resolve.Base{proc("/home/u/proj/src"), proc("/home/u/proj")}, path: "/home/u/proj/src/main.rs", exists: true},
		{name: "project root found by walking up", in: "src/main.rs", bases: []resolve.Base{proc("/home/u/proj/sub/deep")}, path: "/home/u/proj/src/main.rs", src: resolve.SourceRoot, exists: true},
		{name: "stale window cwd still searched", in: "stale.cpp", bases: []resolve.Base{proc("/home/u/proj"), {Dir: "/home/u/old", Source: resolve.SourceWindow}}, path: "/home/u/old/stale.cpp", src: resolve.SourceWindow, exists: true},
		{name: "same name under two bases", in: "Cargo.toml", bases: []resolve.Base{proc("/home/u/proj"), {Dir: "/home/u/old", Source: resolve.SourceWindow}}, path: "/home/u/proj/Cargo.toml", exists: true, alts: 1},
		{name: "directory path above", in: "x(y /home/u/other z)\n  README.md", path: "/home/u/other/README.md", src: resolve.SourceScreen, exists: true, alts: 1},
		{name: "tilde directory above", in: "~/other\nREADME.md", path: "/home/u/other/README.md", src: resolve.SourceScreen, exists: true, alts: 1},
		{name: "quoted directory above", in: "'/home/u/other'\nREADME.md", path: "/home/u/other/README.md", src: resolve.SourceScreen, exists: true, alts: 1},
		{name: "directory below does not count", in: "/home/u/proj\nREADME.md\n/home/u/other", pick: "README.md", path: "/home/u/proj/README.md", exists: true},
		{name: "nearest directory wins", in: "/home/u/proj\n/home/u/other\nREADME.md", path: "/home/u/other/README.md", src: resolve.SourceScreen, exists: true, alts: 1},
		{name: "directory without the file falls through", in: "/home/u/other\nsrc/main.rs", path: "/home/u/proj/src/main.rs", src: resolve.SourceProcess, exists: true},
		{name: "file path above is not a base", in: "/home/u/other/README.md\nREADME.md", path: "/home/u/proj/README.md", src: resolve.SourceProcess, exists: true},

		// Wine: drives come from each prefix's dosdevices links.
		{name: "wine z drive", in: `Z:\home\u\x.html`, path: "/home/u/x.html", exists: true},
		{name: "wine c drive", in: `C:\users\u\Desktop\notes.txt`, path: winePfx + "/drive_c/users/u/Desktop/notes.txt", src: resolve.SourceWine, exists: true},
		{name: "wine is case-insensitive", in: `C:\USERS\U\desktop\NOTES.TXT`, path: winePfx + "/drive_c/users/u/Desktop/notes.txt", exists: true},
		{name: "wine forward slashes", in: "C:/users/u/Desktop/notes.txt", path: winePfx + "/drive_c/users/u/Desktop/notes.txt", exists: true},
		{name: "wine json-doubled", in: `"C:\\users\\u\\Desktop\\notes.txt"`, path: winePfx + "/drive_c/users/u/Desktop/notes.txt", exists: true},
		{name: "wine mapped drive", in: `D:\Data\file.bin`, path: "/media/d/Data/file.bin", exists: true},
		{name: "wine space in path", in: `C:\Program Files\App\app.log`, path: winePfx + "/drive_c/Program Files/App/app.log", exists: true},
		{name: "proton prefix", in: `C:\users\steamuser\AppData\Local\Vendor\run.log`, path: protonPfx + "/drive_c/users/steamuser/AppData/Local/Vendor/run.log", exists: true},
		{name: "file in two prefixes", in: `C:\windows\win.ini`, path: winePfx + "/drive_c/windows/win.ini", exists: true, alts: 1},
		{name: "c-escapes not applied to drive paths", in: `C:\temp\new.txt`, path: winePfx + "/drive_c/temp/new.txt", exists: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := testEnv()
			if tc.bases != nil {
				env.Bases = tc.bases
			}
			ms := Scan(tc.in, resolve.New(env))
			if len(ms) == 0 {
				t.Fatalf("Scan(%q) found nothing", tc.in)
			}
			m := ms[len(ms)-1]
			if tc.pick != "" {
				i := slices.IndexFunc(ms, func(m Match) bool { return m.Text == tc.pick })
				if i < 0 {
					t.Fatalf("no match with text %q", tc.pick)
				}
				m = ms[i]
			}
			if m.Path != tc.path || m.Exists != tc.exists {
				t.Errorf("got (%q, exists=%v), want (%q, exists=%v)", m.Path, m.Exists, tc.path, tc.exists)
			}
			if tc.src != 0 && m.Source != tc.src {
				t.Errorf("source %v, want %v", m.Source, tc.src)
			}
			if len(m.Alternatives) != tc.alts {
				t.Errorf("%d alternatives %v, want %d", len(m.Alternatives), m.Alternatives, tc.alts)
			}
		})
	}
}

func TestScanWidth(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  []string
	}{
		{"full row is a real wrap", "/tmp/bui\r\nld-cache/5b090f6779e3.log", 8, []string{"path:/tmp/build-cache/5b090f6779e3.log"}},
		{"short row is a hard break", "see /tmp/out\r\n.bin here", 40, []string{"path:/tmp/out"}},
		{"wide characters count as two cells", "日本/x\r\n.txt", 6, []string{"path:日本/x.txt"}},
		{"no width trusts every marker", "see /tmp/out\r\n.bin here", 0, []string{"path:/tmp/out.bin"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, m := range ScanWidth(tc.in, resolve.New(testEnv()), tc.width) {
				got = append(got, render(m))
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
