package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// grid is the captured screen split into display rows, keeping the byte
// offset of each row so match spans (offsets into the capture) can be drawn.
type grid struct {
	text string
	rows []row
}

type row struct {
	start, end int // byte range in text, excluding the newline and markers
}

// newGrid splits text into the rows the screen had. A row ends at a newline or
// a carriage return (kitty's wrap marker, which also ends the rows of a soft
// wrapped line), and a terminator at the very end opens no further row.
func newGrid(text string) *grid {
	g := &grid{text: text}
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] != '\n' && text[i] != '\r' {
			continue
		}
		g.rows = append(g.rows, row{start, trimNul(text, start, i)})
		if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(text) || len(g.rows) == 0 {
		g.rows = append(g.rows, row{start, trimNul(text, start, len(text))})
	}
	return g
}

// trimNul drops the NUL wrap markers the hints kitten leaves at a row's end.
func trimNul(text string, start, end int) int {
	for end > start && text[end-1] == 0 {
		end--
	}
	return end
}

// line returns row r's visible text.
func (g *grid) line(r int) string { return g.text[g.rows[r].start:g.rows[r].end] }

// position maps a byte offset to its row and display column.
func (g *grid) position(off int) (r, col int) {
	for i, rw := range g.rows {
		if off <= rw.end || i == len(g.rows)-1 {
			off = max(off, rw.start)
			return i, ansi.StringWidth(g.text[rw.start:min(off, rw.end)])
		}
	}
	return 0, 0
}

// piece is the part of a span that falls on one row, as byte offsets.
type piece struct {
	row        int
	start, end int
}

// pieces splits [start,end) at row boundaries.
func (g *grid) pieces(start, end int) []piece {
	var out []piece
	i := sort.Search(len(g.rows), func(i int) bool { return g.rows[i].end > start })
	for ; i < len(g.rows) && g.rows[i].start < end; i++ {
		rw := g.rows[i]
		if a, b := max(start, rw.start), min(end, rw.end); a < b {
			out = append(out, piece{i, a, b})
		}
	}
	return out
}

// visibleFrom reports whether any of [start,end) is drawn on row first or
// below.
func (g *grid) visibleFrom(first, start, end int) bool {
	ps := g.pieces(start, end)
	return len(ps) > 0 && ps[len(ps)-1].row >= first
}

// padRight pads s with spaces to n cells.
func padRight(s string, n int) string {
	if w := ansi.StringWidth(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}
