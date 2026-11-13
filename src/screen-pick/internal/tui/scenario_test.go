package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"screenpick/internal/action"
	"screenpick/internal/detect"
	"screenpick/internal/resolve"
)

var (
	keyUp       = tea.KeyPressMsg{Code: tea.KeyUp}
	keyDown     = tea.KeyPressMsg{Code: tea.KeyDown}
	keyRight    = tea.KeyPressMsg{Code: tea.KeyRight}
	keyEnter    = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyEsc      = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyTab      = tea.KeyPressMsg{Code: tea.KeyTab}
	keyShiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	keyBackspc  = tea.KeyPressMsg{Code: tea.KeyBackspace}
	keyCtrlP    = tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	keyCtrlN    = tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
)

func text(s string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: []rune(s)[0], Text: s} }

type noActions struct{ Backend }

func (noActions) Actions(context.Context, detect.Match) []action.Action { return nil }

// A target with nothing to offer must still draw and take every key.
func TestMenuWithNoActions(t *testing.T) {
	for _, mode := range []Mode{Hints, List} {
		m, _ := newTest(t, mode)
		m.backend = noActions{m.backend}
		for _, sz := range [][2]int{{60, 10}, {120, 30}, {20, 3}} {
			sizes(m, sz[0], sz[1])
			press(m, "enter")
			if mode == Hints {
				press(m, "s")
			}
			if m.mode != menu || !m.actReady || len(m.actions) != 0 {
				t.Fatalf("mode %v: want an empty menu, got mode %v ready %v actions %d", mode, m.mode, m.actReady, len(m.actions))
			}
			checkFits(t, m, sz[0], sz[1])
			send(m, keyDown, keyUp, keyCtrlN, keyCtrlP, keyEnter, text("y"), keyTab)
			if m.mode != menu || m.act != 0 {
				t.Fatalf("keys moved an empty menu: mode %v act %d", m.mode, m.act)
			}
			checkFits(t, m, sz[0], sz[1])
			send(m, keyEsc)
			if m.mode != mode {
				t.Fatalf("esc should leave the empty menu, mode %v", m.mode)
			}
		}
	}
}

// A rescan that changes what the open menu points at brings the menu along, so
// its actions never act on the path it used to be.
func TestRescanUpdatesTheOpenMenu(t *testing.T) {
	m, b := newTest(t, Hints)
	press(m, "s")
	if !strings.Contains(m.actions[0].Arg, "/proj/src/main.rs") {
		t.Fatalf("fixture: first action %+v", m.actions[0])
	}
	moved := append([]detect.Match(nil), b.refined...)
	moved[1].Resolution = resolve.Resolution{Path: "/home/u/proj/b/src/main.rs", Source: resolve.SourceProcess, Exists: true}
	send(m, refinedMsg{moved})
	if got := m.actions[0].Arg; !strings.Contains(got, "/proj/b/src/main.rs") {
		t.Errorf("the menu still acts on the old path: %+v", m.actions[0])
	}
	checkFits(t, m, 60, 10)

	press(m, "tab")
	if m.mode == alts {
		t.Fatal("no copies to list")
	}
	m.matches[m.sel].Alternatives = []resolve.Candidate{{Path: "/x/main.rs"}}
	press(m, "tab")
	moved[1].Resolution.Alternatives = []resolve.Candidate{{Path: "/y/main.rs"}, {Path: "/z/main.rs"}}
	send(m, refinedMsg{moved})
	if len(m.altList) != 3 {
		t.Errorf("the copies list should follow the rescan, got %d", len(m.altList))
	}
}

// Replacing the matches under an open menu or a list cursor never leaves
// either pointing past the end.
func TestMatchesShrinkUnderTheUser(t *testing.T) {
	m, _ := newTest(t, Hints)
	press(m, "a")
	if m.mode != menu || m.sel != 2 {
		t.Fatal("fixture should open the menu for the last match")
	}
	m.setMatches(m.matches[:1])
	checkFits(t, m, 60, 10)
	m.setMatches(nil)
	checkFits(t, m, 60, 10)
	send(m, keyEsc, keyEsc)

	m, _ = newTest(t, List)
	send(m, keyDown, keyDown)
	m.setMatches(m.matches[:1])
	if m.cursor != 0 {
		t.Errorf("cursor %d beyond a one row list", m.cursor)
	}
	checkFits(t, m, 60, 10)
	send(m, keyEnter, keyDown, keyEsc)
	checkFits(t, m, 60, 10)
}

// Typing narrows the list to its best matches first, so the cursor starts over
// at the top instead of staying on whichever row it was.
func TestTypingAFilterReturnsTheCursorToTheTop(t *testing.T) {
	m := manyMatches(t, 50, List, 80, 20)
	send(m, keyEnd)
	send(m, text("f"), text("i"))
	if m.cursor != 0 {
		t.Errorf("cursor %d after typing, want the best match at 0", m.cursor)
	}
	send(m, keyEnd, keyBackspc)
	if m.cursor != 0 {
		t.Errorf("cursor %d after backspace", m.cursor)
	}
	send(m, keyEnd, keyEsc)
	if m.filter != "" || m.cursor != 0 {
		t.Errorf("clearing the filter: %q, cursor %d", m.filter, m.cursor)
	}
	send(m, keyEnd, tea.PasteMsg{Content: "pkg1"})
	if m.cursor != 0 {
		t.Errorf("cursor %d after a paste", m.cursor)
	}
}

// Every key the footers name does what they say, in the mode they name it.
func TestEveryDocumentedKey(t *testing.T) {
	t.Run("hints typed prefix", func(t *testing.T) {
		m := manyMatches(t, 60, Hints, 80, 60)
		var first string
		for _, l := range m.labels {
			if len(l) == 2 {
				first = l[:1]
			}
		}
		send(m, text(first))
		if m.typed != first || m.mode != Hints {
			t.Fatalf("a label prefix should be kept: typed %q mode %v", m.typed, m.mode)
		}
		send(m, keyEsc)
		if m.typed != "" || m.mode != Hints || m.ctx.Err() != nil {
			t.Errorf("esc should clear a typed prefix before it quits: typed %q", m.typed)
		}
		send(m, text(first), keyEnter)
		if m.mode != menu || !strings.HasPrefix(m.labels[m.sel], first) {
			t.Errorf("enter takes the shortest label under the typed prefix: sel %d label %q", m.sel, m.labels[m.sel])
		}
	})
	t.Run("copies", func(t *testing.T) {
		m, _ := newTest(t, Hints)
		press(m, "s")
		send(m, keyTab)
		if m.mode != menu {
			t.Fatal("tab with no other copies should do nothing")
		}
		m.matches[m.sel].Alternatives = []resolve.Candidate{{Path: "/x/a/main.rs"}, {Path: "/y/b/main.rs"}}
		send(m, keyTab)
		if m.mode != alts {
			t.Fatal("tab should list the copies")
		}
		for _, tc := range []struct {
			k    tea.KeyPressMsg
			want int
		}{{keyDown, 1}, {keyTab, 2}, {keyCtrlN, 2}, {keyUp, 1}, {keyShiftTab, 0}, {keyCtrlP, 0}} {
			send(m, tc.k)
			if m.altCursor != tc.want {
				t.Errorf("%v: copy %d, want %d", tc.k, m.altCursor, tc.want)
			}
		}
		send(m, text("y"), text("b"))
		if m.altFilter != "yb" || len(m.altItems) != 1 {
			t.Errorf("typing narrows the copies: %q, %d left", m.altFilter, len(m.altItems))
		}
		send(m, keyBackspc)
		if m.altFilter != "y" {
			t.Errorf("backspace: %q", m.altFilter)
		}
		send(m, tea.PasteMsg{Content: "/b\n"})
		if m.altFilter != "y/b" {
			t.Errorf("paste: %q", m.altFilter)
		}
		send(m, keyEsc)
		if m.altFilter != "" || m.mode != alts {
			t.Errorf("first esc clears the copies filter: %q mode %v", m.altFilter, m.mode)
		}
		send(m, keyEnter)
		if m.mode != menu || m.target().Path != "/y/b/main.rs" {
			t.Errorf("clearing the filter keeps the cursor on the same copy, and enter acts on it: mode %v target %q", m.mode, m.target().Path)
		}
		send(m, keyTab, keyEsc)
		if m.mode != menu {
			t.Errorf("esc leaves the copies for the menu, mode %v", m.mode)
		}
	})
	t.Run("filter with nothing left", func(t *testing.T) {
		m, _ := newTest(t, List)
		send(m, text("z"), text("z"), text("z"), keyEnter, keyRight)
		if m.mode != List {
			t.Errorf("enter with no rows opened %v", m.mode)
		}
		send(m, keyBackspc, keyBackspc, keyBackspc)
		if len(m.items) != 3 {
			t.Errorf("clearing the filter restores the rows, got %d", len(m.items))
		}
	})
}

func TestScreenWithNoMatchesTakesEveryKey(t *testing.T) {
	for _, mode := range []Mode{Hints, List} {
		m := New(Config{Mode: mode, Screen: "nothing here", Backend: &fakeBackend{}})
		sizes(m, 40, 5)
		for _, k := range []tea.Msg{keyEnter, text("a"), text("/"), keyDown, keyPgDown, keyEnd, keyTab, keyBackspc, tea.PasteMsg{Content: "x"}} {
			send(m, k)
			checkFits(t, m, 40, 5)
		}
		if view := ansi.Strip(m.View().Content); !strings.Contains(view, "No paths") {
			t.Errorf("mode %v: %q", mode, view)
		}
	}
}

// Every action in every kind of menu is reachable by its own key and does what
// its verb says.
func TestEveryActionKeyDispatches(t *testing.T) {
	for _, label := range []string{"s", "d", "a"} {
		probe, _ := newTest(t, Hints)
		press(probe, label)
		for _, want := range probe.actions {
			m, b := newTest(t, Hints)
			press(m, label)
			cmd := press(m, want.Key)
			name := fmt.Sprintf("%s/%s (%v)", label, want.Key, want.Verb)
			switch want.Verb {
			case action.Copy:
				if cmd == nil || m.ctx.Err() == nil {
					t.Errorf("%s: should copy and close", name)
				}
			case action.Open, action.Tab, action.Run:
				if len(b.launched) != 1 || m.ctx.Err() == nil {
					t.Errorf("%s: should launch and close: launched %d", name, len(b.launched))
				}
			case action.Edit, action.Git:
				if len(b.commands) != 1 || m.ctx.Err() != nil {
					t.Errorf("%s: should start a foreground command and stay open: commands %d", name, len(b.commands))
				}
				send(m, execDoneMsg{})
				if m.ctx.Err() == nil {
					t.Errorf("%s: a finished command should close the picker", name)
				}
			case action.Search:
				if m.status == "" {
					t.Errorf("%s: should start a search", name)
				}
			default:
				t.Errorf("%s: unknown verb", name)
			}
		}
	}
}
