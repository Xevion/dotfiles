package preview

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode"

	"github.com/google/go-cmp/cmp"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFile(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		b.WriteString("line\t" + strings.Repeat("x", i%3) + "\x1b[31m\n")
	}
	p := write(t, "a.txt", b.String())

	top := File(p, 0, 3)
	if diff := cmp.Diff([]Line{{No: 1, Text: "line    x[31m"}, {No: 2, Text: "line    xx[31m"}, {No: 3, Text: "line    [31m"}}, top.Lines); diff != "" {
		t.Errorf("tabs expand, escapes lose their ESC (-want +got):\n%s", diff)
	}
	at := File(p, 30, 9)
	if at.Lines[0].No != 27 || !at.Lines[3].Focus || at.Lines[3].No != 30 {
		t.Errorf("focus line should sit a third of the way down: %+v", at.Lines)
	}
	if past := File(p, 99, 5); past.Note == "" {
		t.Error("a line past the end should say so")
	}
}

func TestFileNotes(t *testing.T) {
	for _, tc := range []struct{ name, content, want string }{
		{"empty", "", "empty file"},
		{"bin", "\x7fELF\x02\x01", "application/x-executable"},
	} {
		if got := File(write(t, tc.name, tc.content), 0, 5); got.Note != tc.want || got.Lines != nil {
			t.Errorf("%s: %+v, want note %q", tc.name, got, tc.want)
		}
	}
	if got := File("/nonexistent/x", 0, 5); got.Note == "" {
		t.Error("an unreadable file should leave a note")
	}
}

func TestDir(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.txt", "A.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "zdir"), 0o700); err != nil {
		t.Fatal(err)
	}
	want := []Line{{Text: "zdir", Dir: true}, {Text: "A.txt"}, {Text: "b.txt"}}
	got := Dir(dir, 3)
	if diff := cmp.Diff(want, got.Lines); diff != "" || got.More != 1 {
		t.Errorf("folders first, then names, then a count (-want +got):\n%s\nmore=%d", diff, got.More)
	}
	if got := Dir(dir, 0); len(got.Lines) != 0 || got.More != 4 {
		t.Errorf("no rows lists nothing: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "x\x1b_Gq\x1b\\\n"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Dir(dir, 8); slices.ContainsFunc(got.Lines, func(l Line) bool { return strings.ContainsFunc(l.Text, unicode.IsControl) }) {
		t.Errorf("entry names must not carry control characters: %+v", got.Lines)
	}
	if got := Dir(t.TempDir(), 3); got.Note != "empty folder" {
		t.Errorf("empty folder note: %+v", got)
	}
}

// Opening a named pipe for reading blocks until something writes to it, which
// would hang the pane's loader for good.
func TestFileDoesNotBlockOnAPipe(t *testing.T) {
	p := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skip(err)
	}
	done := make(chan Preview, 1)
	go func() { done <- File(p, 0, 5) }()
	select {
	case got := <-done:
		if got.Note == "" {
			t.Errorf("a pipe should leave a note: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("File blocked on a named pipe")
	}
}
