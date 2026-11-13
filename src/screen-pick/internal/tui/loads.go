package tui

import (
	"context"
	"fmt"
	"io/fs"

	tea "charm.land/bubbletea/v2"

	"screenpick/internal/detect"
)

const (
	// previewRows is how much of a file or folder one preview load reads;
	// drawing takes the part that fits.
	previewRows = 48
	// maxPreviews bounds the preview cache.
	maxPreviews = 64
	// altStatSpan is how many copies either side of the cursor get their age
	// looked up.
	altStatSpan = 32
)

type statResult struct {
	info fs.FileInfo
	err  error
}

// load is a read that is out, so that it can be cancelled once nothing shows
// its answer and told apart from a later one for the same key.
type load struct {
	cancel context.CancelFunc
	id     int
}

// previewKey identifies what a preview read is of, so it can be reused across
// layouts.
func previewKey(mt detect.Match) string {
	return fmt.Sprint("p\x00", mt.Kind, "\x00", mt.Path, "\x00", mt.Text, "\x00", mt.Line)
}

// previewable says whether the backend has anything to read for mt.
func previewable(mt detect.Match) bool {
	switch mt.Kind {
	case detect.KindSHA:
		return true
	case detect.KindPath:
		return mt.Exists && mt.Path != ""
	}
	return false
}

// focus is the target whose details are on screen: the menu's, or the list
// cursor's when the window is wide enough for a detail pane.
func (m *Model) focus() (detect.Match, bool) {
	switch m.mode {
	case menu, alts:
		if m.sel < len(m.matches) {
			return m.target(), true
		}
	case List:
		if w, _ := m.size(); w >= wideMin && m.cursor < len(m.items) {
			return m.matches[m.items[m.cursor]], true
		}
	}
	return detect.Match{}, false
}

// ensure starts the disk reads the current view needs and cancels those it no
// longer does, so a slow mount costs a placeholder and never a stalled loop.
func (m *Model) ensure() tea.Cmd {
	if m.ctx.Err() != nil {
		return nil
	}
	keep := map[string]bool{}
	var cmds []tea.Cmd
	b := m.backend

	stat := func(p string) {
		key := "s\x00" + p
		keep[key] = true
		if _, ok := m.stats[p]; ok {
			return
		}
		if _, ok := m.loads[key]; ok {
			return
		}
		cmds = append(cmds, m.begin(key, func(ctx context.Context, id int) tea.Msg {
			fi, err := b.Stat(p)
			return statMsg{key: key, path: p, id: id, canceled: ctx.Err() != nil, res: statResult{fi, err}}
		}))
	}
	prev := func(mt detect.Match) {
		key := previewKey(mt)
		keep[key] = true
		if _, ok := m.previews[key]; ok {
			return
		}
		if _, ok := m.loads[key]; ok {
			return
		}
		cmds = append(cmds, m.begin(key, func(ctx context.Context, id int) tea.Msg {
			pv := b.Preview(ctx, mt, previewRows)
			return previewMsg{key: key, id: id, canceled: ctx.Err() != nil, pv: pv}
		}))
	}

	if mt, ok := m.focus(); ok {
		if m.mode == alts {
			lo := max(m.altCursor-altStatSpan, 0)
			for _, j := range m.altItems[min(lo, len(m.altItems)):min(m.altCursor+altStatSpan, len(m.altItems))] {
				stat(m.altList[j].Path)
			}
		} else {
			if mt.Kind == detect.KindPath && mt.Exists && mt.Path != "" {
				stat(mt.Path)
			}
			if previewable(mt) {
				prev(mt)
			}
		}
	}
	for key, l := range m.loads {
		if !keep[key] {
			l.cancel()
			delete(m.loads, key)
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) begin(key string, run func(ctx context.Context, id int) tea.Msg) tea.Cmd {
	ctx, cancel := context.WithCancel(m.ctx)
	m.loadSeq++
	id := m.loadSeq
	m.loads[key] = load{cancel, id}
	return func() tea.Msg { return run(ctx, id) }
}

func (m *Model) finish(key string, id int) {
	if l, ok := m.loads[key]; ok && l.id == id {
		l.cancel()
		delete(m.loads, key)
	}
}
