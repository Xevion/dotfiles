package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
	"github.com/sahilm/fuzzy"

	"screenpick/internal/detect"
)

var (
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleLabel   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11"))
	styleTyped   = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("3"))
	styleFound   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleMissing = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleURL     = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleHost    = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	styleSHA     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleCursor  = lipgloss.NewStyle().Reverse(true)
	styleKey     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	stylePanel   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func matchStyle(m detect.Match) lipgloss.Style {
	switch m.Kind {
	case detect.KindURL:
		return styleURL
	case detect.KindHost:
		return styleHost
	case detect.KindSHA:
		return styleSHA
	}
	if m.Exists {
		return styleFound
	}
	return styleMissing
}

func badge(m detect.Match) string {
	switch m.Kind {
	case detect.KindURL:
		return styleURL.Render("↗")
	case detect.KindHost:
		return styleHost.Render("⇄")
	case detect.KindSHA:
		return styleSHA.Render("#")
	}
	if m.Exists {
		return styleFound.Render("●")
	}
	return styleMissing.Render("○")
}

func (m *Model) View() tea.View {
	var content string
	switch {
	case len(m.matches) == 0:
		content = styleDim.Render("No paths, URLs, hosts or commits on screen.  esc to close")
	case m.mode == List || m.mode == menu && m.origin == List:
		content = m.listView()
	default:
		content = m.hintsView()
	}
	if m.mode == menu {
		content = overlayBottom(content, m.menuView(), m.height)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// overlayBottom draws panel over the last rows of base, which is padded to
// height rows first so the panel sits at the bottom of the window.
func overlayBottom(base, panel string, height int) string {
	lines := strings.Split(base, "\n")
	pl := strings.Split(panel, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(pl) >= len(lines) {
		return panel
	}
	return strings.Join(append(lines[:len(lines)-len(pl)], pl...), "\n")
}

// hintsView redraws the captured screen faded, with each target coloured and
// its label laid over its first cells.
func (m *Model) hintsView() string {
	type mark struct {
		piece
		match int
		first bool
	}
	byRow := make([][]mark, len(m.screen.rows))
	for i, mt := range m.matches {
		for j, p := range m.screen.pieces(mt.Start, mt.End) {
			byRow[p.row] = append(byRow[p.row], mark{p, i, j == 0})
		}
	}

	first := 0
	if m.height > 0 && len(m.screen.rows) > m.height {
		first = len(m.screen.rows) - m.height
	}
	var b strings.Builder
	for r := first; r < len(m.screen.rows); r++ {
		if r > first {
			b.WriteByte('\n')
		}
		rw := m.screen.rows[r]
		pos := rw.start
		for _, mk := range byRow[r] {
			b.WriteString(styleDim.Render(m.screen.text[pos:mk.start]))
			text := m.screen.text[mk.start:mk.end]
			label := m.labels[mk.match]
			if !strings.HasPrefix(label, m.typed) {
				b.WriteString(styleDim.Render(text))
			} else {
				if mk.first {
					b.WriteString(styleTyped.Render(m.typed) + styleLabel.Render(label[len(m.typed):]))
					text = ansi.TruncateLeft(text, len(label), "")
				}
				b.WriteString(matchStyle(m.matches[mk.match]).Render(text))
			}
			pos = mk.end
		}
		b.WriteString(styleDim.Render(m.screen.text[pos:rw.end]))
	}
	return b.String()
}

// itemKey identifies a target across repeats on screen.
func itemKey(mt detect.Match) string {
	if mt.Kind == detect.KindPath && mt.Exists {
		return "path\x00" + mt.Path
	}
	return mt.Kind.String() + "\x00" + mt.Text
}

// refreshItems rebuilds the list: one row per distinct target, newest first,
// narrowed and ordered by the fuzzy filter when one is typed.
func (m *Model) refreshItems() {
	seen := map[string]bool{}
	var base []int
	for i := len(m.matches) - 1; i >= 0; i-- {
		if k := itemKey(m.matches[i]); !seen[k] {
			seen[k] = true
			base = append(base, i)
		}
	}
	m.items = base
	if m.filter != "" {
		src := make([]string, len(base))
		for j, i := range base {
			src[j] = m.matches[i].Text + " " + m.matches[i].Path
		}
		var ranked []int
		for _, f := range fuzzy.Find(m.filter, src) {
			ranked = append(ranked, base[f.Index])
		}
		m.items = ranked
	}
	m.cursor = min(m.cursor, max(len(m.items)-1, 0))
}

func (m *Model) count(i int) int {
	k := itemKey(m.matches[i])
	n := 0
	for _, mt := range m.matches {
		if itemKey(mt) == k {
			n++
		}
	}
	return n
}

func (m *Model) listView() string {
	var b strings.Builder
	status := ""
	if m.streams > 0 {
		status = styleDim.Render("  searching…")
	}
	fmt.Fprintf(&b, "%s %s%s\n", styleKey.Render(">"), m.filter, status)

	rows := len(m.items)
	if m.height > 2 {
		rows = min(rows, m.height-2)
	}
	start := max(0, m.cursor-rows+1)
	for n, i := range m.items[start:min(start+rows, len(m.items))] {
		mt := m.matches[i]
		line := badge(mt) + " " + mt.Text + location(mt)
		var detail []string
		if mt.Kind == detect.KindPath && mt.Path != "" && tilde(mt.Path, m.home) != mt.Text {
			detail = append(detail, tilde(mt.Path, m.home))
		}
		if n := len(mt.Alternatives); n > 0 {
			detail = append(detail, "+"+strconv.Itoa(n))
		}
		if c := m.count(i); c > 1 {
			detail = append(detail, "×"+strconv.Itoa(c))
		}
		if len(detail) > 0 {
			line += "  " + styleDim.Render(strings.Join(detail, "  "))
		}
		if start+n == m.cursor {
			line = styleCursor.Render(ansi.Strip(line))
		}
		b.WriteString(line + "\n")
	}
	if len(m.items) == 0 {
		b.WriteString(styleDim.Render("no match for " + strconv.Quote(m.filter)))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func location(mt detect.Match) string {
	switch {
	case mt.Line > 0 && mt.Col > 0:
		return fmt.Sprintf(":%d:%d", mt.Line, mt.Col)
	case mt.Line > 0:
		return ":" + strconv.Itoa(mt.Line)
	}
	return ""
}

// tilde shortens a path under home to ~/...
func tilde(p, home string) string {
	if home != "" && (p == home || strings.HasPrefix(p, home+"/")) {
		return "~" + p[len(home):]
	}
	return p
}

// details describes the selected target's resolution in one line.
func (m *Model) details(mt detect.Match) string {
	if mt.Kind != detect.KindPath {
		return mt.Kind.String()
	}
	var parts []string
	switch {
	case mt.Path == "":
		parts = append(parts, "cannot be placed on this machine")
	case !mt.Exists:
		parts = append(parts, tilde(mt.Path, m.home), styleMissing.Render("not found"))
	default:
		parts = append(parts, tilde(mt.Path, m.home))
		if fi, err := m.backend.Stat(mt.Path); err == nil {
			if fi.IsDir() {
				parts = append(parts, "directory")
			} else {
				parts = append(parts, humanize.IBytes(uint64(fi.Size())))
			}
			if !fi.ModTime().IsZero() {
				parts = append(parts, "modified "+humanize.Time(fi.ModTime()))
			}
		}
		parts = append(parts, "via "+mt.Source.String())
		if mt.Expanded {
			parts = append(parts, "expanded")
		}
	}
	if n := len(mt.Alternatives); n > 0 {
		parts = append(parts, fmt.Sprintf("+%d more (tab)", n))
	}
	return strings.Join(parts, styleDim.Render("  ·  "))
}

func (m *Model) menuView() string {
	mt := m.matches[m.sel]
	lines := []string{
		styleBold.Render(mt.Text + location(mt)),
		m.details(mt),
		"",
	}
	for i, a := range m.actions {
		line := styleKey.Render(a.Key) + "  " + padRight(a.Label, 30) + styleDim.Render(tilde(a.Arg, m.home))
		if i == m.cursor {
			line = styleCursor.Render(ansi.Strip(line))
		}
		lines = append(lines, line)
	}
	if m.status != "" {
		lines = append(lines, "", m.status)
	}
	lines = append(lines, "", styleDim.Render("key or enter: run  ·  tab: other matches  ·  esc: back"))
	style := stylePanel
	if m.width > 0 {
		style = style.Width(m.width)
	}
	return style.Render(strings.Join(lines, "\n"))
}
