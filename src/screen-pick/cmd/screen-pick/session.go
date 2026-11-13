package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"screenpick/internal/action"
	"screenpick/internal/detect"
	"screenpick/internal/index"
	"screenpick/internal/lookup"
	"screenpick/internal/preview"
	"screenpick/internal/probe"
	"screenpick/internal/resolve"
)

// deepTimeout bounds an on-demand disk search.
const deepTimeout = 20 * time.Second

const (
	// quickBudget is how long the first disk pass waits on one check before
	// skipping that mount.
	quickBudget = 150 * time.Millisecond
	// fullBudget is the patient version, for the pass that has the final say
	// on a mount that merely answers slowly.
	fullBudget = 2 * time.Second
	// probeBudget bounds learning where the window's processes are.
	probeBudget = 3 * time.Second
)

// session is the picker's Backend over the real system.
type session struct {
	// ctx lasts until the picker closes, and bounds work shared between searches.
	ctx    context.Context
	cancel context.CancelFunc

	text     string
	width    int
	windowID int
	home     string
	editor   string
	// cwd resolves lazily: reading it touches the window's folder, which may
	// sit on a mount that is not answering.
	cwd func() string
	// lex is the disk-independent half of every scan, built once on first use.
	lex func() *detect.Lexed

	// mu guards r, which the UI loop and the search goroutines both use.
	mu sync.Mutex
	// r starts out blind to the disk and is replaced as the scans finish.
	r       *resolve.Resolver
	refined bool
	// commits caches which checkout holds each commit, "" for none. The menu's
	// action and preview loads share it, so commitsMu guards it.
	commitsMu sync.Mutex
	commits   map[string]string

	// indexes remembers each checkout's file list, which the quick and the full
	// rescan's searches both need.
	// stat is the one guarded stat every disk lookup of the session shares, so
	// a mount that stops answering is learned once and skipped everywhere.
	stat func(string) (fs.FileInfo, error)

	indexMu  sync.Mutex
	indexes  map[string]*listing
	gitIndex func(ctx context.Context, root string) ([]string, error)
}

// listing is one checkout's file list, shared by every search that asks for it.
type listing struct {
	done  chan struct{}
	paths []string
	err   error
}

func newSession(text string, width, windowID int) *session {
	home, _ := os.UserHomeDir()
	ctx, cancel := context.WithCancel(context.Background())
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	return &session{
		ctx: ctx, cancel: cancel,
		text: text, width: width, windowID: windowID, home: home, editor: editor,
		cwd:     sync.OnceValue(func() string { cwd, _ := os.Getwd(); return cwd }),
		r:       resolve.New(resolve.NoDisk(home)),
		lex:     sync.OnceValue(func() *detect.Lexed { return detect.Lex(text, width) }),
		commits: map[string]string{},
		stat:    resolve.NewGuard(fullBudget).WrapStat(os.Stat),
		indexes: map[string]*listing{}, gitIndex: index.Git,
	}
}

// Textual scans the screen without touching the disk, so the first frame can
// be drawn at once. Paths show up by shape alone.
func (s *session) Textual() []detect.Match {
	return s.lex().Resolve(s.r)
}

// Quick checks paths against the window's folder and the home folder.
func (s *session) Quick(ctx context.Context) []detect.Match {
	env := resolve.NewGuard(quickBudget).Wrap(probe.Fast())
	r := resolve.New(env)
	ms := s.lex().Resolve(r)
	s.publish(r, false)
	return ms
}

// Refine rescans with every folder the window's processes sit in.
func (s *session) Refine(ctx context.Context) []detect.Match {
	r := resolve.New(resolve.NewGuard(fullBudget).Wrap(s.fullEnv(ctx)))
	ms := s.lex().Resolve(r)
	s.publish(r, true)
	return ms
}

// fullEnv probes the window, falling back to what is known for free when that
// takes too long.
func (s *session) fullEnv(ctx context.Context) resolve.Env {
	ctx, cancel := context.WithTimeout(ctx, probeBudget)
	defer cancel()
	done := make(chan resolve.Env, 1)
	go func() { done <- probe.Full(ctx, s.windowID) }()
	select {
	case env := <-done:
		return env
	case <-ctx.Done():
		return probe.Fast()
	}
}

// publish makes r the resolver later lookups use. A quick scan never replaces
// a refined one.
func (s *session) publish(r *resolve.Resolver, final bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refined && !final {
		return
	}
	s.r, s.refined = r, s.refined || final
}

// roots lists the checkouts the window's processes work in, plus those of ms.
func (s *session) roots(ms []detect.Match) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return lookup.Roots(s.r, ms)
}

func (s *session) Index(ctx context.Context, ms []detect.Match) <-chan lookup.Result {
	qs := lookup.Queries(ms)
	if len(qs) == 0 {
		done := make(chan lookup.Result)
		close(done)
		return done
	}
	searcher := lookup.Searcher{Roots: s.roots(ms), Load: s.loadIndex, Stat: s.stat}
	return searcher.Index(ctx, qs)
}

// loadIndex lists a checkout's files once per session. The listing runs for
// the session, not for whichever search asked first, so a search that is
// replaced while it waits leaves the listing running for its successor.
func (s *session) loadIndex(ctx context.Context, root string) ([]string, error) {
	s.indexMu.Lock()
	l, ok := s.indexes[root]
	if !ok {
		l = &listing{done: make(chan struct{})}
		s.indexes[root] = l
		go func() {
			l.paths, l.err = s.gitIndex(s.ctx, root)
			close(l.done)
		}()
	}
	s.indexMu.Unlock()
	select {
	case <-l.done:
		return l.paths, l.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *session) Deep(ctx context.Context, id int, m detect.Match) <-chan lookup.Result {
	q, ok := lookup.QueryFor(id, m)
	if !ok || s.home == "" {
		ch := make(chan lookup.Result)
		close(ch)
		return ch
	}
	ctx, cancel := context.WithTimeout(ctx, deepTimeout)
	searcher := lookup.Searcher{Stat: s.stat}
	in := searcher.Deep(ctx, q, []string{s.home})
	out := make(chan lookup.Result)
	go func() {
		defer cancel()
		defer close(out)
		for r := range in {
			select {
			case out <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (s *session) Actions(ctx context.Context, m detect.Match) []action.Action {
	return action.For(m, action.Context{
		Cwd:        s.cwd(),
		CommitRepo: func(sha string) string { return s.commitRepo(ctx, sha) },
	})
}

// commitRepo finds the checkout holding sha among the window's working
// folder and the project roots of its processes. An answer cut short by ctx is
// not remembered, nor is "none" while the roots are still only those known
// without probing the window.
func (s *session) commitRepo(ctx context.Context, sha string) string {
	s.commitsMu.Lock()
	repo, ok := s.commits[sha]
	s.commitsMu.Unlock()
	if ok {
		return repo
	}
	roots, refined := s.rootsAndRefined()
	if root := action.RepoRoot(s.cwd(), os.Stat); root != "" {
		roots = append([]string{root}, roots...)
	}
	for _, root := range roots {
		if exec.CommandContext(ctx, "git", "-C", root, "cat-file", "-e", sha+"^{commit}").Run() == nil {
			repo = root
			break
		}
	}
	if ctx.Err() != nil {
		return ""
	}
	if repo != "" || refined {
		s.commitsMu.Lock()
		s.commits[sha] = repo
		s.commitsMu.Unlock()
	}
	return repo
}

// rootsAndRefined is roots(nil) with whether the resolver behind it is the
// final one.
func (s *session) rootsAndRefined() ([]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return lookup.Roots(s.r, nil), s.refined
}

func (s *session) Preview(ctx context.Context, m detect.Match, rows int) preview.Preview {
	switch {
	case m.Kind == detect.KindPath && m.Exists && m.IsDir:
		return preview.Dir(m.Path, rows)
	case m.Kind == detect.KindPath && m.Exists:
		return preview.File(m.Path, m.Line, rows)
	case m.Kind == detect.KindSHA:
		repo := s.commitRepo(ctx, m.Text)
		if repo == "" {
			return preview.Preview{Note: "not a commit in any repository this window uses"}
		}
		out, err := exec.CommandContext(ctx, "git", "-C", repo, "show", "--stat", "--no-color", "--format=%h %s%n%an, %ar  ("+tildeHome(repo, s.home)+")%n", m.Text, "--").Output()
		if err != nil {
			return preview.Preview{Note: fmt.Sprintf("git show: %v", err)}
		}
		return preview.Text(string(out), rows)
	}
	return preview.Preview{}
}

func tildeHome(p, home string) string {
	if home != "" && strings.HasPrefix(p+"/", home+"/") {
		return "~" + p[len(home):]
	}
	return p
}

func (s *session) Launch(a action.Action) error {
	cmd, _ := a.Command(s.editor)
	if cmd == nil {
		return nil
	}
	return action.Detach(cmd)
}

func (s *session) Command(a action.Action) *exec.Cmd {
	cmd, _ := a.Command(s.editor)
	if cmd != nil && cmd.Dir == "" {
		cmd.Dir = s.cwd()
	}
	return cmd
}

func (s *session) Stat(p string) (fs.FileInfo, error) { return s.stat(p) }
