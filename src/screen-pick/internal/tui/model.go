// Package tui is the picker: hint labels drawn over the captured screen, or a
// fuzzy list, leading to an action menu for the chosen target.
package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"screenpick/internal/action"
	"screenpick/internal/detect"
	"screenpick/internal/lookup"
	"screenpick/internal/preview"
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
	Actions(ctx context.Context, m detect.Match) []action.Action
	// Launch starts a detached program (Open, Tab, Run).
	Launch(a action.Action) error
	// Command builds a foreground program (Edit, Git).
	Command(a action.Action) *exec.Cmd
	Stat(path string) (fs.FileInfo, error)
	// Preview reads up to rows lines of what m points at. The picker calls
	// Actions, Stat and Preview off its event loop, so they may block on a slow
	// disk; ctx is cancelled once the answer is no longer wanted.
	Preview(ctx context.Context, m detect.Match, rows int) preview.Preview
}

type Mode uint8

const (
	Hints Mode = iota
	List
	menu
	// alts picks which copy of the menu's target to act on.
	alts
)

// Config starts a picker.
type Config struct {
	Mode    Mode
	Screen  string
	Matches []detect.Match
	// Quick, when set, means Matches came from text alone and paths are not
	// yet checked against the disk: it rescans with direct disk checks and
	// runs beside Backend.Refine.
	Quick   func(ctx context.Context) []detect.Match
	Backend Backend
	// Home shortens displayed paths to ~.
	Home string
}

type Model struct {
	backend Backend
	ctx     context.Context
	cancel  context.CancelFunc

	quick func(ctx context.Context) []detect.Match
	// unchecked holds until a rescan has looked paths up on disk, so a path
	// is not drawn as missing merely because nothing has asked yet.
	unchecked bool

	home    string
	screen  *grid
	matches []detect.Match
	labels  []string
	merger  lookup.Merger

	mode   Mode
	origin Mode // layout, and where the menu returns to

	typed  string         // hint label typed so far
	filter string         // list filter
	items  []int          // list rows, as match indices in display order
	counts map[string]int // repeats of each target, as of the last frame
	cursor int            // list row
	top    int            // first list row on screen
	// listRows and altRows are how many rows the last frame had room for in the
	// list and in the copies, which a page key moves by.
	listRows, altRows int

	sel     int // match the menu is for
	actions []action.Action
	act     int // menu row
	// actReady holds once the backend has listed the actions for the target;
	// until then the menu shows a placeholder and keeps one key typed ahead.
	actReady bool
	actSeq   int
	// actCancel stops the read of the actions that are no longer wanted.
	actCancel context.CancelFunc
	queued    *tea.KeyPressMsg
	// choice is the copy picked from the alternatives, for this menu only.
	choice *resolve.Resolution

	altList   []resolve.Candidate
	altItems  []int
	altFilter string
	altCursor int

	// stats, previews and loads belong to the slow-disk reads that run beside
	// the event loop: what has come back, and what is still out.
	stats    map[string]statResult
	previews map[string]preview.Preview
	loads    map[string]load
	loadSeq  int

	// touched holds once any key was pressed, after which labels and rows never
	// move under the user.
	touched bool

	streams int // result streams still open
	// indexGen numbers the index search in flight, which a newer rescan
	// replaces: its results are dropped because their match numbers and root
	// ranks refer to a rescan that is no longer current.
	indexGen    int
	indexCancel context.CancelFunc
	status      string
	width       int
	height      int
}

func New(cfg Config) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Model{
		backend:   cfg.Backend,
		ctx:       ctx,
		cancel:    cancel,
		home:      cfg.Home,
		quick:     cfg.Quick,
		unchecked: cfg.Quick != nil,
		screen:    newGrid(cfg.Screen),
		mode:      cfg.Mode,
		origin:    cfg.Mode,
		stats:     map[string]statResult{},
		previews:  map[string]preview.Preview{},
		loads:     map[string]load{},
	}
	m.setMatches(cfg.Matches)
	return m
}

type (
	refinedMsg struct{ matches []detect.Match }
	quickMsg   struct{ matches []detect.Match }
	resultMsg  struct {
		r   lookup.Result
		ch  <-chan lookup.Result
		gen int
	}
	streamDoneMsg struct{}
	execDoneMsg   struct {
		err    error
		stderr *tail
	}
	// actionsMsg answers loadActions. checked says the target was stat'ed, and
	// then exists and isDir are what it found.
	actionsMsg struct {
		seq             int
		actions         []action.Action
		checked, exists bool
		isDir           bool
	}
	statMsg struct {
		key, path string
		id        int
		canceled  bool
		res       statResult
	}
	previewMsg struct {
		key      string
		id       int
		canceled bool
		pv       preview.Preview
	}
)

// Init starts both rescans side by side: the quick one lands first and
// colours paths by existence, the full one follows with every base directory.
func (m *Model) Init() tea.Cmd {
	refine := func() tea.Msg { return refinedMsg{m.backend.Refine(m.ctx)} }
	if m.quick == nil {
		return refine
	}
	return tea.Batch(func() tea.Msg { return quickMsg{m.quick(m.ctx)} }, refine)
}

func (m *Model) setMatches(ms []detect.Match) {
	m.matches = ms
	m.assignHints()
	m.refreshItems()
	if (m.mode == menu || m.mode == alts) && m.sel >= len(ms) {
		m.closeMenu()
	}
}

// filterChanged redraws the rows for the typed filter, whose best match is
// first, so the cursor goes back to it.
func (m *Model) filterChanged() {
	m.refreshItems()
	m.cursor, m.top = 0, 0
}

// assignHints labels the matches the window shows, in screen order from the
// bottom, so scrolled-off ones never use up the short labels. Hidden matches
// get none.
func (m *Model) assignHints() {
	m.labels = make([]string, len(m.matches))
	_, h := m.size()
	first := max(len(m.screen.rows)-h, 0)
	var shown []int
	for i, mt := range m.matches {
		if m.screen.visibleFrom(first, mt.Start, mt.End) {
			shown = append(shown, i)
		}
	}
	for k, l := range assignLabels(len(shown)) {
		m.labels[shown[k]] = l
	}
}

// listen reads one result from ch, then is re-armed by Update. gen is the
// index search the stream belongs to, or 0 for one that outlives rescans.
func listen(ch <-chan lookup.Result, gen int) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return resultMsg{r, ch, gen}
	}
}

// stream opens a result stream off the event loop, since opening one may read
// the disk.
func (m *Model) stream(gen int, open func() <-chan lookup.Result) tea.Cmd {
	m.streams++
	return func() tea.Msg { return listen(open(), gen)() }
}

// searchIndex looks for the paths that are still unresolved in the git indexes
// of the window's checkouts. Each rescan starts a new search in place of the
// last, since what it found is measured against the matches as they were.
func (m *Model) searchIndex() tea.Cmd {
	if m.indexCancel != nil {
		m.indexCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.indexCancel = cancel
	m.indexGen++
	ms := slices.Clone(m.matches)
	return m.stream(m.indexGen, func() <-chan lookup.Result { return m.backend.Index(ctx, ms) })
}

// Update never waits on the disk or on a program: slow reads run as commands
// and come back as messages, so keys are handled the moment they arrive.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	return m, tea.Batch(cmd, m.ensure())
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		resized := msg.Height != m.height
		m.width, m.height = msg.Width, msg.Height
		if resized && !m.touched {
			m.assignHints()
		}
	case quickMsg:
		// A full rescan that got here first already says more.
		if m.unchecked {
			m.unchecked = false
			return tea.Batch(m.applyRefined(msg.matches), m.searchIndex())
		}
	case refinedMsg:
		m.unchecked = false
		return tea.Batch(m.applyRefined(msg.matches), m.searchIndex())
	case resultMsg:
		if msg.gen != 0 && msg.gen != m.indexGen {
			return listen(msg.ch, msg.gen)
		}
		return tea.Batch(m.applyResult(msg.r), listen(msg.ch, msg.gen))
	case streamDoneMsg:
		m.streams--
		if m.streams == 0 && strings.HasPrefix(m.status, "searching") && m.sel < len(m.matches) {
			m.status = fmt.Sprintf("search finished: %d other copies", len(m.matches[m.sel].Alternatives))
		}
	case actionsMsg:
		return m.applyActions(msg)
	case statMsg:
		m.finish(msg.key, msg.id)
		if !msg.canceled {
			m.stats[msg.path] = msg.res
		}
	case previewMsg:
		m.finish(msg.key, msg.id)
		if !msg.canceled {
			if len(m.previews) >= maxPreviews {
				shown := ""
				if mt, ok := m.focus(); ok {
					shown = previewKey(mt)
				}
				maps.DeleteFunc(m.previews, func(k string, _ preview.Preview) bool { return k != msg.key && k != shown })
			}
			m.previews[msg.key] = msg.pv
		}
	case execDoneMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			if line := msg.stderr.lastLine(); line != "" {
				m.status += ": " + line
			}
			return nil
		}
		return m.quit()
	case tea.KeyPressMsg:
		m.touched = true
		return m.key(msg)
	case tea.PasteMsg:
		m.touched = true
		m.paste(msg.Content)
	}
	return nil
}

// applyResult folds a streamed copy into its match. When it changes what the
// open menu acts on, the actions are listed again.
func (m *Model) applyResult(r lookup.Result) tea.Cmd {
	if r.Err != nil || r.ID < 0 || r.ID >= len(m.matches) {
		return nil
	}
	before := m.matches[r.ID].Resolution
	if !m.merger.ApplyMatch(&m.matches[r.ID], r) || r.ID != m.sel {
		return nil
	}
	return m.menuSees(before)
}

// applyRefined takes a fuller rescan. Before any input the new matches simply
// replace the old; after, only resolutions of spans already shown are updated
// so labels and rows do not move under the user. An open menu is brought up to
// date with what it now points at.
func (m *Model) applyRefined(ms []detect.Match) tea.Cmd {
	m.dropCommitPreviews()
	if !m.touched {
		m.setMatches(ms)
		return nil
	}
	var before resolve.Resolution
	if m.sel < len(m.matches) {
		before = m.matches[m.sel].Resolution
	}
	type span struct {
		start, end int
		kind       detect.Kind
	}
	fresh := make(map[span]int, len(ms))
	for j, n := range ms {
		if _, dup := fresh[span{n.Start, n.End, n.Kind}]; !dup {
			fresh[span{n.Start, n.End, n.Kind}] = j
		}
	}
	for i, old := range m.matches {
		if j, ok := fresh[span{old.Start, old.End, old.Kind}]; ok {
			m.matches[i].Resolution = keepSearched(old.Resolution, ms[j].Resolution)
		}
	}
	if m.sel >= len(m.matches) {
		return nil
	}
	return m.menuSees(before)
}

// dropCommitPreviews forgets what was read about commits and stops the reads
// still out: which checkout holds a commit depends on the rescans, so an answer
// from before one may be wrong.
func (m *Model) dropCommitPreviews() {
	for _, mt := range m.matches {
		if mt.Kind != detect.KindSHA {
			continue
		}
		key := previewKey(mt)
		delete(m.previews, key)
		if l, ok := m.loads[key]; ok {
			l.cancel()
			delete(m.loads, key)
		}
	}
}

// keepSearched is the rescan's reading of a match with the copies a disk
// search found carried over, since no rescan repeats that search.
func keepSearched(old, rescan resolve.Resolution) resolve.Resolution {
	if old.Source == resolve.SourceSearch && old.Exists && !rescan.Exists {
		return old
	}
	rescan.Alternatives = slices.Clone(rescan.Alternatives)
	candidates := slices.Clone(old.Alternatives)
	if old.Source == resolve.SourceSearch && old.Exists {
		candidates = append(candidates, resolve.Candidate{Path: old.Path, Source: old.Source})
	}
	for _, c := range candidates {
		if c.Source == resolve.SourceSearch && c.Path != rescan.Path && !slices.ContainsFunc(rescan.Alternatives, func(a resolve.Candidate) bool { return a.Path == c.Path }) {
			rescan.Alternatives = append(rescan.Alternatives, c)
		}
	}
	return rescan
}

// menuSees re-lists the open menu's actions when its target was read
// differently from before.
func (m *Model) menuSees(before resolve.Resolution) tea.Cmd {
	after := m.matches[m.sel].Resolution
	switch m.mode {
	case menu:
		if m.choice == nil && (after.Path != before.Path || after.Exists != before.Exists || after.IsDir != before.IsDir) {
			return m.loadActions(false)
		}
	case alts:
		m.refreshAlts()
	}
	return nil
}

func (m *Model) quit() tea.Cmd {
	m.cancel()
	return tea.Quit
}

func (m *Model) key(k tea.KeyPressMsg) tea.Cmd {
	// q quits outright wherever it is not typed into a filter, ctrl+q even
	// there; esc steps back.
	if k.String() == "ctrl+c" || k.String() == "ctrl+q" || k.String() == "q" && (m.mode == Hints || m.mode == menu) {
		return m.quit()
	}
	switch m.mode {
	case Hints:
		return m.hintKey(k)
	case List:
		return m.listKey(k)
	case alts:
		return m.altsKey(k)
	}
	return m.menuKey(k)
}

func (m *Model) hintKey(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc":
		if m.typed != "" {
			m.typed = ""
			return nil
		}
		return m.quit()
	case "backspace":
		m.typed = trimLast(m.typed)
	case "enter":
		if i := m.firstLabelled(); i >= 0 {
			return m.openMenu(i)
		}
	case "/":
		m.mode, m.origin = List, List
	default:
		if k.Text == "" || !strings.Contains(labelAlphabet, k.Text) {
			return nil
		}
		typed := m.typed + k.Text
		if i := slices.Index(m.labels, typed); i >= 0 {
			m.typed = typed
			return m.openMenu(i)
		}
		if slices.ContainsFunc(m.labels, func(l string) bool { return strings.HasPrefix(l, typed) }) {
			m.typed = typed
		}
	}
	return nil
}

// firstLabelled is the match whose label is shortest among those still
// matching what was typed: the newest one when nothing was.
func (m *Model) firstLabelled() int {
	best := -1
	for i, l := range m.labels {
		if l != "" && strings.HasPrefix(l, m.typed) && (best < 0 || len(l) < len(m.labels[best]) || len(l) == len(m.labels[best]) && i > best) {
			best = i
		}
	}
	return best
}

func (m *Model) listKey(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.filterChanged()
			return nil
		}
		return m.quit()
	case "up", "ctrl+p":
		m.cursor = max(m.cursor-1, 0)
	case "down", "ctrl+n":
		m.cursor = min(m.cursor+1, max(len(m.items)-1, 0))
	case "pgup", "pgdown", "ctrl+u", "ctrl+d", "home", "end":
		m.cursor = jump(k.String(), m.cursor, len(m.items), m.page(m.listRows, 2))
	case "enter", "right":
		if m.cursor < len(m.items) {
			return m.openMenu(m.items[m.cursor])
		}
	case "backspace":
		m.filter = trimLast(m.filter)
		m.filterChanged()
	default:
		if k.Text != "" {
			m.filter += k.Text
			m.filterChanged()
		}
	}
	return nil
}

// jump moves a list cursor over n rows for the paging keys, where page is how
// many rows one screen shows. It stays inside the list.
func jump(key string, cur, n, page int) int {
	switch key {
	case "pgup":
		cur -= page
	case "pgdown":
		cur += page
	case "ctrl+u":
		cur -= max(page/2, 1)
	case "ctrl+d":
		cur += max(page/2, 1)
	case "home":
		cur = 0
	case "end":
		cur = n - 1
	}
	return min(max(cur, 0), max(n-1, 0))
}

// page is the rows the last frame showed, or what the window leaves after
// reserve rows of chrome when nothing has been drawn yet.
func (m *Model) page(drawn, reserve int) int {
	if drawn > 0 {
		return drawn
	}
	_, h := m.size()
	return max(h-reserve, 1)
}

var lineBreaks = strings.NewReplacer("\r", "", "\n", "")

// paste types text into whichever filter is open, as one line.
func (m *Model) paste(s string) {
	s = lineBreaks.Replace(s)
	switch m.mode {
	case List:
		m.filter += s
		m.filterChanged()
	case alts:
		m.altFilter += s
		m.refreshAlts()
	}
}

func (m *Model) openMenu(i int) tea.Cmd {
	m.mode = menu
	m.sel = i
	m.act = 0
	m.choice = nil
	m.status = ""
	m.actions, m.actReady, m.queued = nil, false, nil
	return m.loadActions(false)
}

// closeMenu returns to the hints or list, forgetting any copy picked for this
// menu so the target reads as it did before.
func (m *Model) closeMenu() {
	m.mode, m.act, m.typed, m.choice, m.status = m.origin, 0, "", nil, ""
	m.actions, m.actReady, m.queued = nil, false, nil
	m.actSeq++
	if m.actCancel != nil {
		m.actCancel()
	}
}

// target is the menu's match as it will be acted on: the picked copy when
// there is one.
func (m *Model) target() detect.Match {
	mt := m.matches[m.sel]
	if c := m.choice; c != nil {
		mt.Path, mt.Source, mt.Exists, mt.IsDir, mt.Expanded, mt.Unknown = c.Path, c.Source, c.Exists, c.IsDir, false, false
	}
	return mt
}

// loadActions lists the menu's actions off the event loop. check says the
// target's kind is not yet known, so it is looked up first.
func (m *Model) loadActions(check bool) tea.Cmd {
	m.actSeq++
	if m.actCancel != nil {
		m.actCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.actCancel = cancel
	seq, target, b := m.actSeq, m.target(), m.backend
	return func() tea.Msg {
		out := actionsMsg{seq: seq}
		if check {
			fi, err := b.Stat(target.Path)
			out.checked, out.exists, out.isDir = true, err == nil, err == nil && fi.IsDir()
			target.Exists, target.IsDir = out.exists, out.isDir
		}
		out.actions = b.Actions(ctx, target)
		return out
	}
}

func (m *Model) applyActions(msg actionsMsg) tea.Cmd {
	if msg.seq != m.actSeq || m.mode != menu && m.mode != alts {
		return nil
	}
	if msg.checked && m.choice != nil {
		m.choice.Exists, m.choice.IsDir = msg.exists, msg.isDir
	}
	m.actions, m.actReady = msg.actions, true
	m.act = min(m.act, max(len(m.actions)-1, 0))
	if q := m.queued; q != nil && m.mode == menu {
		m.queued = nil
		return m.menuKey(*q)
	}
	return nil
}

func (m *Model) menuKey(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc", "left":
		m.closeMenu()
		return nil
	case "up", "ctrl+p":
		m.act = max(m.act-1, 0)
		return nil
	case "down", "ctrl+n":
		m.act = min(m.act+1, max(len(m.actions)-1, 0))
		return nil
	case "tab":
		if len(m.matches[m.sel].Alternatives) > 0 {
			m.openAlts()
		}
		return nil
	}
	if !m.actReady {
		// Typed ahead of the list: it runs once the list is there.
		if k.Text != "" || k.String() == "enter" {
			m.queued = &k
		}
		return nil
	}
	if k.String() == "enter" {
		if m.act < len(m.actions) {
			return m.run(m.actions[m.act])
		}
		return nil
	}
	if i := slices.IndexFunc(m.actions, func(a action.Action) bool { return a.Key == k.Text }); i >= 0 && k.Text != "" {
		return m.run(m.actions[i])
	}
	return nil
}

func (m *Model) run(a action.Action) tea.Cmd {
	switch a.Verb {
	case action.Copy:
		return tea.Sequence(tea.SetClipboard(a.Arg), m.quit())
	case action.Search:
		m.status = "searching the disk…"
		id, mt := m.sel, m.matches[m.sel]
		return m.stream(0, func() <-chan lookup.Result { return m.backend.Deep(m.ctx, id, mt) })
	case action.Open, action.Tab, action.Run:
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
	// The terminal is cleared on return, so keep the end of stderr to explain
	// a failure in the menu.
	stderr := &tail{}
	cmd.Stderr = io.MultiWriter(os.Stderr, stderr)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return execDoneMsg{err, stderr} })
}

// tail keeps the last few KB written to it.
type tail struct{ b []byte }

func (t *tail) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if over := len(t.b) - 4096; over > 0 {
		t.b = t.b[over:]
	}
	return len(p), nil
}

func (t *tail) lastLine() string {
	if t == nil {
		return ""
	}
	lines := bytes.Split(bytes.TrimSpace(t.b), []byte("\n"))
	return string(bytes.TrimSpace(lines[len(lines)-1]))
}

// trimLast drops the last character the way a reader counts it: a letter with
// its accents, or an emoji sequence, goes whole.
func trimLast(s string) string {
	last := 0
	for i := 0; i < len(s); {
		g, _ := ansi.FirstGraphemeCluster(s[i:], ansi.GraphemeWidth)
		if g == "" {
			break
		}
		last, i = i, i+len(g)
	}
	return s[:last]
}
