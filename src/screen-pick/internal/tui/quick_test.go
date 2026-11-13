package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
	"screenpick/internal/preview"
	"screenpick/internal/resolve"
)

// textual is what a scan that cannot see the disk finds in screen.
func textual() []detect.Match {
	return detect.ScanWidth(screen, resolve.New(resolve.NoDisk("/home/u")), 19)
}

func newQuick(t *testing.T, quick func(context.Context) []detect.Match) (*Model, *fakeBackend) {
	t.Helper()
	disk := fakefs.New("/etc/hosts", "/home/u/proj/src/main.rs")
	b := &fakeBackend{disk: disk}
	m := New(Config{Mode: Hints, Screen: screen, Matches: textual(), Quick: quick, Backend: b, Home: "/home/u"})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	return m, b
}

// The first frame must not depend on the disk: it draws from text alone, with
// paths neutral rather than claimed missing, and fills in when results land.
func TestFirstFrameDoesNotWaitForTheDisk(t *testing.T) {
	block := make(chan struct{})
	m, _ := newQuick(t, func(context.Context) []detect.Match { <-block; return nil })
	defer close(block)

	frame := m.View().Content
	if !strings.Contains(ansi.Strip(frame), "and src/main.rs:12 too") {
		t.Errorf("the screen is drawn before any disk check:\n%s", ansi.Strip(frame))
	}
	if strings.Contains(frame, iconMissing) {
		t.Errorf("an unchecked path must not be drawn as missing")
	}
	if got := m.place(m.matches[0]); got != "checking" {
		t.Errorf("place of an unchecked path = %q", got)
	}
}

func TestQuickResultCoversUncheckedPaths(t *testing.T) {
	m, b := newQuick(t, func(context.Context) []detect.Match { return nil })
	r := resolve.New(resolve.Env{
		Bases: []resolve.Base{{Dir: "/home/u/proj", Source: resolve.SourceProcess}},
		Home:  "/home/u", Lstat: b.disk.Lstat, Readlink: b.disk.Readlink, ReadDir: b.disk.ReadDir,
	})
	checked := detect.ScanWidth(screen, r, 19)

	m.Update(quickMsg{checked})
	if m.unchecked {
		t.Fatal("a disk result ends the unchecked state")
	}
	var found int
	for _, mt := range m.matches {
		if mt.Kind == detect.KindPath && mt.Exists {
			found++
		}
	}
	if found != 2 {
		t.Errorf("want both paths found after the quick pass, got %d", found)
	}
}

// The fuller rescan can finish first; the quick one arriving after it is stale.
func TestLateQuickResultIsIgnored(t *testing.T) {
	m, _ := newQuick(t, func(context.Context) []detect.Match { return nil })
	m.Update(refinedMsg{nil})
	m.Update(quickMsg{textual()})
	if len(m.matches) != 0 {
		t.Errorf("a stale quick result replaced the refined one: %d matches", len(m.matches))
	}
}

// A path whose check timed out is not known to be missing: nothing on screen
// may say "not found" about it, though a clean miss still does.
func TestTimedOutPathIsDrawnUnknownNotMissing(t *testing.T) {
	m, _ := newQuick(t, func(context.Context) []detect.Match { return nil })
	ms := textual()
	var paths []int
	for i := range ms {
		if ms[i].Kind == detect.KindPath {
			ms[i].Unknown = true
			paths = append(paths, i)
		}
	}
	m.Update(quickMsg{ms})

	if got := m.place(ms[paths[0]]); got != "not checked" {
		t.Errorf("place of a timed-out path = %q", got)
	}
	if ic, _ := m.icon(ms[paths[0]]); ic == iconMissing {
		t.Error("a timed-out path must not get the missing icon")
	}
	if !strings.Contains(ansi.Strip(strings.Join(m.infoLines(ms[paths[0]], 60), "\n")), "could not check") {
		t.Error("the details should say the check could not be made")
	}

	m.mode, m.origin = List, List
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	frame := ansi.Strip(m.View().Content)
	rows := strings.Split(frame, "\n")
	for _, row := range rows[:len(rows)-1] {
		if strings.Contains(row, "not found") {
			t.Errorf("a list row says not found for a path that timed out: %q", row)
		}
	}

	ms[paths[0]].Unknown = false
	m.Update(refinedMsg{ms})
	if got := m.place(m.matches[paths[0]]); got != "not found" {
		t.Errorf("a clean miss reads not found, got %q", got)
	}
}

type lateRepo struct {
	*fakeBackend
	found bool
}

func (l *lateRepo) Preview(_ context.Context, m detect.Match, rows int) preview.Preview {
	if m.Kind == detect.KindSHA && !l.found {
		return preview.Preview{Note: "not a commit in any repository this window uses"}
	}
	return preview.Text("commit "+m.Text, rows)
}

// A commit looked up before the rescans know the window's checkouts reads as
// unknown; the answer must be asked again once they land.
func TestCommitPreviewIsReadAgainAfterARescan(t *testing.T) {
	m, f := newTest(t, List)
	l := &lateRepo{fakeBackend: f}
	m.backend = l
	m.setMatches(append(m.matches, detect.Match{Kind: detect.KindSHA, Start: 0, End: 7, Text: "abc1234"}))
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	drain(m, cmd)
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "not a commit") {
		t.Fatalf("fixture: the first read should say the commit is unknown:\n%s", view)
	}

	l.found = true
	_, cmd = m.Update(refinedMsg{m.matches})
	drain(m, cmd)
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "commit abc1234") {
		t.Errorf("the commit should be read again after the rescan:\n%s", view)
	}
}
