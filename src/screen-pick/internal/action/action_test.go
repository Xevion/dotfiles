package action

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
	"screenpick/internal/resolve"
)

var disk = fakefs.New("/home/u/proj/src/main.rs", "/tmp/build-cache/")

func pathMatch(text, p string, exists, dir bool) detect.Match {
	return detect.Match{Kind: detect.KindPath, Text: text, Resolution: resolve.Resolution{Path: p, Exists: exists, IsDir: dir}}
}

// keyed renders actions as "key verb arg" for compact comparison.
func keyed(as []Action) []string {
	names := map[Verb]string{Copy: "copy", Open: "open", Edit: "edit", Tab: "tab", Show: "show", Search: "search"}
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.Key + " " + names[a.Verb] + " " + a.Arg
	}
	return out
}

func TestFor(t *testing.T) {
	ctx := Context{Cwd: "/home/u/proj", Stat: disk.Lstat}
	withLine := pathMatch("src/main.rs", "/home/u/proj/src/main.rs", true, false)
	withLine.Line, withLine.Col = 12, 4
	cases := []struct {
		name string
		m    detect.Match
		want []string
	}{
		{"relative file: shown equals relative, so one copy", pathMatch("src/main.rs", "/home/u/proj/src/main.rs", true, false), []string{
			"e edit /home/u/proj/src/main.rs",
			"o open /home/u/proj/src/main.rs",
			"f open /home/u/proj/src",
			"t tab /home/u/proj/src",
			"y copy /home/u/proj/src/main.rs",
			"r copy src/main.rs",
			"d copy /home/u/proj/src",
			"/ search src/main.rs",
		}},
		{"absolute file outside cwd: shown equals path", pathMatch("/etc/hosts", "/etc/hosts", true, false), []string{
			"e edit /etc/hosts",
			"o open /etc/hosts",
			"f open /etc",
			"t tab /etc",
			"y copy /etc/hosts",
			"d copy /etc",
			"/ search /etc/hosts",
		}},
		{"tilde form is its own copy", pathMatch("~/proj/src/main.rs", "/home/u/proj/src/main.rs", true, false), []string{
			"e edit /home/u/proj/src/main.rs",
			"o open /home/u/proj/src/main.rs",
			"f open /home/u/proj/src",
			"t tab /home/u/proj/src",
			"y copy /home/u/proj/src/main.rs",
			"r copy src/main.rs",
			"s copy ~/proj/src/main.rs",
			"d copy /home/u/proj/src",
			"/ search ~/proj/src/main.rs",
		}},
		{"directory", pathMatch("/tmp/build-cache", "/tmp/build-cache", true, true), []string{
			"o open /tmp/build-cache",
			"t tab /tmp/build-cache",
			"y copy /tmp/build-cache",
			"/ search /tmp/build-cache",
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
		{"sha", detect.Match{Kind: detect.KindSHA, Text: "3f2a9c1"}, []string{"g show 3f2a9c1", "y copy 3f2a9c1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, keyed(For(tc.m, ctx))); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
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
	}
	for _, m := range ms {
		seen := map[string]bool{}
		for _, a := range For(m, Context{Cwd: "/elsewhere", Stat: disk.Lstat}) {
			if seen[a.Key] {
				t.Errorf("%q: key %q used twice", m.Text, a.Key)
			}
			seen[a.Key] = true
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
