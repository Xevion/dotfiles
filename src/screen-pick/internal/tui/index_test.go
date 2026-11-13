package tui

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
	"screenpick/internal/lookup"
	"screenpick/internal/resolve"
)

const indexScreen = "open lib/util.go and lib/other.go now"

// deadMount is the picker's backend when the window's folder sits on a mount
// that does not answer: the quick rescan gives up after quick, the full one
// after full, and both report the paths as unchecked. The index is local and
// answers at once.
type deadMount struct {
	*fakeBackend
	quick, full time.Duration

	mu     sync.Mutex
	passes [][]string
	// hold, when set, delays the first index pass's one result until closed,
	// ignoring cancellation the way a result already in flight does.
	hold chan struct{}
	// reverse makes the full rescan list its matches in the other order.
	reverse bool
}

func unchecked() []detect.Match {
	ms := detect.ScanWidth(indexScreen, resolve.New(resolve.NoDisk("/home/u")), 40)
	if len(ms) != 2 {
		panic("fixture scan found the wrong matches")
	}
	for i := range ms {
		ms[i].Unknown = true
	}
	return ms
}

func nap(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

func (d *deadMount) Refine(ctx context.Context) []detect.Match {
	nap(ctx, d.full)
	ms := unchecked()
	if d.reverse {
		slices.Reverse(ms)
	}
	return ms
}

func (d *deadMount) Index(ctx context.Context, ms []detect.Match) <-chan lookup.Result {
	qs := lookup.Queries(ms)
	var texts []string
	for _, q := range qs {
		texts = append(texts, q.Text)
	}
	d.mu.Lock()
	d.passes = append(d.passes, texts)
	first := len(d.passes) == 1
	hold := d.hold
	d.mu.Unlock()
	if first && hold != nil {
		out := make(chan lookup.Result, 1)
		go func() {
			defer close(out)
			<-hold
			out <- lookup.Result{ID: 0, Path: "/home/u/proj/pkg/lib/util.go", Source: resolve.SourceIndex}
		}()
		return out
	}
	s := lookup.Searcher{
		Roots: []string{"/home/u/proj"},
		Stat:  d.disk.Lstat,
		Load:  func(context.Context, string) ([]string, error) { return []string{"pkg/lib/util.go"}, nil },
	}
	return s.Index(ctx, qs)
}

func (d *deadMount) passCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.passes)
}

func newDead(t *testing.T, quick, full time.Duration) (*sim, *deadMount) {
	t.Helper()
	b := &deadMount{fakeBackend: &fakeBackend{disk: fakefs.New("/home/u/proj/pkg/lib/util.go")}, quick: quick, full: full}
	m := New(Config{
		Mode:    Hints,
		Screen:  indexScreen,
		Matches: detect.ScanWidth(indexScreen, resolve.New(resolve.NoDisk("/home/u")), 40),
		Quick: func(ctx context.Context) []detect.Match {
			nap(ctx, quick)
			return unchecked()
		},
		Backend: b,
		Home:    "/home/u",
	})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	return newSim(t, m), b
}

// The index search used to start only after the full rescan, so a dead mount
// that held the rescan for its whole budget delayed a search that takes
// milliseconds. It starts from the quick rescan now.
func TestIndexSearchStartsAfterTheQuickRescan(t *testing.T) {
	const quick, full = 30 * time.Millisecond, 600 * time.Millisecond
	s, b := newDead(t, quick, full)
	start := time.Now()
	s.run(s.m.Init())
	s.until("the index to resolve the path", func() bool { return s.m.matches[0].Exists })
	took := time.Since(start)
	t.Logf("path resolved %v after the start (quick %v, full %v)", took.Round(time.Millisecond), quick, full)
	if took > full/2 {
		t.Errorf("resolved after %v: the index search waited for the full rescan (%v)", took, full)
	}
	if got := s.m.matches[0].Source; got != resolve.SourceIndex {
		t.Errorf("source %v, want index", got)
	}

	s.until("the full rescan and its own index pass", func() bool { return b.passCount() == 2 && s.m.streams == 0 && !s.m.unchecked })
	if !s.m.matches[0].Exists || s.m.matches[0].Path != "/home/u/proj/pkg/lib/util.go" {
		t.Errorf("the second pass should settle on the same copy: %+v", s.m.matches[0].Resolution)
	}
	if s.m.matches[1].Exists {
		t.Errorf("lib/other.go is in no index: %+v", s.m.matches[1].Resolution)
	}
	for i, texts := range b.passes {
		if want := []string{"lib/util.go", "lib/other.go"}; !slices.Equal(texts, want) {
			t.Errorf("pass %d searched %v, want %v", i, texts, want)
		}
	}
}

// The full rescan can come first, in which case the quick one never runs an
// index search: there is one pass, not two.
func TestOnlyOneIndexPassWhenTheFullRescanWins(t *testing.T) {
	s, b := newDead(t, time.Hour, 10*time.Millisecond)
	s.run(s.m.Init())
	s.until("the index to resolve the path", func() bool { return s.m.matches[0].Exists && s.m.streams == 0 })
	if n := b.passCount(); n != 1 {
		t.Errorf("%d index passes, want 1", n)
	}
	s.m.cancel()
}

// A result still in flight from the quick pass's search refers to matches the
// full rescan has since replaced, and must not land on whichever one now has
// that number.
func TestStaleIndexResultIsDropped(t *testing.T) {
	s, b := newDead(t, 10*time.Millisecond, 100*time.Millisecond)
	b.hold, b.reverse = make(chan struct{}), true
	s.run(s.m.Init())
	s.until("the second pass to resolve its match", func() bool { return b.passCount() == 2 && s.m.matches[1].Exists })
	if s.m.matches[0].Text != "lib/other.go" {
		t.Fatalf("fixture should have reversed the matches: %q first", s.m.matches[0].Text)
	}

	close(b.hold)
	s.until("both passes to end", func() bool { return s.m.streams == 0 })
	if mt := s.m.matches[0]; mt.Exists {
		t.Errorf("%s got a result meant for a match that was replaced: %+v", mt.Text, mt.Resolution)
	}
}
