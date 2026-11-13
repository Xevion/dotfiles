package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
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
