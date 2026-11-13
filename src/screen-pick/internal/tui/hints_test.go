package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func labelled(m *Model) (n, longest int) {
	for _, l := range m.labels {
		if l != "" {
			n++
			longest = max(longest, len(l))
		}
	}
	return n, longest
}

// Only the rows the window shows can be picked, so a long scrollback's worth
// of matches must not use up the short labels.
func TestHintsLabelOnlyWhatIsVisible(t *testing.T) {
	m := manyMatches(t, 3000, Hints, 100, 40)
	n, longest := labelled(m)
	if n != 40 || longest > 2 {
		t.Fatalf("a 40 row window shows 40 matches: %d labelled, longest label %d", n, longest)
	}
	first := len(m.matches) - 40
	for i, l := range m.labels {
		if (l != "") != (i >= first) {
			t.Fatalf("match %d: label %q, but the first visible match is %d", i, l, first)
		}
	}
	checkFits(t, m, 100, 40)

	hidden := manyMatches(t, 3000, Hints, 100, 40)
	for _, r := range hidden.labels[len(hidden.labels)-1] {
		press(hidden, string(r))
	}
	if hidden.mode != menu || hidden.sel != 2999 {
		t.Errorf("typing a label opened match %d, which is not on screen", hidden.sel)
	}
}

func TestHintsEnterNeverPicksAHiddenMatch(t *testing.T) {
	m := manyMatches(t, 200, Hints, 100, 10)
	press(m, "enter")
	if m.mode != menu || m.sel != 199 {
		t.Errorf("enter should open the newest visible match, got mode %v sel %d", m.mode, m.sel)
	}
	m = manyMatches(t, 200, Hints, 100, 10)
	for _, r := range m.labels[195] {
		press(m, string(r))
	}
	if m.mode != menu || m.sel != 195 {
		t.Errorf("a visible match's label opened match %d", m.sel)
	}
}

func TestHintsLabelsFollowTheWindowUntilTouched(t *testing.T) {
	m := manyMatches(t, 500, Hints, 100, 10)
	if n, _ := labelled(m); n != 10 {
		t.Fatalf("want 10 labelled in a 10 row window, got %d", n)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if n, longest := labelled(m); n != 30 || longest > 2 {
		t.Fatalf("growing the window should label what it reveals: %d labelled", n)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 5})
	if n, _ := labelled(m); n != 5 {
		t.Fatalf("shrinking should free the labels of rows that left: %d labelled", n)
	}

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	before := append([]string(nil), m.labels...)
	press(m, "a")
	press(m, "esc")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 35})
	if fmt.Sprint(before) != fmt.Sprint(m.labels) {
		t.Errorf("labels moved under the user after input")
	}
}

// Rows scrolled off the top never get a label, and a match that wraps across
// the window's top edge is labelled where it can be seen.
func TestHintsLabelOfAMatchCutByTheTopEdge(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("/home/u/proj/cut/by/the/edge/of/the/window/file.go\n")
	for i := range 4 {
		fmt.Fprintf(&sb, "/etc/row%d/file.go\n", i)
	}
	m, _ := fuzzModel(t, sb.String(), 80, 4, Hints)
	for i, l := range m.labels {
		first := strings.HasPrefix(m.matches[i].Text, "/home/u/proj/cut")
		if first && l != "" {
			t.Errorf("a match above the window has label %q", l)
		}
	}
	view := ansi.Strip(m.View().Content)
	if strings.Contains(view, "cut/by") {
		t.Errorf("the scrolled off row is drawn:\n%s", view)
	}
}
