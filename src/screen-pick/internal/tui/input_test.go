package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
	"screenpick/internal/resolve"
)

func TestTrimLastRemovesAWholeCharacter(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"ab", "a"},
		{"aé", "a"},
		{"a日", "a"},
		{"a👨‍👩‍👧", "a"},
		{"a🇯🇵", "a"},
	} {
		if got := trimLast(tc.in); got != tc.want {
			t.Errorf("trimLast(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPasteGoesToTheFilter(t *testing.T) {
	m, _ := newTest(t, List)
	m.Update(tea.PasteMsg{Content: "src/main\n.rs"})
	if m.filter != "src/main.rs" || len(m.items) != 1 {
		t.Errorf("a paste is typed text without its newlines: filter %q, %d items", m.filter, len(m.items))
	}

	m, b := newTest(t, Hints)
	b.deep = nil
	press(m, "s")
	m.Update(tea.PasteMsg{Content: "x"})
	if m.mode != menu {
		t.Error("a paste in the menu is ignored")
	}
}

// manyMatches is a screen of n distinct paths, one per row.
func manyMatches(t testing.TB, n int, mode Mode, w, h int) *Model {
	var sb strings.Builder
	for i := range n {
		fmt.Fprintf(&sb, "internal/pkg%d/file%d.go:%d\n", i, i, i+1)
	}
	sc := sb.String()
	ms := detect.ScanWidth(sc, resolve.New(resolve.NoDisk("/home/u")), 80)
	if len(ms) != n {
		t.Fatalf("scan found %d of %d", len(ms), n)
	}
	m := New(Config{Mode: mode, Screen: sc, Matches: ms, Backend: &fakeBackend{disk: fakefs.New()}, Home: "/home/u"})
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

// Every match gets a label that can be typed, however many there are, and
// typing it opens that match.
func TestEveryMatchIsReachableByItsLabel(t *testing.T) {
	m := manyMatches(t, 100, Hints, 100, 100)
	for i, l := range m.labels {
		if strings.ContainsAny(l, "q/") {
			t.Fatalf("label %q uses a key that does something else", l)
		}
		mm := manyMatches(t, 100, Hints, 100, 100)
		for _, r := range l {
			press(mm, string(r))
		}
		if mm.mode != menu || mm.sel != i {
			t.Fatalf("typing %q opened match %d (mode %v), want %d", l, mm.sel, mm.mode, i)
		}
		if i > 40 {
			break
		}
	}
}

// The selected row stays on screen however the cursor, filter and window move.
func TestListCursorStaysInView(t *testing.T) {
	for _, w := range []int{50} {
		m := manyMatches(t, 300, List, w, 12)
		check := func(what string) {
			t.Helper()
			if len(m.items) == 0 {
				return
			}
			want := m.matches[m.items[m.cursor]].Text
			if !strings.Contains(ansi.Strip(m.View().Content), strings.TrimSuffix(want, ":1")) {
				t.Fatalf("w=%d %s: cursor row %d (%s) is not on screen:\n%s", w, what, m.cursor, want, ansi.Strip(m.View().Content))
			}
		}
		for i := range 25 {
			press(m, "down")
			check(fmt.Sprint("down ", i))
		}
		sizes(m, w, 5)
		check("shrink")
		sizes(m, w, 40)
		check("grow")
		press(m, "enter", "esc")
		check("menu round trip")
		press(m, "1", "2")
		check("filter")
	}
}

// Redrawing the list is linear in the matches: a full screen of them must not
// make each keypress slow.
func TestListDrawsFastWithManyMatches(t *testing.T) {
	m := manyMatches(t, 3000, List, 140, 50)
	start := time.Now()
	for range 3 {
		m.View()
	}
	if per := time.Since(start) / 3; per > 150*time.Millisecond {
		t.Errorf("one frame of a 3000 match list took %v", per)
	}
}

// ctrl+c and ctrl+q quit from every state, and esc steps out of any of them
// within a few presses.
func TestQuitFromEveryState(t *testing.T) {
	states := map[string]func(t *testing.T) *Model{
		"hints":       func(t *testing.T) *Model { m, _ := newTest(t, Hints); return m },
		"hints typed": func(t *testing.T) *Model { m, _ := newTest(t, Hints); m.typed = "a"; return m },
		"menu":        func(t *testing.T) *Model { m, _ := newTest(t, Hints); press(m, "s"); return m },
		"list":        func(t *testing.T) *Model { m, _ := newTest(t, List); return m },
		"list filter": func(t *testing.T) *Model { m, _ := newTest(t, List); press(m, "x", "y"); return m },
		"list menu":   func(t *testing.T) *Model { m, _ := newTest(t, List); press(m, "enter"); return m },
		"alts": func(t *testing.T) *Model {
			m, _ := newTest(t, Hints)
			press(m, "s")
			m.matches[m.sel].Alternatives = []resolve.Candidate{{Path: "/x/main.rs"}}
			press(m, "tab")
			if m.mode != alts {
				t.Fatal("fixture should reach the copies list")
			}
			return m
		},
	}
	for name, mk := range states {
		for _, k := range []tea.KeyPressMsg{{Code: 'c', Mod: tea.ModCtrl}, {Code: 'q', Mod: tea.ModCtrl}} {
			m := mk(t)
			if _, cmd := m.Update(k); m.ctx.Err() == nil || cmd == nil {
				t.Errorf("%s: %v did not quit", name, k)
			}
		}
		m := mk(t)
		for range 5 {
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		}
		if m.ctx.Err() == nil {
			t.Errorf("%s: five escapes did not get out", name)
		}
	}
}
