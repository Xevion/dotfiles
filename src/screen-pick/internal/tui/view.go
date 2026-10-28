package tui

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
	"github.com/sahilm/fuzzy"

	"screenpick/internal/action"
	"screenpick/internal/detect"
	"screenpick/internal/filetype"
	"screenpick/internal/resolve"
)

const (
	// wideMin is the width from which the list gets a detail pane beside it
	// instead of expanding under the selected row.
	wideMin = 110
	// splitMin is the width from which a detail block puts its preview beside
	// the actions rather than under them.
	splitMin = 96
)

var (
	selBG = lipgloss.Color("237")

	styleDim     = lipgloss.NewStyle().Faint(true)
	styleLabel   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11"))
	styleTyped   = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("3"))
	stylePlain   = lipgloss.NewStyle()
	styleFile    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleFolder  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleMissing = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleURL     = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleHost    = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	styleSHA     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleKey     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	styleHeading = lipgloss.NewStyle().Faint(true).Italic(true)
	stylePanel   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

// Nerd Font glyphs.
const (
	iconFile    = "\uf15b"
	iconText    = "\uf15c"
	iconFolder  = "\uf07b"
	iconImage   = "\uf1c5"
	iconDoc     = "\uf1c1"
	iconArchive = "\uf1c6"
	iconMedia   = "\uf1c8"
	iconBinary  = "\uf471"
	iconWindows = "\uf17a"
	iconMissing = "\uf128"
	iconURL     = "\uf0ac"
	iconHost    = "\uf233"
	iconSHA     = "\uf417"
)

// icon marks what a target is and, for paths, whether it was found.
func icon(mt detect.Match) (string, lipgloss.Style) {
	switch mt.Kind {
	case detect.KindURL:
		return iconURL, styleURL
	case detect.KindHost:
		return iconHost, styleHost
	case detect.KindSHA:
		return iconSHA, styleSHA
	}
	switch {
	case !mt.Exists:
		return iconMissing, styleMissing
	case mt.IsDir:
		return iconFolder, styleFolder
	}
	switch filetype.ByName(mt.Path) {
	case filetype.Text:
		return iconText, styleFile
	case filetype.Image:
		return iconImage, styleFile
	case filetype.Document:
		return iconDoc, styleFile
	case filetype.Archive:
		return iconArchive, styleFile
	case filetype.Media:
		return iconMedia, styleFile
	case filetype.WindowsExe:
		return iconWindows, styleFile
	case filetype.Executable, filetype.Binary:
		return iconBinary, styleFile
	}
	return iconFile, styleFile
}

func legend() string {
	parts := []string{
		styleFile.Render(iconFile) + " file",
		styleFolder.Render(iconFolder) + " folder",
		styleMissing.Render(iconMissing) + " not found",
		styleURL.Render(iconURL) + " url",
		styleHost.Render(iconHost) + " host",
		styleSHA.Render(iconSHA) + " commit",
	}
	return styleDim.Render(strings.Join(parts, "  "))
}

func (m *Model) size() (w, h int) {
	w, h = m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}

func (m *Model) View() tea.View {
	w, h := m.size()
	var content string
	switch {
	case len(m.matches) == 0:
		content = styleDim.Render("No paths, URLs, hosts or commits on screen.  esc to close")
	case m.origin == List:
		content = m.listLayout(w, h)
	default:
		content = m.hintsLayout(w, h)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// footer is the key help for the current mode, with the icon legend when it
// fits.
func (m *Model) footer(w int) string {
	var keys string
	switch m.mode {
	case List:
		keys = "type to filter  ·  ↑↓ move  ·  enter: actions  ·  esc or ctrl+q: quit"
	case menu:
		keys = "key or enter: run  ·  esc: back  ·  q: quit"
		if n := len(m.matches[m.sel].Alternatives); n > 0 {
			keys = fmt.Sprintf("key or enter: run  ·  tab: %d other copies  ·  esc: back  ·  q: quit", n)
		}
	case alts:
		keys = "type to filter  ·  enter: act on this copy  ·  esc: back  ·  ctrl+q: quit"
	}
	out := styleDim.Render(keys)
	if l := legend(); ansi.StringWidth(out)+4+ansi.StringWidth(l) <= w {
		out += strings.Repeat(" ", w-ansi.StringWidth(out)-ansi.StringWidth(l)) + l
	}
	return ansi.Truncate(out, w, "…")
}

// hintsLayout draws the screen with labels, and once a target is picked, its
// menu on whichever half of the window the target is not on.
func (m *Model) hintsLayout(w, h int) string {
	lines, first := m.hintsLines(h)
	if m.mode != menu && m.mode != alts {
		return strings.Join(lines, "\n")
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	row, _ := m.screen.position(m.matches[m.sel].Start)
	row -= first
	atTop := row >= h/2
	room := h - row - 1
	if atTop {
		room = row
	}
	inner := max(w-4, 10)
	// Title, facts, gap, actions, gap, footer and the border: never cut the
	// actions to keep the target visible.
	acts, _ := m.actionLines(inner)
	need := 7 + len(acts)
	if m.status != "" {
		need += 2
	}
	room = max(room, min(h, need))

	body := m.block(m.target(), inner, room-4, true)
	body = append(body, "", m.footer(inner))
	panel := strings.Split(stylePanel.Width(w).Render(strings.Join(body, "\n")), "\n")
	if len(panel) >= h {
		return strings.Join(panel[:h], "\n")
	}
	if atTop {
		copy(lines, panel)
	} else {
		copy(lines[h-len(panel):], panel)
	}
	return strings.Join(lines, "\n")
}

// hintsLines redraws the captured screen faded, with each target coloured and
// its label laid over its first cells. first is the screen row drawn first.
func (m *Model) hintsLines(h int) (lines []string, first int) {
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
	if len(m.screen.rows) > h {
		first = len(m.screen.rows) - h
	}
	for r := first; r < len(m.screen.rows); r++ {
		var b strings.Builder
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
				_, st := icon(m.matches[mk.match])
				b.WriteString(st.Render(text))
			}
			pos = mk.end
		}
		b.WriteString(styleDim.Render(m.screen.text[pos:rw.end]))
		lines = append(lines, b.String())
	}
	return lines, first
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

// listLayout is the list with the selected target's details beside it when
// the window is wide, or expanded under its row when it is not.
func (m *Model) listLayout(w, h int) string {
	avail := h - 1
	var lines []string
	if w < wideMin {
		lines = m.listLines(w, avail, true)
	} else {
		lw := w * 11 / 20
		left := m.listLines(lw, avail, false)
		var right []string
		switch {
		case m.mode == menu || m.mode == alts:
			right = m.block(m.target(), w-lw-3, avail, true)
		case m.cursor < len(m.items):
			right = m.block(m.matches[m.items[m.cursor]], w-lw-3, avail, false)
		}
		lines = besides(left, right, lw, w-lw-3, avail)
	}
	for len(lines) < avail {
		lines = append(lines, "")
	}
	return strings.Join(append(lines, m.footer(w)), "\n")
}

// listLines is the prompt and the rows that fit in h lines, scrolled to keep
// the cursor in view. inline puts an open menu under the cursor's row.
func (m *Model) listLines(w, h int, inline bool) []string {
	prompt := styleKey.Render(">") + " " + m.filter
	info := strconv.Itoa(len(m.items)) + "/" + strconv.Itoa(len(m.matches))
	if m.streams > 0 {
		info += "  searching…"
	}
	lines := []string{prompt + "  " + styleDim.Render(info)}
	if len(m.items) == 0 {
		return append(lines, styleDim.Render("no match for "+strconv.Quote(m.filter)))
	}

	rows := h - 1
	var block []string
	if inline && (m.mode == menu || m.mode == alts) {
		for _, l := range m.block(m.target(), w-2, min(rows-1, 20), true) {
			block = append(block, styleDim.Render("│ ")+l)
		}
		block = block[:min(len(block), rows-1)]
	}
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if need := m.cursor - m.top + 1 + len(block); need > rows {
		m.top += need - rows
	}
	m.top = max(m.top, 0)

	cols := m.columns(w)
	for i := m.top; i < len(m.items) && len(lines) <= rows; i++ {
		lines = append(lines, m.listRow(m.items[i], w, cols, i == m.cursor))
		if i == m.cursor {
			lines = append(lines, block...)
		}
	}
	return lines[:min(len(lines), h)]
}

type columns struct{ text, place, flags int }

// columns sizes the list: the shown text gets what most rows need, up to
// about half, and where it resolved to gets the rest.
func (m *Model) columns(w int) columns {
	var c columns
	textNat, placeNat := 0, 0
	for _, i := range m.items {
		mt := m.matches[i]
		textNat = max(textNat, ansi.StringWidth(mt.Text+location(mt)))
		placeNat = max(placeNat, ansi.StringWidth(m.place(mt)))
		c.flags = max(c.flags, ansi.StringWidth(m.flags(i)))
	}
	avail := w - 2
	if c.flags > 0 {
		avail -= c.flags + 2
	}
	if placeNat == 0 {
		c.text = min(textNat, avail)
		return c
	}
	c.text = min(textNat, max(avail-placeNat-2, avail*9/20, 12))
	c.place = max(avail-c.text-2, 0)
	return c
}

// place says where a target was found, when that differs from how it reads
// on screen.
func (m *Model) place(mt detect.Match) string {
	if mt.Kind != detect.KindPath {
		return ""
	}
	switch {
	case mt.Path == "":
		return "cannot be placed here"
	case !mt.Exists:
		return "not found"
	}
	p := tilde(mt.Path, m.home)
	if p == mt.Text || mt.Path == mt.Text || p == tilde(mt.Text, m.home) {
		return ""
	}
	if rel := strings.TrimPrefix(mt.Text, "./"); strings.HasSuffix(mt.Path, "/"+rel) {
		return "in " + tilde(strings.TrimSuffix(mt.Path, "/"+rel), m.home)
	}
	return p
}

func (m *Model) flags(i int) string {
	var out []string
	if n := len(m.matches[i].Alternatives); n > 0 {
		out = append(out, "+"+strconv.Itoa(n))
	}
	if c := m.count(i); c > 1 {
		out = append(out, "×"+strconv.Itoa(c))
	}
	return strings.Join(out, " ")
}

func (m *Model) listRow(i, w int, c columns, sel bool) string {
	mt := m.matches[i]
	seg := func(st lipgloss.Style, s string) string {
		if sel {
			st = st.Background(selBG)
		}
		return st.Render(s)
	}
	ic, ist := icon(mt)
	row := seg(ist, ic+" ") + seg(stylePlain, padRight(shortenMiddle(mt.Text+location(mt), c.text), c.text))
	if c.place > 0 {
		pst := styleDim
		if mt.Kind == detect.KindPath && !mt.Exists {
			pst = styleMissing.Faint(true)
		}
		row += seg(stylePlain, "  ") + seg(pst, padRight(shortenMiddle(m.place(mt), c.place), c.place))
	}
	if c.flags > 0 {
		f := m.flags(i)
		row += seg(stylePlain, "  "+strings.Repeat(" ", c.flags-ansi.StringWidth(f))) + seg(styleDim, f)
	}
	if pad := w - ansi.StringWidth(row); pad > 0 {
		row += seg(stylePlain, strings.Repeat(" ", pad))
	}
	return row
}

// block describes one target in w×h cells: what it is and where, then its
// actions when focused, then a preview in whatever room is left.
func (m *Model) block(mt detect.Match, w, h int, focused bool) []string {
	lines := m.blockLines(mt, w, h, focused)
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, w, "…")
	}
	return lines
}

func (m *Model) blockLines(mt detect.Match, w, h int, focused bool) []string {
	if m.mode == alts {
		return m.altsLines(w, h)
	}
	if h <= 0 {
		return nil
	}
	head := m.infoLines(mt, w)
	var acts []string
	if focused {
		var at int
		acts, at = m.actionLines(w)
		if m.status != "" {
			acts = append(acts, "", m.status)
		}
		acts = fitAround(acts, at, h-len(head)-1)
	}
	if focused && w >= splitMin {
		lw := min(56, w/2)
		left := append(m.infoLines(mt, lw), "")
		narrow, at := m.actionLines(lw)
		if m.status != "" {
			narrow = append(narrow, "", shortenMiddle(m.status, lw))
		}
		left = append(left, fitAround(narrow, at, h-len(left))...)
		rows := min(h, max(len(left), 8))
		right := m.previewLines(mt, w-lw-3, rows)
		if len(right) == 0 {
			return append(append(head, ""), acts...)[:min(len(head)+1+len(acts), h)]
		}
		return besides(left, right, lw, w-lw-3, min(h, max(len(left), len(right))))
	}
	out := append(head, "")
	out = append(out, acts...)
	if rows := h - len(out) - 1; rows >= 3 {
		if pv := m.previewLines(mt, w, rows); len(pv) > 0 {
			if len(acts) > 0 {
				out = append(out, "")
			}
			out = append(out, pv...)
		}
	}
	return out[:min(len(out), h)]
}

// infoLines names the target and says where it is, how big, how old and how
// it was found: the one place its full path is shown.
func (m *Model) infoLines(mt detect.Match, w int) []string {
	ic, ist := icon(mt)
	more := ""
	if n := len(m.matches[m.sel].Alternatives); n > 0 && (m.mode == menu || m.mode == alts) {
		more = fmt.Sprintf("  +%d other copies", n)
	}
	title := ist.Render(ic) + " " + styleBold.Render(shortenMiddle(mt.Text+location(mt), w-2-ansi.StringWidth(more))) + styleDim.Render(more)

	sep := styleDim.Render("  ·  ")
	var facts []string
	where := ""
	switch {
	case mt.Kind == detect.KindURL:
		facts = append(facts, "url")
	case mt.Kind == detect.KindHost:
		facts = append(facts, "host")
	case mt.Kind == detect.KindSHA:
		facts = append(facts, "commit")
	case mt.Path == "":
		facts = append(facts, styleMissing.Render("cannot be placed on this machine"))
	case !mt.Exists:
		facts = append(facts, styleMissing.Render("not found"))
		where = "looked for " + tilde(mt.Path, m.home)
	default:
		where = tilde(mt.Path, m.home)
		if fi, err := m.backend.Stat(mt.Path); err == nil {
			if fi.IsDir() {
				facts = append(facts, "folder")
			} else {
				facts = append(facts, humanize.IBytes(uint64(fi.Size())))
			}
			if !fi.ModTime().IsZero() {
				facts = append(facts, "modified "+humanize.Time(fi.ModTime()))
			}
		}
		via := "via " + mt.Source.String()
		if m.choice != nil && (m.mode == menu || m.mode == alts) {
			via = "picked copy, " + via
		}
		if mt.Expanded {
			via += ", expanded"
		}
		facts = append(facts, via)
	}
	rest := strings.Join(facts, sep)
	if where == "" {
		return []string{title, rest}
	}
	room := w - ansi.StringWidth(rest) - ansi.StringWidth(sep)
	if room < 20 {
		return []string{title, shortenMiddle(where, w), rest}
	}
	return []string{title, shortenMiddle(where, room) + sep + rest}
}

// actionLines lists the menu by group, reporting which line is selected. The
// value column appears only where it says something the header does not.
func (m *Model) actionLines(w int) (out []string, at int) {
	target := m.target().Path
	labelW := 0
	for _, a := range m.actions {
		labelW = max(labelW, ansi.StringWidth(a.Label))
	}
	group := action.Group(255)
	for i, a := range m.actions {
		if a.Group != group {
			group = a.Group
			out = append(out, styleHeading.Render(group.String()))
		}
		sel := i == m.act
		seg := func(st lipgloss.Style, s string) string {
			if sel {
				st = st.Background(selBG)
			}
			return st.Render(s)
		}
		line := seg(stylePlain, " ") + seg(styleKey, padRight(a.Key, 2)) + seg(stylePlain, padRight(a.Label, labelW))
		if v := tilde(a.Arg, m.home); (a.Verb == action.Copy || a.Verb == action.Tab) && a.Arg != target && room(w, line) > 6 {
			line += seg(styleDim, "  "+shortenMiddle(v, room(w, line)-2))
		}
		if pad := w - ansi.StringWidth(line); pad > 0 && sel {
			line += seg(stylePlain, strings.Repeat(" ", pad))
		}
		if sel {
			at = len(out)
		}
		out = append(out, line)
	}
	return out, at
}

// fitAround keeps at most n lines, scrolled so line at stays in view.
func fitAround(lines []string, at, n int) []string {
	if n <= 0 {
		return nil
	}
	if len(lines) <= n {
		return lines
	}
	start := min(max(at-n/2, 0), len(lines)-n)
	return lines[start : start+n]
}

func room(w int, s string) int { return w - ansi.StringWidth(s) }

// besides joins two columns into n lines, cutting each to its width so the
// divider stays straight.
func besides(left, right []string, lw, rw, n int) []string {
	sep := styleDim.Render(" │ ")
	out := make([]string, n)
	for i := range n {
		l, r := "", ""
		if i < len(left) {
			l = ansi.Truncate(left[i], lw, "…")
		}
		if i < len(right) {
			r = ansi.Truncate(right[i], rw, "…")
		}
		out[i] = padRight(l, lw) + sep + r
	}
	return out
}

// previewLines draws the file around its line, the folder's entries, or a
// commit's summary.
func (m *Model) previewLines(mt detect.Match, w, rows int) []string {
	if rows <= 0 || w < 10 {
		return nil
	}
	key := fmt.Sprint(mt.Kind, "\x00", mt.Path, "\x00", mt.Text, "\x00", mt.Line, "\x00", rows)
	pv, ok := m.previews[key]
	if !ok {
		pv = m.backend.Preview(mt, rows)
		m.previews[key] = pv
	}
	if pv.Note != "" {
		return []string{styleDim.Render(shortenMiddle(pv.Note, w))}
	}
	gutter := 0
	for _, l := range pv.Lines {
		gutter = max(gutter, len(strconv.Itoa(l.No)))
	}
	var out []string
	for _, l := range pv.Lines {
		switch {
		case l.No > 0:
			num := fmt.Sprintf("%*d ", gutter, l.No)
			text := ansi.Truncate(l.Text, w-len(num), "…")
			if l.Focus {
				out = append(out, styleKey.Render(num)+styleBold.Background(selBG).Render(padRight(text, w-len(num))))
			} else {
				out = append(out, styleDim.Render(num)+text)
			}
		case l.Dir:
			out = append(out, styleFolder.Render(iconFolder)+" "+ansi.Truncate(l.Text, w-2, "…"))
		case mt.Kind == detect.KindPath:
			ic, st := icon(detect.Match{Kind: detect.KindPath, Resolution: resolve.Resolution{Path: path.Join(mt.Path, l.Text), Exists: true}})
			out = append(out, st.Render(ic)+" "+ansi.Truncate(l.Text, w-2, "…"))
		default:
			out = append(out, ansi.Truncate(l.Text, w, "…"))
		}
	}
	return out
}

// altsLines lists every copy of the menu's target to pick one, nearest to the
// best reading first.
func (m *Model) altsLines(w, h int) []string {
	mt := m.matches[m.sel]
	head := styleBold.Render(shortenMiddle(mt.Text, w-24)) + styleDim.Render(fmt.Sprintf("  %d copies", len(m.altList)))
	if m.streams > 0 {
		head += styleDim.Render("  searching…")
	}
	out := []string{head, styleKey.Render(">") + " " + m.altFilter}
	rows := h - len(out)
	if rows <= 0 {
		return out[:max(h, 0)]
	}
	if len(m.altItems) == 0 {
		return append(out, styleDim.Render("no copy matches "+strconv.Quote(m.altFilter)))
	}
	start := max(0, min(m.altCursor-rows/2, len(m.altItems)-rows))
	current := ""
	if m.choice != nil {
		current = m.choice.Path
	}
	for n, j := range m.altItems[start:min(start+rows, len(m.altItems))] {
		c := m.altList[j]
		sel := start+n == m.altCursor
		seg := func(st lipgloss.Style, s string) string {
			if sel {
				st = st.Background(selBG)
			}
			return st.Render(s)
		}
		mark := "  "
		if j == 0 && current == "" || c.Path == current {
			mark = "● "
		}
		note := c.Source.String()
		if j == 0 {
			note = "best, " + note
		}
		if fi, err := m.backend.Stat(c.Path); err == nil && !fi.ModTime().IsZero() {
			note += ", " + humanize.Time(fi.ModTime())
		}
		p := tilde(c.Path, m.home)
		pw := max(w-2-ansi.StringWidth(note)-2, 10)
		p = shortenMiddle(p, pw)
		dir, base := path.Split(p)
		line := seg(styleKey, mark) + seg(styleDim, dir) + seg(stylePlain, base)
		line += seg(stylePlain, strings.Repeat(" ", max(2, w-ansi.StringWidth(line)-ansi.StringWidth(note)))) + seg(styleDim, note)
		out = append(out, line)
	}
	return out
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

// shortenMiddle fits s in w cells by cutting from the middle, keeping the
// final path component whole when it fits, since that names the thing.
func shortenMiddle(s string, w int) string {
	sw := ansi.StringWidth(s)
	switch {
	case sw <= w:
		return s
	case w <= 0:
		return ""
	case w == 1:
		return "…"
	}
	if cut := strings.LastIndexAny(s, `/\`); cut > 0 {
		tail := s[cut:]
		if tw := ansi.StringWidth(tail); tw+4 <= w {
			return ansi.Truncate(s[:cut], w-tw-1, "") + "…" + tail
		}
	}
	return "…" + ansi.TruncateLeft(s, sw-(w-1), "")
}
