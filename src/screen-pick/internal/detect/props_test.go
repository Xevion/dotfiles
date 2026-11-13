package detect

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"screenpick/internal/resolve"
)

// checkInvariants reports every way a scan of text can be structurally wrong.
func checkInvariants(text string, ms []Match) error {
	prevEnd := 0
	for i, m := range ms {
		switch {
		case m.Kind < KindPath || m.Kind > KindSHA:
			return fmt.Errorf("match %d: invalid kind %d", i, m.Kind)
		case m.Start < 0 || m.Start >= m.End || m.End > len(text):
			return fmt.Errorf("match %d: bad range [%d,%d) in %d bytes", i, m.Start, m.End, len(text))
		case m.Start < prevEnd:
			return fmt.Errorf("match %d: [%d,%d) overlaps or precedes previous end %d", i, m.Start, m.End, prevEnd)
		case m.Text == "":
			return fmt.Errorf("match %d: empty text", i)
		case strings.ContainsAny(m.Text, "\x1b\x00\r"):
			return fmt.Errorf("match %d: text %q holds raw control bytes", i, m.Text)
		case m.Line < 0 || m.Col < 0:
			return fmt.Errorf("match %d: negative location %d:%d", i, m.Line, m.Col)
		case m.Kind != KindPath && (m.Line != 0 || m.Col != 0 || m.Path != "" || m.Exists):
			return fmt.Errorf("match %d: %s carries path state", i, m.Kind)
		}
		prevEnd = m.End
		// The raw slice, cleaned on its own, must give back the visible text.
		// Hard breaks kept at a known width add newlines the lone slice cannot
		// know about, so only the rest is compared.
		got, _, _ := clean(text[m.Start:m.End], 0)
		if strip(got) != strip(m.Text) {
			return fmt.Errorf("match %d: text %q, but raw [%d,%d) cleans to %q", i, m.Text, m.Start, m.End, got)
		}
	}
	return nil
}

func strip(s string) string { return strings.ReplaceAll(s, "\n", "") }

func scanFor(text string, width int) []Match {
	return ScanWidth(text, resolve.New(screensEnv()), width)
}

func fragments(t testing.TB) []string {
	frags := []string{
		"/etc/hosts", "/home/u/proj/src/main.rs", "src/lib.rs:12:4", "~/.bashrc", "./x.go", "../up/y.py",
		"C:\\Users\\u\\a.txt", "\\\\srv\\share\\f", "%APPDATA%\\x", "$HOME/.cfg", "${HOME}/a",
		"https://example.com/a?b=1#c", "example.com", "localhost:3000", "10.0.0.1:80", "192.168.0.0/16", "[::1]:8080",
		"git@github.com:o/r.git", "mailto:a@b.co", "file:///etc/hosts", "file://host/home/u/x%20y.txt",
		"9c8d7e6", "3f2a1b49c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3", "fe80::1", "a.b.c", "1.2.3", "e.g.",
		"'quoted path/x.rs'", "\"dq/y.rs\"", "`bt.rs`", "(paren/z.rs)", "[br/w.rs]", "{cu/v.rs}", "<an/u.rs>",
		"a/b.c", "b/src/main.rs", "foo(1).txt", "My Documents/notes.txt", "file with spaces.txt",
		" ", "  ", "\t", "\n", "\r\n", "\r", "\x00", "\x00\n", ":", "::", ",", ".", "...", "-", "--", "*", "?", "#L12", "|", "=",
		"\x1b[31m", "\x1b[0m", "\x1b[2K", "\x1b[3C", "\x1b[", "\x1b", "\x1b]8;;https://e.com\x1b\\", "\x1b]8;;\x1b\\",
		"\x1b]8;;file:///etc/hosts\x07", "\x1b]0;title\x07", "\x1b(B", "\x1bP1$r\x1b\\", "\x1b]8;;",
		"日本", "é", "…", "│", "─", "\u200b", "\u0301", "😀", "\xff", "\xc3",
		"x", "ab", "dir/", "/", "//", "~", "~/", "$", "${", "%", "^C", "-I/usr/include",
	}
	for _, f := range screenFiles(t) {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.SplitSeq(string(raw), "\n") {
			if len(line) < 120 {
				frags = append(frags, line)
			}
		}
	}
	return frags
}

func TestScanInvariantsScreenPrefixes(t *testing.T) {
	for _, file := range screenFiles(t) {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		step := 1 + len(text)/10
		for n := 0; n <= len(text); n += step {
			for _, w := range []int{0, 40} {
				if err := checkInvariants(text[:n], scanFor(text[:n], w)); err != nil {
					t.Fatalf("%s[:%d] width %d: %v", file, n, w, err)
				}
			}
		}
	}
}

func FuzzScan(f *testing.F) {
	for _, file := range screenFiles(f) {
		raw, err := os.ReadFile(file)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(raw), uint8(0))
		f.Add(string(raw), uint8(80))
	}
	for _, tc := range scanCases {
		f.Add(tc.in, uint8(0))
	}
	for _, frag := range fragments(f) {
		f.Add(frag, uint8(20))
	}
	f.Fuzz(func(t *testing.T, text string, width uint8) {
		if err := checkInvariants(text, scanFor(text, int(width))); err != nil {
			t.Fatalf("width %d: %v", width, err)
		}
	})
}

func TestLexedResolveConcurrent(t *testing.T) {
	text := realisticScreen(t)
	l := Lex(text, 120)
	want := renderGolden(text, 120)
	done := make(chan string)
	for range 4 {
		go func() {
			var b strings.Builder
			for _, m := range l.Resolve(resolve.New(screensEnv())) {
				fmt.Fprintf(&b, "%s %q\n", m.Kind, m.Text)
			}
			done <- b.String()
		}()
	}
	var first string
	for i := range 4 {
		got := <-done
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("concurrent resolves disagree:\n%s\nvs\n%s", first, got)
		}
	}
	if want == "" || first == "" {
		t.Fatal("no matches")
	}
}

func TestCleanOffsets(t *testing.T) {
	check := func(text string, width int) {
		t.Helper()
		s, offs, _ := clean(text, width)
		if offs.at(len(s)) != len(text) {
			t.Fatalf("at(len) = %d, want %d for %q", offs.at(len(s)), len(text), text)
		}
		prev := -1
		for c := range len(s) {
			r := offs.at(c)
			if r <= prev || r >= len(text) {
				t.Fatalf("cleaned byte %d maps to %d (previous %d) in %d bytes: %q", c, r, prev, len(text), text)
			}
			// Bytes clean() invents stand for the escape or marker they replace.
			if s[c] != text[r] && s[c] != ' ' && s[c] != '\n' {
				t.Fatalf("cleaned byte %d is %q but original %d is %q: %q", c, s[c], r, text[r], text)
			}
			prev = r
		}
	}
	for _, file := range screenFiles(t) {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range []int{0, 40, 120} {
			check(string(raw), w)
		}
	}
	frags := fragments(t)
	rng := rand.New(rand.NewPCG(5, 9))
	for range 300 {
		var b strings.Builder
		for range 1 + rng.IntN(12) {
			b.WriteString(frags[rng.IntN(len(frags))])
		}
		check(b.String(), []int{0, 5, 40}[rng.IntN(3)])
	}
}
