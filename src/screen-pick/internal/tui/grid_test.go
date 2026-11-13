package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/google/go-cmp/cmp"
)

func rowsOf(g *grid) []string {
	out := make([]string, len(g.rows))
	for i := range g.rows {
		out[i] = g.line(i)
	}
	return out
}

// kitty ends every row of @screen text with a carriage return when wrap
// markers are on, and puts a newline only between rows that are not one
// wrapped line; there is no newline after the last row.
func TestGridRowsFollowTheScreenRows(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"plain", "a\nb\nc", []string{"a", "b", "c"}},
		{"trailing newline", "a\nb\n", []string{"a", "b"}},
		{"trailing crlf", "a\r\nb\r\n", []string{"a", "b"}},
		{"empty", "", []string{""}},
		{"one newline", "\n", []string{""}},
		{"blank rows kept", "a\n\n\nb", []string{"a", "", "", "b"}},
		{"marker format", "one\r\ntwo\r\nlast\r", []string{"one", "two", "last"}},
		{"soft wrap", "AAAA\rBBB\r\nccc\r", []string{"AAAA", "BBB", "ccc"}},
		{"blank last row", "a\r\n\r", []string{"a", ""}},
		{"wide wrapped", "日本\r語\r\nx\r", []string{"日本", "語", "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, rowsOf(newGrid(tc.text))); diff != "" {
				t.Errorf("rows (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGridPositionAcrossSoftWrap(t *testing.T) {
	g := newGrid("AAAA\rBBB\r\nccc\r")
	for _, tc := range []struct{ off, row, col int }{
		{0, 0, 0}, {3, 0, 3}, {5, 1, 0}, {7, 1, 2}, {10, 2, 0}, {12, 2, 2},
	} {
		if r, c := g.position(tc.off); r != tc.row || c != tc.col {
			t.Errorf("position(%d) = (%d,%d), want (%d,%d)", tc.off, r, c, tc.row, tc.col)
		}
	}
	want := []piece{{0, 2, 4}, {1, 5, 8}}
	if diff := cmp.Diff(want, g.pieces(2, 8), cmp.AllowUnexported(piece{})); diff != "" {
		t.Errorf("a match across a soft wrap splits per row (-want +got):\n%s", diff)
	}
	g = newGrid("ab\r\ncd日本ef\nlast")
	for _, tc := range []struct{ off, row, col int }{
		{4, 1, 0}, {6, 1, 2}, {12, 1, 6},
	} {
		if r, c := g.position(tc.off); r != tc.row || c != tc.col {
			t.Errorf("position(%d) = (%d,%d), want (%d,%d)", tc.off, r, c, tc.row, tc.col)
		}
	}
}

// A newline after the last row must not add a blank row that pushes the top
// row out of a window exactly as tall as the screen.
func TestTrailingNewlineKeepsTopRowInHints(t *testing.T) {
	for _, text := range []string{
		"/etc/hosts here\nsrc/main.rs:12\nhttps://x.y/a",
		"/etc/hosts here\nsrc/main.rs:12\nhttps://x.y/a\n",
		"/etc/hosts here\r\nsrc/main.rs:12\r\nhttps://x.y/a\r",
	} {
		m := hintsModel(t, text, 40, 3)
		got := strings.Split(ansi.Strip(m.View().Content), "\n")
		if len(got) != 3 || !strings.Contains(got[0], "etc/hosts here") || !strings.Contains(got[2], "x.y/a") {
			t.Errorf("text %q: rows misaligned:\n%s", text, strings.Join(got, "\n"))
		}
	}
}

// A soft-wrapped line is drawn as the rows it took on screen, with no raw
// carriage return left in the frame.
func TestSoftWrapDrawsAsRows(t *testing.T) {
	m := hintsModel(t, "path /etc/ho\rsts end\r\nsecond\r", 40, 5)
	view := m.View().Content
	if strings.ContainsRune(view, '\r') {
		t.Errorf("a carriage return reached the frame: %q", view)
	}
	if rows := strings.Split(ansi.Strip(view), "\n"); len(rows) < 3 {
		t.Errorf("want the wrapped line as two rows plus the next, got %q", rows)
	}
}

func hintsModel(t testing.TB, text string, w, h int) *Model {
	t.Helper()
	m, _ := fuzzModel(t, text, w, h, Hints)
	return m
}
