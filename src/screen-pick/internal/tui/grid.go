package tui

import (
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

func newGrid(text string) *grid {
	g := &grid{text: text}
	start := 0
	for i := 0; i <= len(text); i++ {
		if i < len(text) && text[i] != '\n' {
			continue
		}
		end := i
		for end > start && (text[end-1] == '\r' || text[end-1] == 0) {
			end--
		}
		g.rows = append(g.rows, row{start, end})
		start = i + 1
	}
	return g
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
	for i, rw := range g.rows {
		a, b := max(start, rw.start), min(end, rw.end)
		if a < b {
			out = append(out, piece{i, a, b})
		}
	}
	return out
}

// padRight pads s with spaces to n cells.
func padRight(s string, n int) string {
	if w := ansi.StringWidth(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}
