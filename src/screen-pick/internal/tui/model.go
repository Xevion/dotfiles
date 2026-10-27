// Package tui is the picker: hint labels drawn over the captured screen, or a
// fuzzy list, leading to an action menu for the chosen target.
package tui

import (
	"context"
	"io/fs"
	"os/exec"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"screenpick/internal/action"
	"screenpick/internal/detect"
	"screenpick/internal/lookup"
	"screenpick/internal/resolve"
)

// Backend is everything the picker asks of the outside world, so tests can
// run it without a disk, kitty or a desktop.
type Backend interface {
	// Refine rescans with the slower, fuller environment.
	Refine(ctx context.Context) []detect.Match
	// Index streams index results for unresolved matches.
	Index(ctx context.Context, ms []detect.Match) <-chan lookup.Result
	// Deep walks the disk for one match.
	Deep(ctx context.Context, id int, m detect.Match) <-chan lookup.Result
	Actions(m detect.Match) []action.Action
	// Launch starts a detached program (Open, Tab).
	Launch(a action.Action) error
	// Command builds a foreground program (Edit, Show).
	Command(a action.Action) *exec.Cmd
	Stat(path string) (fs.FileInfo, error)
}

type Mode uint8

const (
	Hints Mode = iota
	List
	menu
)

// Config starts a picker.
type Config struct {
	Mode    Mode
	Screen  string
	Matches []detect.Match
	Backend Backend
	// Home shortens displayed paths to ~.
	Home string
}

type Model struct {
	backend Backend
	ctx     context.Context
	cancel  context.CancelFunc

	home    string
	screen  *grid
	matches []detect.Match
	labels  []string
	merger  lookup.Merger

	mode   Mode
	origin Mode // where the menu returns to

	typed  string // hint label typed so far
	filter string // list filter
	items  []int  // list rows, as match indices in display order
	cursor int

	sel     int // match the menu is for
	actions []action.Action

	streams int // result streams still open
	status  string
	width   int
	height  int
}

func New(cfg Config) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Model{
		backend: cfg.Backend,
		ctx:     ctx,
		cancel:  cancel,
		home:    cfg.Home,
		screen:  newGrid(cfg.Screen),
		mode:    cfg.Mode,
		origin:  cfg.Mode,
	}
	m.setMatches(cfg.Matches)
	return m
}

type (
	refinedMsg struct{ matches []detect.Match }
	resultMsg  struct {
		r  lookup.Result
		ch <-chan lookup.Result
	}
	streamDoneMsg struct{}
	execDoneMsg   struct{ err error }
)

func (m *Model) Init() tea.Cmd {
	return func() tea.Msg { return refinedMsg{m.backend.Refine(m.ctx)} }
}

func (m *Model) setMatches(ms []detect.Match) {
	m.matches = ms
	m.labels = assignLabels(len(ms))
	m.refreshItems()
}

// listen reads one result from ch, then is re-armed by Update.
func listen(ch <-chan lookup.Result) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return resultMsg{r, ch}
	}
}

func (m *Model) stream(ch <-chan lookup.Result) tea.Cmd {
	m.streams++
	return listen(ch)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case refinedMsg:
		m.applyRefined(msg.matches)
		return m, m.stream(m.backend.Index(m.ctx, m.matches))
	case resultMsg:
		if msg.r.Err == nil && msg.r.ID >= 0 && msg.r.ID < len(m.matches) {
			if m.merger.Apply(msg.r.ID, &m.matches[msg.r.ID].Resolution, msg.r) && m.mode == menu && msg.r.ID == m.sel {
				m.actions = m.backend.Actions(m.matches[m.sel])
			}
		}
		return m, listen(msg.ch)
	case streamDoneMsg:
		m.streams--
		if m.streams == 0 && m.mode == menu && strings.HasPrefix(m.status, "searching") {
			m.status = "search finished"
		}
	case execDoneMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		return m, m.quit()
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

// applyRefined takes a fuller rescan. Before any input the new matches simply
// replace the old; after, only resolutions of spans already shown are updated
// so labels and rows do not move under the user.
func (m *Model) applyRefined(ms []detect.Match) {
	if m.typed == "" && m.filter == "" && m.mode != menu {
		m.setMatches(ms)
		return
	}
	for i, old := range m.matches {
		if j := slices.IndexFunc(ms, func(n detect.Match) bool { return n.Start == old.Start && n.End == old.End && n.Kind == old.Kind }); j >= 0 {
			m.matches[i].Resolution = ms[j].Resolution
		}
	}
}

func (m *Model) quit() tea.Cmd {
	m.cancel()
	return tea.Quit
}

func (m *Model) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if k.String() == "ctrl+c" {
		return m, m.quit()
	}
	switch m.mode {
	case Hints:
		return m.hintKey(k)
	case List:
		return m.listKey(k)
	}
	return m.menuKey(k)
}

func (m *Model) hintKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		if m.typed != "" {
			m.typed = ""
			return m, nil
		}
		return m, m.quit()
	case "backspace":
		m.typed = trimLast(m.typed)
	case "enter":
		if i := m.firstLabelled(); i >= 0 {
			m.openMenu(i)
		}
	case "/":
		m.mode, m.origin = List, List
	default:
		if k.Text == "" || !strings.Contains(labelAlphabet, k.Text) {
			return m, nil
		}
		typed := m.typed + k.Text
		if i := slices.Index(m.labels, typed); i >= 0 {
			m.typed = typed
			m.openMenu(i)
			return m, nil
		}
		if slices.ContainsFunc(m.labels, func(l string) bool { return strings.HasPrefix(l, typed) }) {
			m.typed = typed
		}
	}
	return m, nil
}

// firstLabelled is the match whose label is shortest among those still
// matching what was typed: the newest one when nothing was.
func (m *Model) firstLabelled() int {
	best := -1
	for i, l := range m.labels {
		if strings.HasPrefix(l, m.typed) && (best < 0 || len(l) < len(m.labels[best]) || len(l) == len(m.labels[best]) && i > best) {
			best = i
		}
	}
	return best
}

func (m *Model) listKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.refreshItems()
			return m, nil
		}
		return m, m.quit()
	case "up", "ctrl+p":
		m.cursor = max(m.cursor-1, 0)
	case "down", "ctrl+n":
		m.cursor = min(m.cursor+1, max(len(m.items)-1, 0))
	case "enter":
		if m.cursor < len(m.items) {
			m.openMenu(m.items[m.cursor])
		}
	case "backspace":
		m.filter = trimLast(m.filter)
		m.refreshItems()
	default:
		if k.Text != "" {
			m.filter += k.Text
			m.refreshItems()
		}
	}
	return m, nil
}

func (m *Model) openMenu(i int) {
	m.origin = m.mode
	m.mode = menu
	m.sel = i
	m.cursor = 0
	m.status = ""
	m.actions = m.backend.Actions(m.matches[i])
}

func (m *Model) menuKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.mode, m.cursor, m.typed = m.origin, 0, ""
		return m, nil
	case "up", "ctrl+p":
		m.cursor = max(m.cursor-1, 0)
		return m, nil
	case "down", "ctrl+n":
		m.cursor = min(m.cursor+1, max(len(m.actions)-1, 0))
		return m, nil
	case "tab", "shift+tab":
		m.cycleAlternative(k.String() == "tab")
		return m, nil
	case "enter":
		if m.cursor < len(m.actions) {
			return m, m.run(m.actions[m.cursor])
		}
		return m, nil
	}
	if i := slices.IndexFunc(m.actions, func(a action.Action) bool { return a.Key == k.Text }); i >= 0 {
		return m, m.run(m.actions[i])
	}
	return m, nil
}

// cycleAlternative rotates the selected match's best reading through its
// alternatives.
func (m *Model) cycleAlternative(forward bool) {
	res := &m.matches[m.sel].Resolution
	if len(res.Alternatives) == 0 {
		return
	}
	ring := append([]resolve.Candidate{{Path: res.Path, Source: res.Source}}, res.Alternatives...)
	if forward {
		ring = append(ring[1:], ring[0])
	} else {
		ring = append(ring[len(ring)-1:], ring[:len(ring)-1]...)
	}
	res.Path, res.Source, res.Alternatives = ring[0].Path, ring[0].Source, ring[1:]
	fi, err := m.backend.Stat(res.Path)
	res.IsDir = err == nil && fi.IsDir()
	m.actions = m.backend.Actions(m.matches[m.sel])
	m.cursor = 0
}

func (m *Model) run(a action.Action) tea.Cmd {
	switch a.Verb {
	case action.Copy:
		return tea.Sequence(tea.SetClipboard(a.Arg), m.quit())
	case action.Search:
		q := m.matches[m.sel]
		m.status = "searching the disk…"
		return m.stream(m.backend.Deep(m.ctx, m.sel, q))
	case action.Open, action.Tab:
		if err := m.backend.Launch(a); err != nil {
			m.status = err.Error()
			return nil
		}
		return m.quit()
	}
	cmd := m.backend.Command(a)
	if cmd == nil {
		return nil
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return execDoneMsg{err} })
}

func trimLast(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(r[:len(r)-1])
}
