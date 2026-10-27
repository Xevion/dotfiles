package tui

import (
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/go-cmp/cmp"

	"screenpick/internal/action"
	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
	"screenpick/internal/lookup"
	"screenpick/internal/resolve"
)

func TestLabels(t *testing.T) {
	for _, n := range []int{1, 5, 26, 27, 100, 700} {
		ls := labels(n)
		if len(ls) != n {
			t.Fatalf("labels(%d) gave %d", n, len(ls))
		}
		seen := map[string]bool{}
		for i, a := range ls {
			if seen[a] {
				t.Errorf("labels(%d): %q repeated", n, a)
			}
			seen[a] = true
			if i > 0 && len(a) < len(ls[i-1]) {
				t.Errorf("labels(%d) not shortest first at %d", n, i)
			}
			for _, b := range ls {
				if a != b && strings.HasPrefix(b, a) {
					t.Errorf("labels(%d): %q is a prefix of %q", n, a, b)
				}
			}
		}
	}
	if got := assignLabels(3); got[2] != "a" || got[1] != "s" || got[0] != "d" {
		t.Errorf("bottom match should get the first label: %q", got)
	}
}

func TestGrid(t *testing.T) {
	g := newGrid("ab\r\ncd日本ef\nlast")
	if diff := cmp.Diff([]string{"ab", "cd日本ef", "last"}, []string{g.line(0), g.line(1), g.line(2)}); diff != "" {
		t.Errorf("rows (-want +got):\n%s", diff)
	}
	for _, tc := range []struct{ off, row, col int }{
		{0, 0, 0}, {2, 0, 2}, {4, 1, 0}, {6, 1, 2}, {12, 1, 6}, {15, 2, 0}, {19, 2, 4},
	} {
		if r, c := g.position(tc.off); r != tc.row || c != tc.col {
			t.Errorf("position(%d) = (%d,%d), want (%d,%d)", tc.off, r, c, tc.row, tc.col)
		}
	}
	want := []piece{{0, 1, 2}, {1, 4, 6}}
	if diff := cmp.Diff(want, g.pieces(1, 6), cmp.AllowUnexported(piece{})); diff != "" {
		t.Errorf("pieces (-want +got):\n%s", diff)
	}
}

type fakeBackend struct {
	disk     *fakefs.FS
	refined  []detect.Match
	index    []lookup.Result
	deep     []lookup.Result
	launched []action.Action
	commands []action.Action
}

func feed(rs []lookup.Result) <-chan lookup.Result {
	ch := make(chan lookup.Result, len(rs))
	for _, r := range rs {
		ch <- r
	}
	close(ch)
	return ch
}

func (f *fakeBackend) Refine(context.Context) []detect.Match { return f.refined }
func (f *fakeBackend) Index(context.Context, []detect.Match) <-chan lookup.Result {
	return feed(f.index)
}
func (f *fakeBackend) Deep(context.Context, int, detect.Match) <-chan lookup.Result {
	return feed(f.deep)
}
func (f *fakeBackend) Actions(m detect.Match) []action.Action {
	return action.For(m, action.Context{Cwd: "/home/u/proj", Stat: f.disk.Lstat})
}
func (f *fakeBackend) Launch(a action.Action) error {
	f.launched = append(f.launched, a)
	return nil
}
func (f *fakeBackend) Command(a action.Action) *exec.Cmd {
	f.commands = append(f.commands, a)
	return exec.Command("true")
}
func (f *fakeBackend) Stat(p string) (fs.FileInfo, error) { return f.disk.Lstat(p) }

const screen = "see /etc/hosts here\r\nand src/main.rs:12 too\nhttps://x.y/a"

func newTest(t *testing.T, mode Mode) (*Model, *fakeBackend) {
	t.Helper()
	disk := fakefs.New("/etc/hosts", "/home/u/proj/src/main.rs", "/home/u/proj/b/src/main.rs")
	r := resolve.New(resolve.Env{
		Bases: []resolve.Base{{Dir: "/home/u/proj", Source: resolve.SourceProcess}},
		Home:  "/home/u", Lstat: disk.Lstat, Readlink: disk.Readlink, ReadDir: disk.ReadDir,
	})
	ms := detect.ScanWidth(screen, r, 19)
	if len(ms) != 3 {
		t.Fatalf("fixture scan found %d matches: %+v", len(ms), ms)
	}
	b := &fakeBackend{disk: disk, refined: ms}
	m := New(Config{Mode: mode, Screen: screen, Matches: ms, Backend: b, Home: "/home/u"})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	return m, b
}

func press(m *Model, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		default:
			r := []rune(k)[0]
			msg = tea.KeyPressMsg{Code: r, Text: k}
		}
		_, cmd = m.Update(msg)
	}
	return cmd
}

// drain runs cmd and feeds every message it yields back into m, the way the
// Bubble Tea runtime would, until nothing is left.
func drain(m *Model, cmd tea.Cmd) {
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return
		}
		_, cmd = m.Update(msg)
	}
}

func TestHintsSelectByLabel(t *testing.T) {
	m, _ := newTest(t, Hints)
	// Labels go bottom-up: the URL is "a", src/main.rs "s", /etc/hosts "d".
	if diff := cmp.Diff([]string{"d", "s", "a"}, m.labels); diff != "" {
		t.Fatalf("labels (-want +got):\n%s", diff)
	}
	press(m, "x") // not a label: ignored
	if m.typed != "" || m.mode != Hints {
		t.Fatalf("stray key changed state: typed=%q mode=%v", m.typed, m.mode)
	}
	press(m, "s")
	if m.mode != menu || m.matches[m.sel].Text != "src/main.rs" {
		t.Fatalf("want the menu for src/main.rs, got mode=%v sel=%d", m.mode, m.sel)
	}
	if m.actions[0].Label != "edit at line 12" {
		t.Errorf("first action %+v, want edit at line 12", m.actions[0])
	}
	press(m, "esc")
	if m.mode != Hints || m.typed != "" {
		t.Errorf("esc should return to hints with nothing typed")
	}
}

func TestHintsEnterPicksNewest(t *testing.T) {
	m, _ := newTest(t, Hints)
	press(m, "enter")
	if m.mode != menu || m.matches[m.sel].Kind != detect.KindURL {
		t.Errorf("enter should open the bottom match, got %+v", m.matches[m.sel])
	}
}

func TestHintsView(t *testing.T) {
	m, _ := newTest(t, Hints)
	got := strings.Split(ansi.Strip(m.View().Content), "\n")
	// src/main.rs's label "s" happens to land on its own first letter.
	want := []string{"see detc/hosts here", "and src/main.rs:12 too", "attps://x.y/a"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("labels should overwrite each target's first cell (-want +got):\n%s", diff)
	}
}

func TestRefineBeforeAndAfterInput(t *testing.T) {
	m, b := newTest(t, Hints)
	fresh := append([]detect.Match(nil), b.refined...)
	fresh[0].Source = resolve.SourceScreen
	fresh = append(fresh, detect.Match{Kind: detect.KindSHA, Start: 0, End: 3, Text: "see"})
	m.Update(refinedMsg{fresh})
	if len(m.matches) != 4 {
		t.Fatalf("before input a rescan replaces matches, got %d", len(m.matches))
	}

	m, b = newTest(t, Hints)
	press(m, "s")
	press(m, "esc")
	press(m, "d") // opens /etc/hosts: input has happened
	fresh = append([]detect.Match(nil), b.refined...)
	fresh[0].Source = resolve.SourceScreen
	fresh = append(fresh, detect.Match{Kind: detect.KindSHA, Start: 0, End: 3, Text: "see"})
	m.Update(refinedMsg{fresh})
	if len(m.matches) != 3 || m.matches[0].Source != resolve.SourceScreen {
		t.Errorf("after input only resolutions update: %d matches, source %v", len(m.matches), m.matches[0].Source)
	}
}

func TestStreamedResultsReachOpenMenu(t *testing.T) {
	m, b := newTest(t, Hints)
	press(m, "s")
	b.index = []lookup.Result{{ID: 1, Path: "/home/u/proj/b/src/main.rs", Source: resolve.SourceIndex}}
	_, cmd := m.Update(refinedMsg{m.matches})
	drain(m, cmd)
	if n := len(m.matches[1].Alternatives); n != 1 {
		t.Fatalf("want the index hit as an alternative, got %d", n)
	}
	if m.streams != 0 {
		t.Errorf("stream count %d after the stream closed", m.streams)
	}
	press(m, "tab")
	if m.matches[1].Path != "/home/u/proj/b/src/main.rs" || m.matches[1].Alternatives[0].Path != "/home/u/proj/src/main.rs" {
		t.Errorf("tab should rotate to the alternative: %+v", m.matches[1].Resolution)
	}
	if !strings.Contains(m.actions[0].Arg, "/b/src/main.rs") {
		t.Errorf("actions should follow the new best reading: %+v", m.actions[0])
	}
}

func TestActions(t *testing.T) {
	m, b := newTest(t, Hints)
	press(m, "d", "o")
	if len(b.launched) != 1 || b.launched[0].Arg != "/etc/hosts" {
		t.Errorf("o should launch an opener for /etc/hosts: %+v", b.launched)
	}
	if m.ctx.Err() == nil {
		t.Error("launching should close the picker")
	}

	m, _ = newTest(t, Hints)
	if cmd := press(m, "d", "y"); cmd == nil || m.ctx.Err() == nil {
		t.Error("copy should return a command and close the picker")
	}

	m, b = newTest(t, Hints)
	b.deep = []lookup.Result{{ID: 0, Path: "/mnt/x/etc/hosts", Source: resolve.SourceSearch}}
	drain(m, press(m, "d", "/"))
	if n := len(m.matches[0].Alternatives); n != 1 || m.status != "search finished" {
		t.Errorf("search should stream into alternatives: %d alternatives, status %q", n, m.status)
	}
}

func TestLaunchFailureKeepsPickerOpen(t *testing.T) {
	m, _ := newTest(t, Hints)
	m.backend = failingLaunch{m.backend}
	press(m, "d", "o")
	if m.ctx.Err() != nil || m.status != "no opener" {
		t.Errorf("a failed launch should report and stay open: status %q", m.status)
	}
}

type failingLaunch struct{ Backend }

func (failingLaunch) Launch(action.Action) error { return errors.New("no opener") }

func TestList(t *testing.T) {
	m, _ := newTest(t, List)
	m.matches = append(m.matches, m.matches[0]) // /etc/hosts twice
	m.refreshItems()
	if len(m.items) != 3 {
		t.Fatalf("repeats should collapse: %d items", len(m.items))
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "×2") {
		t.Error("a repeated target should show its count")
	}
	press(m, "m", "a", "i", "n")
	if len(m.items) != 1 || m.matches[m.items[0]].Text != "src/main.rs" {
		t.Fatalf("fuzzy filter should leave src/main.rs: %v", m.items)
	}
	press(m, "enter")
	if m.mode != menu || m.origin != List {
		t.Error("enter should open the menu over the list")
	}
	press(m, "esc", "esc")
	if m.filter != "" || m.mode != List {
		t.Errorf("first esc closes the menu, second clears the filter: filter=%q", m.filter)
	}
}

func TestMenuView(t *testing.T) {
	m, _ := newTest(t, Hints)
	press(m, "s")
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"src/main.rs:12", "~/proj/src/main.rs", "via process", "edit at line 12", "copy relative path"} {
		if !strings.Contains(view, want) {
			t.Errorf("menu missing %q:\n%s", want, view)
		}
	}
}
