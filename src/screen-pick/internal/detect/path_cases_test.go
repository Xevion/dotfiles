package detect

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"screenpick/internal/fakefs"
	"screenpick/internal/resolve"
)

// pathDisk is this file's own tree so its cases never depend on, or disturb,
// the shared one.
var pathDisk = fakefs.New(
	"/etc/hosts",
	"/tmp/out.bin",
	"/tmp/a+b.txt",
	"/tmp/x@y.txt",
	"/tmp/100%.txt",
	"/tmp/foo(1).txt",
	"/tmp/dir/",
	"/usr/bin/env",
	"/bin/sh",
	"/lib/libc.so",
	"/usr/include/stdio.h",
	"/usr/lib/python3.12/os.py",
	"/var/log/syslog",
	"/home/u/.bashrc",
	"/home/other/notes.md",
	"/home/u/proj/Cargo.toml",
	"/home/u/proj/README.md",
	"/home/u/proj/src/main.rs",
	"/home/u/proj/src/lib.rs",
	"/home/u/proj/tests/test_x.py",
	"/home/u/proj/a#b.txt",
	"/home/u/proj/notes.txt~",
	"/home/u/proj/.hidden",
	"/home/u/proj/etc/hosts",
)

func pathEnv() resolve.Env {
	vars := map[string]string{"HOME": "/home/u"}
	return resolve.Env{
		Bases:    []resolve.Base{{Dir: "/home/u/proj", Source: resolve.SourceProcess}},
		Home:     "/home/u",
		Getenv:   func(k string) string { return vars[k] },
		Lstat:    pathDisk.Lstat,
		Readlink: pathDisk.Readlink,
		ReadDir:  pathDisk.ReadDir,
	}
}

func p(s ...string) []string {
	out := make([]string, len(s))
	for i, t := range s {
		out[i] = "path:" + t
	}
	return out
}

const reportedPath = "/home/xevion/Videos/Snapshot/2026-11/Snapshot_2026-11-12_22-00-46.mp4"

var pathCases = []scanCase{
	// Junk glued in front of a root: caret notation echoed after Ctrl+C, and
	// other marks that carry no meaning of their own.
	{"reported ctrl-c echo", "^C" + reportedPath, p(reportedPath)},
	{"ctrl-c echo on disk", "^C/etc/hosts", p("/etc/hosts")},
	{"double ctrl-c", "^C^C/etc/hosts", p("/etc/hosts")},
	{"ctrl-d", "^D/etc/hosts", p("/etc/hosts")},
	{"ctrl-c before tilde", "^C~/.bashrc", p("~/.bashrc")},
	{"ctrl-c before dot slash", "^C./src/main.rs", p("./src/main.rs")},
	{"ctrl-c before a drive path", `^CC:\Users\x\file.txt`, p(`C:\Users\x\file.txt`)},
	{"ctrl-c before unc", `^C\\srv\share\x.txt`, p(`\\srv\share\x.txt`)},
	{"ctrl-c before dollar var", "^C$HOME/.bashrc", p("$HOME/.bashrc")},
	{"ctrl-c mid line", "$ cat ^C/etc/hosts", p("/etc/hosts")},
	{"variation selector before path", "⚠\ufe0f/opt/missing/lib/a.so", p("/opt/missing/lib/a.so")},
	{"combining mark before path", "\u0301/opt/missing/lib/a.so", p("/opt/missing/lib/a.so")},
	{"icon glyphs before path", "\uf07b/opt/missing/lib/a.so \ue0b0/etc/hosts", p("/opt/missing/lib/a.so", "/etc/hosts")},

	// Redirections and operators fuse with the next word without a space.
	{"redirect", ">/tmp/out.bin", p("/tmp/out.bin")},
	{"fd redirect", "2>/tmp/out.bin", p("/tmp/out.bin")},
	{"append", ">>/tmp/out.bin", p("/tmp/out.bin")},
	{"both streams", "&>/tmp/out.bin", p("/tmp/out.bin")},
	{"input redirect", "cat</etc/hosts", p("/etc/hosts")},
	{"command substitution", "$(/etc/hosts)", p("/etc/hosts")},
	{"pipe", "ls|/etc/hosts", p("/etc/hosts")},
	{"at sign", "@/etc/hosts", p("/etc/hosts")},
	{"scheme-less prefix", "file:/etc/hosts", p("/etc/hosts")},
	{"tool prefix", "ls:/etc/hosts", p("/etc/hosts")},
	{"error prefix", "Error:/etc/hosts", p("/etc/hosts")},
	{"remote-looking prefix", "user@box:/etc/hosts", p("/etc/hosts")},
	{"prefix with nothing on disk", "file:/opt/missing/dir/a.txt", nil},
	{"compiler include flag", "-I/usr/include", p("/usr/include")},
	{"compiler output flag", "-o/tmp/out.bin", p("/tmp/out.bin")},
	{"flag path that is missing", "-L/opt/missing/lib", p("/opt/missing/lib")},
	{"flag before tilde", "-I~/proj", p("~/proj")},
	{"flag before a windows path", `-IC:\inc\a.h`, p(`C:\inc\a.h`)},
	{"long flag with equals", "--config=/home/u/.bashrc", p("/home/u/.bashrc")},

	// Colon-separated lists.
	{"colon list on disk", "PATH=/usr/bin:/usr/include:/home/u", p("/usr/bin", "/usr/include", "/home/u")},
	{"colon list missing", "LD_LIBRARY_PATH=/opt/a/lib:/opt/b/lib", p("/opt/a/lib", "/opt/b/lib")},
	{"colon list after plain names", "a:b:/etc/hosts", p("/etc/hosts")},
	{"colon then tilde", "/etc/hosts:~/.bashrc", p("/etc/hosts", "~/.bashrc")},

	// A root in the middle of a word counts when it is on disk.
	{"mid-word root on disk", "x/etc/hosts", p("/etc/hosts")},
	{"mid-word root deeper", "foo/usr/bin/env", p("/usr/bin/env")},
	{"relative path of known shape stays whole", "build/tmp/out.bin", p("build/tmp/out.bin")},
	{"junk-looking prefix beats shape", "x@/tmp/out.bin", p("/tmp/out.bin")},
	{"debian prompt", "xevion@box:~/proj$ ls", p("~/proj")},
	{"root prompt", "root@box:/tmp/dir# ls", p("/tmp/dir")},
	{"vim quickfix", "src/main.rs|12 col 4| error", p("src/main.rs@12:4")},
	{"fullwidth space between", "/etc/hosts\u3000/tmp/out.bin", p("/etc/hosts", "/tmp/out.bin")},
	{"cjk brackets", "「/etc/hosts」", p("/etc/hosts")},
	{"percent-encoded missing", "/opt/missing/a%20b.txt", p("/opt/missing/a%20b.txt")},
	{"quoted windows path with space", `"C:\Users\x\My File.txt"`, p(`C:\Users\x\My File.txt`)},
	{"ctrl-c after the path", "/etc/hosts^C", p("/etc/hosts")},
	{"ctrl-c after a missing path", "/opt/missing/lib/a.so^C", p("/opt/missing/lib/a.so")},
	{"brace rename in a diffstat", "src/{old => new}/main.rs", p("src/")},
	{"root after a closing bracket that is missing", "(x)/opt/missing/a.txt", nil},
	{"ctrl-c before escaped space", `^C/opt/missing/My\ Documents/x.txt`, p(`/opt/missing/My\ Documents/x.txt`)},
	{"bracket label then path", "[ok]/etc/hosts", p("/etc/hosts")},
	{"tail header", "==> src/main.rs <==", p("src/main.rs")},
	{"csharp line", "in /usr/lib/python3.12/os.py:line 12", p("/usr/lib/python3.12/os.py@12")},
	{"docker volume", "-v /etc/hosts:/etc/hosts:ro", p("/etc/hosts", "/etc/hosts")},
	{"mid-word root missing", "foo/opt/missing/lib/x.so", p("foo/opt/missing/lib/x.so")},
	{"mid-word root inside quotes", `"x/etc/hosts"`, p("/etc/hosts")},
	{"mid-word root inside a space-joined name", "see x/etc/hosts now", p("/etc/hosts")},
	{"mid-word tilde", "x~/.bashrc", p("~/.bashrc")},
	{"mid-word variable", "x$HOME/.bashrc", p("$HOME/.bashrc")},
	{"merged by styling", "\x1b[1mdone\x1b[0m/etc/hosts", p("/etc/hosts")},

	// Escape-sequence remnants and control characters.
	{"charset designator", "\x1b(B\x1b[m/etc/hosts", p("/etc/hosts")},
	{"bare escape before slash", "\x1b/etc/hosts", p("/etc/hosts")},
	{"truncated csi at line end", "\x1b[\n/etc/hosts", p("/etc/hosts")},
	{"escape before newline", "x\x1b\n/etc/hosts", p("/etc/hosts")},
	{"lone carriage return", "10%\r/opt/missing/lib/a.so", p("/opt/missing/lib/a.so")},
	{"cursor forward", "abc\x1b[3C/opt/missing/lib/a.so", p("/opt/missing/lib/a.so")},
	{"cursor position", "abc\x1b[5;9H/opt/missing/lib/a.so", p("/opt/missing/lib/a.so")},
	{"zero width space before", "\u200b/etc/hosts", p("/etc/hosts")},
	{"zero width space after", "/etc/hosts\u200b", p("/etc/hosts")},
	{"zero width space between", "/etc/hosts\u200b/tmp/out.bin", p("/etc/hosts", "/tmp/out.bin")},
	{"non-breaking space before", "ls\u00a0/etc/hosts", p("/etc/hosts")},
	{"non-breaking space between", "/etc/hosts\u00a0/tmp/out.bin", p("/etc/hosts", "/tmp/out.bin")},
	{"tab between", "a\t/etc/hosts\t/tmp/out.bin", p("/etc/hosts", "/tmp/out.bin")},

	// Quoting and escaping.
	{"json-escaped quotes", `\"/tmp/out.bin\"`, p("/tmp/out.bin")},
	{"json in json", `{\"path\":\"/opt/missing/x.txt\"}`, p("/opt/missing/x.txt")},
	{"trailing literal newline escape", `"/tmp/out.bin\n"`, p("/tmp/out.bin")},
	{"trailing literal newline missing", `"/opt/missing/x.txt\n"`, p("/opt/missing/x.txt")},
	{"line continuation", "cp /tmp/out.bin /opt/missing/x.txt\\\n  /tmp/", p("/tmp/out.bin", "/opt/missing/x.txt", "/tmp/")},
	{"escaped quotes in quoted name", `"/opt/missing/a \"b\" c.txt"`, p(`/opt/missing/a \"b\" c.txt`)},

	// Tilde forms.
	{"other user's home", "~other/notes.md", p("~other/notes.md")},
	{"other user's missing", "~nobody/missing/x.txt", p("~nobody/missing/x.txt")},
	{"pwd tilde", "~+/missing/x.txt", p("~+/missing/x.txt")},
	{"tilde with unicode", "~/日本/x.txt", p("~/日本/x.txt")},
	{"dot dot chain", "../../missing/x.txt", p("../../missing/x.txt")},
	{"dot chain", "././src/main.rs", p("././src/main.rs")},

	// Locations.
	{"column range", "src/main.rs:12:4-9", p("src/main.rs@12:4")},
	{"line range", "src/main.rs:12-20", p("src/main.rs@12")},
	{"github fragment", "src/main.rs#L12", p("src/main.rs@12")},
	{"github fragment range", "src/main.rs#L12-L20", p("src/main.rs@12")},
	{"github fragment column", "src/main.rs#L12C5", p("src/main.rs@12:5")},
	{"fragment on missing file", "src/gone.rs#L7", p("src/gone.rs@7")},
	{"python frame", `  File "/usr/lib/python3.12/os.py", line 10, in <module>`, p("/usr/lib/python3.12/os.py@10")},
	{"python relative frame", `File "src/main.rs", line 3`, p("src/main.rs@3")},
	{"php on line", "in /usr/lib/python3.12/os.py on line 7", p("/usr/lib/python3.12/os.py@7")},
	{"go trace", "\t/home/u/proj/src/main.rs:12 +0x1f", p("/home/u/proj/src/main.rs@12")},
	{"rust panic", "thread 'main' panicked at src/main.rs:12:5:", p("src/main.rs@12:5")},
	{"node frame", "    at foo (src/main.rs:3:5)", p("src/main.rs@3:5")},
	{"msbuild", `C:\x\y.cs(12,5): error CS1002`, p(`C:\x\y.cs@12:5`)},
	{"not a line", "src/main.rs:abc", p("src/main.rs")},
	{"double colon", "src/main.rs::foo", p("src/main.rs")},
	{"grep context line", "src/main.rs-12-fn main() {", p("src/main.rs@12")},
	{"rotated log is one name", "app.log-2026-11-12", nil},

	// Diff headers.
	{"mnemonic diff prefixes", "diff --git i/src/main.rs w/src/main.rs", p("src/main.rs", "src/main.rs")},
	{"rename from", "rename from src/main.rs", p("src/main.rs")},
	{"timestamped header", "--- a/src/main.rs\t2026-11-12 22:00:00", p("src/main.rs")},

	// Rows from tools.
	{"git status new file", "\tnew file:   src/lib.rs", p("src/lib.rs")},
	{"git status both modified", "\tboth modified:   Cargo.toml", p("Cargo.toml")},
	{"tree branch", "│   ├── main.rs", p("main.rs")},
	{"tree with classify", "├── Cargo.toml*", p("Cargo.toml")},
	{"ls classify at sign", "README.md@", p("README.md")},
	{"find output", "./src/main.rs", p("./src/main.rs")},
	{"rg line col", "src/main.rs:12:5:fn main()", p("src/main.rs@12:5")},
	{"pytest failure", "FAILED tests/test_x.py::test_a - assert 1 == 2", p("tests/test_x.py")},
	{"tsc", "src/app.ts:12:5 - error TS2322: bad", p("src/app.ts@12:5")},

	// Neighbours of a path.
	{"colon prose", "/etc/hosts:Permission denied", p("/etc/hosts")},
	{"close paren", "(/etc/hosts)", p("/etc/hosts")},
	{"bracket comma", "[/etc/hosts],", p("/etc/hosts")},
	{"quote comma", `"/etc/hosts",`, p("/etc/hosts")},
	{"quote period", "'/etc/hosts'.", p("/etc/hosts")},
	{"json value", `{"path":"/etc/hosts"}`, p("/etc/hosts")},
	{"html attribute", `<a href="/etc/hosts">`, p("/etc/hosts")},
	{"key value pairs", "file=/etc/hosts,line=3", p("/etc/hosts")},
	{"ellipsis glued", "/etc/hosts…more", p("/etc/hosts")},

	// Characters inside names.
	{"plus", "/tmp/a+b.txt", p("/tmp/a+b.txt")},
	{"at sign inside", "/tmp/x@y.txt", p("/tmp/x@y.txt")},
	{"percent inside", "/tmp/100%.txt", p("/tmp/100%.txt")},
	{"hash inside", "a#b.txt", p("a#b.txt")},
	{"backup tilde", "notes.txt~", p("notes.txt~")},
	{"missing backup tilde", "foo.txt~", p("foo.txt~")},
	{"missing with plus dirs", "/opt/missing/c++/x.h", p("/opt/missing/c++/x.h")},
	{"missing scoped package", "/opt/missing/@scope/pkg/index.js", p("/opt/missing/@scope/pkg/index.js")},
	{"hidden file", ".hidden", p(".hidden")},
	{"hidden directory", ".github/workflows/ci.yml", p(".github/workflows/ci.yml")},
	{"trailing slash directory", "/tmp/dir/", p("/tmp/dir/")},

	// Not paths, or not worth a highlight.
	{"caret alone", "^C", nil},
	{"caret then word", "^C foo", nil},
	{"caret in a pattern", "^README.md", nil},
	{"caret then bare root", "^C/", nil},
	{"caret then single component", "^C/x", nil},
	{"mid-word single component", "and/etc", nil},
	{"mid-word top-level dirs", "src/lib foo/bin foo/usr", nil},
	{"prose slashes", "either/or read/write input/output", nil},
	{"prefix directory exists", "src/etc/hosts", nil},
	{"flag letter alone", "-I -o", nil},
	{"unknown glyph then word", "x/y", nil},
	{"glob before a mid-word root", "lib*/etc/hosts", nil},
}

// Hosts and URLs own their own text; a path found inside one would be a
// second, wrong target.
func TestNoPathInsideHostOrURL(t *testing.T) {
	for _, in := range []string{"http://example.com/etc/hosts", "s3://bucket/etc/hosts", "gs://bucket/key.txt", "example.com:443/etc/hosts", "localhost:3000/etc/hosts"} {
		t.Run(in, func(t *testing.T) {
			for _, m := range Scan(in, resolve.New(pathEnv())) {
				if m.Kind == KindPath {
					t.Errorf("Scan(%q) found path %q", in, m.Text)
				}
			}
		})
	}
}

func scanTexts(in string, env resolve.Env) []string {
	var got []string
	for _, m := range Scan(in, resolve.New(env)) {
		got = append(got, render(m))
	}
	return got
}

func TestPathCases(t *testing.T) {
	for _, tc := range pathCases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, scanTexts(tc.in, pathEnv())); diff != "" {
				t.Errorf("Scan(%q) (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}

// The highlighted range must exclude the junk in front of the path.
func TestPathCaseOffsets(t *testing.T) {
	for _, tc := range pathCases {
		if strings.ContainsAny(tc.in, "\x1b\r\x00") {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			prev := 0
			for _, m := range Scan(tc.in, resolve.New(pathEnv())) {
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

func TestPathCaseOffsetsThroughEscapes(t *testing.T) {
	in := "\x1b[31m^C\x1b[0m/etc/hosts"
	ms := Scan(in, resolve.New(pathEnv()))
	if len(ms) != 1 {
		t.Fatalf("got %d matches, want 1: %v", len(ms), ms)
	}
	if got := in[ms[0].Start:ms[0].End]; got != "/etc/hosts" {
		t.Errorf("range covers %q, want /etc/hosts", got)
	}
}

func TestPathResolutions(t *testing.T) {
	cases := []struct {
		name, in, path string
		exists         bool
	}{
		{"junk-stripped suffix is what resolves", "^C/etc/hosts", "/etc/hosts", true},
		{"mid-word root", "x/etc/hosts", "/etc/hosts", true},
		{"colon list element", "PATH=/usr/bin:/usr/include", "/usr/include", true},
		{"other user's home", "~other/notes.md", "/home/other/notes.md", true},
		{"fragment is not part of the file", "src/main.rs#L12", "/home/u/proj/src/main.rs", true},
		{"git prefix still names the repo file", "--- a/etc/hosts", "/home/u/proj/etc/hosts", true},
		{"backup tilde", "notes.txt~", "/home/u/proj/notes.txt~", true},
		{"missing path keeps its place", "^C" + reportedPath, reportedPath, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ms := Scan(tc.in, resolve.New(pathEnv()))
			if len(ms) == 0 {
				t.Fatalf("Scan(%q) found nothing", tc.in)
			}
			m := ms[len(ms)-1]
			if m.Path != tc.path || m.Exists != tc.exists {
				t.Errorf("got (%q, exists=%v), want (%q, exists=%v)", m.Path, m.Exists, tc.path, tc.exists)
			}
		})
	}
}
