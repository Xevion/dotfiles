package action

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
	"screenpick/internal/resolve"
)

var disk = fakefs.New("/home/u/proj/.git/", "/home/u/proj/src/main.rs", "/tmp/build-cache/", "/home/u/.wine/drive_c/app.exe", "/home/u/proj/tool")

// heads stands in for file contents; anything unlisted reads as text.
var heads = map[string]string{
	"/home/u/.wine/drive_c/app.exe": "MZ\x90\x00",
	"/home/u/proj/tool":             "\x7fELF\x02",
}

func head(p string) ([]byte, error) { return []byte(heads[p]), nil }

func lookPath(installed ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		if slices.Contains(installed, name) {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func pathMatch(text, p string, exists, dir bool) detect.Match {
	return detect.Match{Kind: detect.KindPath, Text: text, Resolution: resolve.Resolution{Path: p, Exists: exists, IsDir: dir}}
}

// keyed renders actions as "key verb arg" for compact comparison, with the
// directory and argv of commands that carry them.
func keyed(as []Action) []string {
	names := map[Verb]string{Copy: "copy", Open: "open", Edit: "edit", Tab: "tab", Run: "run", Git: "git", Search: "search"}
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.Key + " " + names[a.Verb] + " " + a.Arg
		if a.Argv != nil {
			out[i] += " [" + strings.Join(a.Argv, " ") + " in " + a.Dir + "]"
		}
	}
	return out
}

func TestFor(t *testing.T) {
	ctx := Context{
		Cwd: "/home/u/proj", Stat: disk.Lstat, Head: head, LookPath: lookPath("wine", "ida"),
		CommitRepo: func(sha string) string {
			if sha == "3f2a9c1" {
				return "/home/u/proj"
			}
			return ""
		},
	}
	cases := []struct {
		name string
		m    detect.Match
		want []string
	}{
		{"file in a repo: shown equals relative and repo path, so one copy", pathMatch("src/main.rs", "/home/u/proj/src/main.rs", true, false), []string{
			"e edit /home/u/proj/src/main.rs",
			"o open /home/u/proj/src/main.rs",
			"f open /home/u/proj/src",
			"t tab /home/u/proj/src",
			"R tab /home/u/proj",
			"l git src/main.rs [log --follow -- src/main.rs in /home/u/proj]",
			"y copy /home/u/proj/src/main.rs",
			"r copy src/main.rs",
			"Y copy /home/u/proj/src",
			"/ search src/main.rs",
		}},
		{"absolute file outside cwd: shown equals path", pathMatch("/etc/hosts", "/etc/hosts", true, false), []string{
			"e edit /etc/hosts",
			"o open /etc/hosts",
			"f open /etc",
			"t tab /etc",
			"y copy /etc/hosts",
			"Y copy /etc",
			"/ search /etc/hosts",
		}},
		{"tilde form is its own copy", pathMatch("~/proj/src/main.rs", "/home/u/proj/src/main.rs", true, false), []string{
			"e edit /home/u/proj/src/main.rs",
			"o open /home/u/proj/src/main.rs",
			"f open /home/u/proj/src",
			"t tab /home/u/proj/src",
			"R tab /home/u/proj",
			"l git src/main.rs [log --follow -- src/main.rs in /home/u/proj]",
			"y copy /home/u/proj/src/main.rs",
			"r copy src/main.rs",
			"s copy ~/proj/src/main.rs",
			"Y copy /home/u/proj/src",
			"/ search ~/proj/src/main.rs",
		}},
		{"windows program: wine and a disassembler, no editor", pathMatch(`C:\app.exe`, "/home/u/.wine/drive_c/app.exe", true, false), []string{
			"w run /home/u/.wine/drive_c/app.exe [/usr/bin/wine /home/u/.wine/drive_c/app.exe in /home/u/.wine/drive_c]",
			"i run /home/u/.wine/drive_c/app.exe [/usr/bin/ida /home/u/.wine/drive_c/app.exe in /home/u/.wine/drive_c]",
			"f open /home/u/.wine/drive_c",
			"t tab /home/u/.wine/drive_c",
			"y copy /home/u/.wine/drive_c/app.exe",
			`s copy C:\app.exe`,
			"Y copy /home/u/.wine/drive_c",
			`/ search C:\app.exe`,
		}},
		{"native program: no editor, no wine", pathMatch("tool", "/home/u/proj/tool", true, false), []string{
			"i run /home/u/proj/tool [/usr/bin/ida /home/u/proj/tool in /home/u/proj]",
			"f open /home/u/proj",
			"t tab /home/u/proj",
			"l git tool [log --follow -- tool in /home/u/proj]",
			"y copy /home/u/proj/tool",
			"r copy tool",
			"Y copy /home/u/proj",
			"/ search tool",
		}},
		{"directory", pathMatch("/tmp/build-cache", "/tmp/build-cache", true, true), []string{
			"o open /tmp/build-cache",
			"t tab /tmp/build-cache",
			"y copy /tmp/build-cache",
			"/ search /tmp/build-cache",
		}},
		{"repo root itself", pathMatch(".", "/home/u/proj", true, true), []string{
			"o open /home/u/proj",
			"t tab /home/u/proj",
			"y copy /home/u/proj",
			"s copy .",
			"/ search .",
		}},
		// The reported menu: a missing absolute path offered the same copy
		// twice and two folder actions for one folder.
		{"missing absolute file", pathMatch("/tmp/build-cache/5b090f6779e3.log", "/tmp/build-cache/5b090f6779e3.log", false, false), []string{
			"s copy /tmp/build-cache/5b090f6779e3.log",
			"f open /tmp/build-cache",
			"/ search /tmp/build-cache/5b090f6779e3.log",
		}},
		{"missing with no existing ancestor", pathMatch("/nope/x.rs", "/nope/x.rs", false, false), []string{
			"s copy /nope/x.rs",
			"/ search /nope/x.rs",
		}},
		{"unplaceable token", pathMatch(`\\srv\share\x.txt`, "", false, false), []string{
			`s copy \\srv\share\x.txt`,
			`/ search \\srv\share\x.txt`,
		}},
		{"url", detect.Match{Kind: detect.KindURL, Text: "https://x.y/a"}, []string{
			"o open https://x.y/a",
			"y copy https://x.y/a",
		}},
		{"host with port", detect.Match{Kind: detect.KindHost, Text: "localhost:5173"}, []string{
			"o open http://localhost:5173",
			"y copy localhost:5173",
			"h copy localhost",
		}},
		{"bare ip", detect.Match{Kind: detect.KindHost, Text: "10.0.0.1"}, []string{"y copy 10.0.0.1"}},
		{"bare domain opens over https", detect.Match{Kind: detect.KindHost, Text: "example.com"}, []string{
			"o open https://example.com",
			"y copy example.com",
		}},
		{"bare local name opens over http", detect.Match{Kind: detect.KindHost, Text: "nas.lan"}, []string{
			"o open http://nas.lan",
			"y copy nas.lan",
		}},
		{"schemeless url gets a scheme to open", detect.Match{Kind: detect.KindURL, Text: "docs.rs/serde"}, []string{
			"o open https://docs.rs/serde",
			"y copy docs.rs/serde",
		}},
		{"custom scheme opens as written", detect.Match{Kind: detect.KindURL, Text: "steam://run/1"}, []string{
			"o open steam://run/1",
			"y copy steam://run/1",
		}},
		{"scp remote can only be copied", detect.Match{Kind: detect.KindURL, Text: "git@github.com:org/repo.git"}, []string{
			"y copy git@github.com:org/repo.git",
		}},
		{"sha runs git where the commit is", detect.Match{Kind: detect.KindSHA, Text: "3f2a9c1"}, []string{
			"g git 3f2a9c1 [show 3f2a9c1 in /home/u/proj]",
			"R tab /home/u/proj",
			"y copy 3f2a9c1",
		}},
		{"sha in no repo can only be copied", detect.Match{Kind: detect.KindSHA, Text: "abcdef1"}, []string{"y copy abcdef1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, keyed(For(tc.m, ctx))); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
	withLine := pathMatch("src/main.rs", "/home/u/proj/src/main.rs", true, false)
	withLine.Line, withLine.Col = 12, 4
	edit := For(withLine, ctx)[0]
	if edit.Label != "edit at line 12" || edit.Line != 12 || edit.Col != 4 {
		t.Errorf("edit action lost its location: %+v", edit)
	}
}

func TestMenusHaveUniqueKeys(t *testing.T) {
	ms := []detect.Match{
		pathMatch("src/main.rs", "/home/u/proj/src/main.rs", true, false),
		pathMatch("x", "/tmp/build-cache", true, true),
		pathMatch("gone.rs", "/home/u/proj/gone.rs", false, false),
		pathMatch("app.exe", "/home/u/.wine/drive_c/app.exe", true, false),
	}
	for _, m := range ms {
		seen := map[string]bool{}
		for _, a := range For(m, Context{Cwd: "/elsewhere", Stat: disk.Lstat, Head: head, LookPath: lookPath("wine", "ida", "ghidraRun")}) {
			if seen[a.Key] {
				t.Errorf("%q: key %q used twice", m.Text, a.Key)
			}
			seen[a.Key] = true
		}
	}
}

func TestRepoRoot(t *testing.T) {
	for dir, want := range map[string]string{
		"/home/u/proj/src": "/home/u/proj",
		"/home/u/proj":     "/home/u/proj",
		"/tmp/build-cache": "",
		"/":                "",
	} {
		if got := RepoRoot(dir, disk.Lstat); got != want {
			t.Errorf("RepoRoot(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestEditorArgs(t *testing.T) {
	cases := []struct {
		editor    string
		line, col int
		want      []string
	}{
		{"", 0, 0, []string{"micro", "f.rs"}},
		{"nvim", 12, 4, []string{"nvim", "+12", "f.rs"}},
		{"/usr/bin/vim", 3, 0, []string{"/usr/bin/vim", "+3", "f.rs"}},
		{"hx", 12, 4, []string{"hx", "f.rs:12:4"}},
		{"zed", 12, 0, []string{"zed", "f.rs:12"}},
		{"code --wait", 12, 4, []string{"code", "--wait", "-g", "f.rs:12:4"}},
		{"unknown-editor", 12, 4, []string{"unknown-editor", "f.rs"}},
	}
	for _, tc := range cases {
		if diff := cmp.Diff(tc.want, EditorArgs(tc.editor, "f.rs", tc.line, tc.col)); diff != "" {
			t.Errorf("EditorArgs(%q, %d, %d) (-want +got):\n%s", tc.editor, tc.line, tc.col, diff)
		}
	}
}
