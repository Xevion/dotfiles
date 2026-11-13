package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"screenpick/internal/resolve"
)

var (
	keyPgUp   = tea.KeyPressMsg{Code: tea.KeyPgUp}
	keyPgDown = tea.KeyPressMsg{Code: tea.KeyPgDown}
	keyHome   = tea.KeyPressMsg{Code: tea.KeyHome}
	keyEnd    = tea.KeyPressMsg{Code: tea.KeyEnd}
	keyHalfUp = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	keyHalfDn = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
)

func send(m *Model, msgs ...tea.Msg) {
	for _, msg := range msgs {
		_, cmd := m.Update(msg)
		drain(m, cmd)
	}
}

func cursorOnScreen(t *testing.T, m *Model, what string) {
	t.Helper()
	want := m.matches[m.items[m.cursor]].Text
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, strings.TrimSuffix(want, ":1")) {
		t.Fatalf("%s: cursor row %d (%s) is not on screen:\n%s", what, m.cursor, want, view)
	}
}

func TestListPagingMovesByAScreen(t *testing.T) {
	for _, w := range []int{50, 130} {
		m := manyMatches(t, 300, List, w, 12)
		m.View()
		page := 12 - 2
		send(m, keyPgDown)
		if m.cursor != page {
			t.Fatalf("w=%d: pgdown moved to %d, want %d", w, m.cursor, page)
		}
		cursorOnScreen(t, m, "pgdown")
		send(m, keyHalfDn)
		if m.cursor != page+page/2 {
			t.Errorf("w=%d: ctrl+d moved to %d, want %d", w, m.cursor, page+page/2)
		}
		cursorOnScreen(t, m, "ctrl+d")
		send(m, keyHalfUp, keyPgUp)
		if m.cursor != 0 {
			t.Errorf("w=%d: back up by half and full pages should land on 0, got %d", w, m.cursor)
		}
		send(m, keyPgUp)
		if m.cursor != 0 {
			t.Errorf("w=%d: pgup at the top moved to %d", w, m.cursor)
		}
		send(m, keyEnd)
		if m.cursor != len(m.items)-1 {
			t.Fatalf("w=%d: end should reach the last row, got %d of %d", w, m.cursor, len(m.items))
		}
		cursorOnScreen(t, m, "end")
		send(m, keyPgDown, keyHalfDn)
		if m.cursor != len(m.items)-1 {
			t.Errorf("w=%d: paging past the end moved to %d", w, m.cursor)
		}
		send(m, keyHome)
		m.View()
		if m.cursor != 0 || m.top != 0 {
			t.Errorf("w=%d: home should show the first row: cursor %d top %d", w, m.cursor, m.top)
		}
	}
}

// The selected row stays visible however the paging keys, filter and window
// size are mixed.
func TestListPagingKeepsCursorInView(t *testing.T) {
	m := manyMatches(t, 500, List, 60, 9)
	steps := []tea.Msg{keyPgDown, keyPgDown, keyHalfDn, keyEnd, keyPgUp, keyHalfUp, keyHome, keyEnd, keyHome}
	for i, s := range steps {
		send(m, s)
		cursorOnScreen(t, m, fmt.Sprint("step ", i))
		if i == 4 {
			send(m, tea.WindowSizeMsg{Width: 60, Height: 4})
			cursorOnScreen(t, m, "shrink")
		}
	}
	send(m, tea.KeyPressMsg{Code: '4', Text: "4"}, keyEnd)
	cursorOnScreen(t, m, "filtered end")
}

func TestPagingInAnEmptyFilteredList(t *testing.T) {
	m := manyMatches(t, 20, List, 60, 9)
	send(m, tea.KeyPressMsg{Code: 'z', Text: "z"}, tea.KeyPressMsg{Code: 'z', Text: "z"}, tea.KeyPressMsg{Code: 'z', Text: "z"})
	for _, k := range []tea.KeyPressMsg{keyPgDown, keyPgUp, keyHome, keyEnd, keyHalfDn, keyHalfUp} {
		send(m, k)
		if m.cursor != 0 || len(m.items) != 0 {
			t.Fatalf("%v: cursor %d over %d items", k, m.cursor, len(m.items))
		}
	}
	m.View()
}

func TestCopiesPaging(t *testing.T) {
	m, _ := newTest(t, Hints)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	press(m, "s")
	for i := range 40 {
		m.matches[m.sel].Alternatives = append(m.matches[m.sel].Alternatives, resolve.Candidate{Path: fmt.Sprintf("/mnt/c%02d/src/main.rs", i)})
	}
	press(m, "tab")
	m.View()
	send(m, keyPgDown)
	if m.altCursor <= 1 {
		t.Errorf("pgdown should move by a screen of copies, cursor %d", m.altCursor)
	}
	send(m, keyEnd)
	if m.altCursor != len(m.altItems)-1 {
		t.Errorf("end should reach the last copy, got %d of %d", m.altCursor, len(m.altItems))
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "c39") {
		t.Errorf("the last copy should be on screen:\n%s", ansi.Strip(m.View().Content))
	}
	send(m, keyHome)
	if m.altCursor != 0 {
		t.Errorf("home should return to the best copy, got %d", m.altCursor)
	}
	send(m, keyPgUp, keyHalfUp)
	if m.altCursor != 0 {
		t.Errorf("paging above the top moved to %d", m.altCursor)
	}
}
