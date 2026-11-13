package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"screenpick/internal/detect"
	"screenpick/internal/preview"
	"screenpick/internal/resolve"
)

func sizes(m *Model, w, h int) { m.Update(tea.WindowSizeMsg{Width: w, Height: h}) }

// Opening a menu over the list in a window too short to hold it used to slice
// with a negative bound.
func TestListMenuInATinyWindow(t *testing.T) {
	for _, h := range []int{1, 2, 3, 4, 5} {
		for _, w := range []int{1, 5, 19, 60} {
			m, _ := newTest(t, List)
			sizes(m, w, h)
			press(m, "enter")
			checkFits(t, m, w, h)
			press(m, "tab", "esc", "esc")
			checkFits(t, m, w, h)
		}
	}
}

// Labels replace cells of the match without moving anything after them, even
// where the first cell is half of a wide character or the match is shorter
// than its label.
func TestLabelsKeepRowWidth(t *testing.T) {
	const sc = "日本語/ファイル.txt and a x/y"
	ms := []detect.Match{
		{Kind: detect.KindPath, Start: 0, End: len("日本語/ファイル.txt"), Text: "日本語/ファイル.txt"},
		{Kind: detect.KindPath, Start: len("日本語/ファイル.txt and "), End: len("日本語/ファイル.txt and a"), Text: "a"},
		{Kind: detect.KindPath, Start: len("日本語/ファイル.txt and a "), End: len(sc), Text: "x/y"},
	}
	m := New(Config{Mode: Hints, Screen: sc, Matches: ms, Backend: &fakeBackend{}})
	m.labels = []string{"ab", "cd", "ef"}
	sizes(m, 80, 3)
	want := ansi.StringWidth(sc)
	if got := ansi.StringWidth(strings.Split(m.View().Content, "\n")[0]); got != want {
		t.Errorf("row is %d cells with labels, %d without: %q", got, want, ansi.Strip(m.View().Content))
	}
	m.typed = "a"
	if got := ansi.StringWidth(strings.Split(m.View().Content, "\n")[0]); got != want {
		t.Errorf("with a prefix typed the row is %d cells, want %d", got, want)
	}
}

// Matches that overlap (a URL's own path, say) must not panic the redraw.
func TestOverlappingMatchesDoNotPanic(t *testing.T) {
	const sc = "see https://x.y/a/b.rs now"
	ms := []detect.Match{
		{Kind: detect.KindURL, Start: 4, End: 22, Text: "https://x.y/a/b.rs"},
		{Kind: detect.KindPath, Start: 14, End: 22, Text: "/a/b.rs"},
	}
	m := New(Config{Mode: Hints, Screen: sc, Matches: ms, Backend: &fakeBackend{}})
	sizes(m, 40, 3)
	checkFits(t, m, 40, 3)
}

// Once any key was pressed a late rescan must not renumber labels, or the
// letters under the user's fingers change meaning.
func TestLateRescanAfterAMenuKeepsLabels(t *testing.T) {
	m, b := newTest(t, Hints)
	press(m, "s", "esc")
	before := append([]string(nil), m.labels...)
	fresh := append([]detect.Match(nil), b.refined...)
	fresh = append(fresh, detect.Match{Kind: detect.KindSHA, Start: 0, End: 3, Text: "see"})
	m.Update(refinedMsg{fresh})
	if len(m.matches) != 3 || strings.Join(m.labels, "") != strings.Join(before, "") {
		t.Errorf("labels moved after input: %v -> %v (%d matches)", before, m.labels, len(m.matches))
	}
}

func TestIconOfUnplacedPathIsNotUnknown(t *testing.T) {
	m, _ := newTest(t, Hints)
	mt := detect.Match{Kind: detect.KindPath, Resolution: resolve.Resolution{Unknown: true}}
	if m.unsure(mt) {
		t.Error("a path with no place is not merely unchecked")
	}
}

// Names and messages that come from the disk or from a program can hold
// terminal commands; none may reach the frame as one.
func TestDiskStringsNeverReachTheFrameAsControls(t *testing.T) {
	const evil = "x\x1b_Gi=1;AAAA\x1b\\\x1bP+q544e\x1b\\\x1b]8;;http://e\x1b\\\r\nZ"
	m, _ := newTest(t, Hints)
	sizes(m, 140, 40)
	m.matches[1].Path = "/home/u/" + evil
	m.matches[1].Alternatives = []resolve.Candidate{{Path: "/mnt/" + evil, Source: resolve.SourceSearch}}
	press(m, "s")
	m.status = evil
	m.previews[previewKey(m.target())] = preview.Preview{Note: evil}
	for _, view := range []string{m.View().Content, func() string { press(m, "tab"); return m.View().Content }()} {
		for _, bad := range []string{"\x1b_", "\x1bP", "\x1b]", "\r", "\u009b"} {
			if strings.Contains(view, bad) {
				t.Errorf("frame carries %q:\n%q", bad, view)
			}
		}
		if n := len(strings.Split(view, "\n")); n > 40 {
			t.Errorf("a newline in a name made the frame %d rows tall", n)
		}
	}
}
