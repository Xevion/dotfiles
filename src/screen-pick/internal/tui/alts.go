package tui

import (
	"cmp"
	"path"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"screenpick/internal/resolve"
)

// rankCopies orders every reading of res for picking: the best one first, then
// the rest by how close they sit to it, how they were found, and length.
func rankCopies(res resolve.Resolution) []resolve.Candidate {
	rest := slices.Clone(res.Alternatives)
	near := path.Dir(res.Path)
	slices.SortStableFunc(rest, func(a, b resolve.Candidate) int {
		return cmp.Or(
			cmp.Compare(sharedDirs(b.Path, near), sharedDirs(a.Path, near)),
			cmp.Compare(sourceOrder(a.Source), sourceOrder(b.Source)),
			cmp.Compare(len(a.Path), len(b.Path)),
			strings.Compare(a.Path, b.Path),
		)
	})
	return append([]resolve.Candidate{{Path: res.Path, Source: res.Source}}, rest...)
}

// sharedDirs counts the leading path components p shares with dir.
func sharedDirs(p, dir string) int {
	a, b := strings.Split(p, "/"), strings.Split(dir, "/")
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// sourceOrder ranks direct checks over index hits over disk walks.
func sourceOrder(s resolve.Source) int {
	switch s {
	case resolve.SourceIndex:
		return 1
	case resolve.SourceSearch:
		return 2
	}
	return 0
}

func (m *Model) openAlts() {
	m.mode = alts
	m.altFilter = ""
	m.altCursor = 0
	m.refreshAlts()
	if m.choice != nil {
		if i := slices.IndexFunc(m.altItems, func(j int) bool { return m.altList[j].Path == m.choice.Path }); i >= 0 {
			m.altCursor = i
		}
	}
}

// refreshAlts re-ranks the copies, keeping the cursor on the same one as
// results stream in.
func (m *Model) refreshAlts() {
	var at string
	if m.altCursor < len(m.altItems) {
		at = m.altList[m.altItems[m.altCursor]].Path
	}
	m.altList = rankCopies(m.matches[m.sel].Resolution)
	m.altItems = m.altItems[:0]
	if m.altFilter == "" {
		for i := range m.altList {
			m.altItems = append(m.altItems, i)
		}
	} else {
		src := make([]string, len(m.altList))
		for i, c := range m.altList {
			src[i] = c.Path
		}
		for _, f := range fuzzy.Find(m.altFilter, src) {
			m.altItems = append(m.altItems, f.Index)
		}
	}
	m.altCursor = min(m.altCursor, max(len(m.altItems)-1, 0))
	if i := slices.IndexFunc(m.altItems, func(j int) bool { return m.altList[j].Path == at }); i >= 0 && at != "" {
		m.altCursor = i
	}
}

func (m *Model) altsKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		if m.altFilter != "" {
			m.altFilter = ""
			m.refreshAlts()
			return m, nil
		}
		m.mode = menu
	case "up", "ctrl+p", "shift+tab":
		m.altCursor = max(m.altCursor-1, 0)
	case "down", "ctrl+n", "tab":
		m.altCursor = min(m.altCursor+1, max(len(m.altItems)-1, 0))
	case "enter":
		if m.altCursor < len(m.altItems) {
			m.choose(m.altItems[m.altCursor])
		}
	case "backspace":
		m.altFilter = trimLast(m.altFilter)
		m.refreshAlts()
	default:
		if k.Text != "" {
			m.altFilter += k.Text
			m.refreshAlts()
		}
	}
	return m, nil
}

// choose acts on copy i from now until the menu closes; 0 is the best
// reading, which clears any earlier pick.
func (m *Model) choose(i int) {
	m.mode = menu
	m.act = 0
	m.choice = nil
	if i > 0 {
		c := m.altList[i]
		fi, err := m.backend.Stat(c.Path)
		m.choice = &resolve.Resolution{Path: c.Path, Source: c.Source, Exists: err == nil, IsDir: err == nil && fi.IsDir()}
	}
	m.actions = m.backend.Actions(m.target())
}
