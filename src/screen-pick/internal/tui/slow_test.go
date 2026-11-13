package tui

import (
	"context"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"screenpick/internal/action"
	"screenpick/internal/detect"
	"screenpick/internal/lookup"
	"screenpick/internal/preview"
)

// gated is a backend whose disk reads all block until release, the way they
// would on a dead network mount, and which notices being cancelled.
type gated struct {
	*fakeBackend
	gate      chan struct{}
	once      sync.Once
	cancelled atomic.Int32
	started   atomic.Int32
	indexWait bool
}

func newGated(t *testing.T, f *fakeBackend) *gated {
	t.Helper()
	g := &gated{fakeBackend: f, gate: make(chan struct{})}
	t.Cleanup(g.release)
	return g
}

func (g *gated) release() { g.once.Do(func() { close(g.gate) }) }

// wait blocks for the gate, or for ctx; it reports whether the gate opened.
func (g *gated) wait(ctx context.Context) bool {
	g.started.Add(1)
	select {
	case <-g.gate:
		return true
	case <-ctx.Done():
		g.cancelled.Add(1)
		return false
	}
}

func (g *gated) Actions(ctx context.Context, m detect.Match) []action.Action {
	g.wait(ctx)
	return g.fakeBackend.Actions(ctx, m)
}

func (g *gated) Preview(ctx context.Context, m detect.Match, rows int) preview.Preview {
	if !g.wait(ctx) {
		return preview.Preview{}
	}
	return g.fakeBackend.Preview(ctx, m, rows)
}

func (g *gated) Stat(p string) (fs.FileInfo, error) {
	g.started.Add(1)
	<-g.gate
	return g.fakeBackend.Stat(p)
}

func (g *gated) Index(ctx context.Context, ms []detect.Match) <-chan lookup.Result {
	if g.indexWait {
		g.wait(ctx)
	}
	return g.fakeBackend.Index(ctx, ms)
}

// sim runs a model the way Bubble Tea does: commands in their own goroutines,
// their messages delivered back one at a time.
type sim struct {
	t    *testing.T
	m    *Model
	msgs chan tea.Msg
	quit atomic.Bool
}

func newSim(t *testing.T, m *Model) *sim {
	return &sim{t: t, m: m, msgs: make(chan tea.Msg, 256)}
}

func (s *sim) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		switch msg := cmd().(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range msg {
				s.run(c)
			}
		case tea.QuitMsg:
			s.quit.Store(true)
		default:
			s.msgs <- msg
		}
	}()
}

func (s *sim) send(msg tea.Msg) {
	_, cmd := s.m.Update(msg)
	s.run(cmd)
}

func (s *sim) key(k string) {
	s.t.Helper()
	switch k {
	case "esc":
		s.send(tea.KeyPressMsg{Code: tea.KeyEscape})
	case "enter":
		s.send(tea.KeyPressMsg{Code: tea.KeyEnter})
	case "down":
		s.send(tea.KeyPressMsg{Code: tea.KeyDown})
	case "ctrl+c":
		s.send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	default:
		s.send(tea.KeyPressMsg{Code: []rune(k)[0], Text: k})
	}
}

// until delivers messages until cond holds.
func (s *sim) until(what string, cond func() bool) {
	s.t.Helper()
	timeout := time.After(3 * time.Second)
	for !cond() {
		select {
		case msg := <-s.msgs:
			s.send(msg)
		case <-time.After(2 * time.Millisecond):
		case <-timeout:
			s.t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// within fails the test when f takes longer than d.
func within(t *testing.T, d time.Duration, what string, f func()) {
	t.Helper()
	start := time.Now()
	f()
	took := time.Since(start)
	t.Logf("%s took %v", what, took)
	if took > d {
		t.Errorf("%s took %v, want under %v", what, took, d)
	}
}

func newSlow(t *testing.T, mode Mode, w, h int) (*sim, *gated) {
	t.Helper()
	m, f := newTest(t, mode)
	g := newGated(t, f)
	m.backend = g
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return newSim(t, m), g
}

// Opening a menu on a file under a mount that is not answering used to hold
// the loop for as long as the mount took, here for as long as the gate is
// shut. Input must stay live and the menu must fill in once the mount answers.
func TestOpenMenuOnASlowFileKeepsInputLive(t *testing.T) {
	s, g := newSlow(t, Hints, 120, 30)
	within(t, 100*time.Millisecond, "opening the menu", func() { s.key("s") })
	var view string
	within(t, 100*time.Millisecond, "drawing it", func() { view = ansi.Strip(s.m.View().Content) })
	for _, want := range []string{"src/main.rs:12", "loading actions", "reading…"} {
		if !strings.Contains(view, want) {
			t.Errorf("placeholder frame missing %q:\n%s", want, view)
		}
	}

	g.release()
	s.until("actions and preview", func() bool {
		_, st := s.m.stats[s.m.target().Path]
		return s.m.actReady && st && len(s.m.previews) == 1
	})
	view = ansi.Strip(s.m.View().Content)
	for _, want := range []string{"edit at line 12", "preview of /home/u/proj/src/main.rs", "via process"} {
		if !strings.Contains(view, want) {
			t.Errorf("loaded frame missing %q:\n%s", want, view)
		}
	}
}

func TestEscAndQuitWorkWhileLoading(t *testing.T) {
	s, _ := newSlow(t, Hints, 120, 30)
	s.key("s")
	within(t, 250*time.Millisecond, "esc", func() { s.key("esc") })
	if s.m.mode != Hints {
		t.Fatalf("esc should leave the menu while it is loading, mode %v", s.m.mode)
	}
	s.key("s")
	within(t, 250*time.Millisecond, "q", func() { s.key("q") })
	if s.m.ctx.Err() == nil {
		t.Error("q must quit while the menu is still loading")
	}
}

// Quitting cancels every read that is out, so none outlives the picker.
func TestQuitCancelsPendingLoads(t *testing.T) {
	s, g := newSlow(t, Hints, 120, 30)
	s.key("s")
	s.until("loads started", func() bool { return g.started.Load() >= 3 })
	s.key("ctrl+c")
	s.until("loads cancelled", func() bool { return g.cancelled.Load() >= 2 })
}

// Moving off a target cancels its reads, and what they return is not shown for
// the next target.
func TestStaleLoadsAreDropped(t *testing.T) {
	s, g := newSlow(t, Hints, 120, 30)
	s.key("s")
	s.until("first loads started", func() bool { return g.started.Load() >= 3 })
	first := s.m.target()
	s.key("esc")
	s.key("d")
	s.until("first loads cancelled", func() bool { return g.cancelled.Load() >= 2 })
	if first.Path == s.m.target().Path {
		t.Fatal("fixture should pick two different files")
	}

	g.release()
	s.until("second target loaded", func() bool { return s.m.actReady && len(s.m.previews) > 0 })
	if _, ok := s.m.previews[previewKey(first)]; ok {
		t.Error("a cancelled preview must not be cached")
	}
	if got := s.m.actions[0].Arg; got != s.m.target().Path {
		t.Errorf("actions are for %q, the menu is on %q", got, s.m.target().Path)
	}
}

// A key typed before the actions arrive runs once they do.
func TestKeyTypedAheadOfActionsRuns(t *testing.T) {
	s, g := newSlow(t, Hints, 120, 30)
	s.key("s")
	s.key("y")
	if s.m.ctx.Err() != nil {
		t.Fatal("nothing can run before the actions exist")
	}
	g.release()
	s.until("the queued copy", func() bool { return s.m.ctx.Err() != nil })
}

// Walking down the list keeps one read out, not one per row passed.
func TestRapidListMovesCancelEarlierReads(t *testing.T) {
	s, g := newSlow(t, List, 140, 30)
	s.m.setMatches(append(s.m.matches, s.m.matches...))
	for range 2 {
		s.key("down")
	}
	s.until("reads started", func() bool { return g.started.Load() >= 2 })
	if n := len(s.m.loads); n > 2 {
		t.Errorf("%d reads out for one cursor row, want at most stat+preview", n)
	}
	g.release()
	row := s.m.matches[s.m.items[s.m.cursor]]
	s.until("the cursor row's reads", func() bool {
		_, ok := s.m.previews[previewKey(row)]
		return ok && len(s.m.loads) == 0
	})
}

// A resize while a preview is out keeps the read and draws it at the new size.
func TestResizeMidPreview(t *testing.T) {
	s, g := newSlow(t, Hints, 120, 30)
	s.key("s")
	s.until("started", func() bool { return g.started.Load() >= 3 })
	for _, sz := range [][2]int{{60, 12}, {200, 50}, {20, 4}, {120, 30}} {
		s.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		checkFits(t, s.m, sz[0], sz[1])
	}
	g.release()
	s.until("loaded", func() bool { return s.m.actReady && len(s.m.previews) == 1 })
	if !strings.Contains(ansi.Strip(s.m.View().Content), "preview of") {
		t.Error("the preview read before the resize should be drawn after it")
	}
}

// Starting the index search must not wait on the disk either.
func TestRefineDoesNotWaitForTheIndex(t *testing.T) {
	s, g := newSlow(t, Hints, 120, 30)
	g.indexWait = true
	within(t, 250*time.Millisecond, "handling the rescan", func() { s.send(refinedMsg{s.m.matches}) })
	s.key("q")
	if s.m.ctx.Err() == nil {
		t.Error("q must quit while the index search is opening")
	}
}
