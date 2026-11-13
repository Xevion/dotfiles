package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// checkFits fails when the frame is taller or wider than the window, which
// makes the terminal scroll or wrap and shifts everything under it.
func checkFits(t *testing.T, m *Model, w, h int) {
	t.Helper()
	checkFitsTB(t, m, w, h, "")
}

func checkFitsTB(t testing.TB, m *Model, w, h int, ctx string) {
	t.Helper()
	view := m.View().Content
	lines := strings.Split(view, "\n")
	if len(lines) > h {
		t.Errorf("%s %dx%d: frame has %d rows (mode %v)", ctx, w, h, len(lines), m.mode)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got > w {
			t.Errorf("%s %dx%d: row %d is %d cells wide (mode %v): %q", ctx, w, h, i, got, m.mode, ansi.Strip(l))
			return
		}
	}
}
