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
	text     string
	width    int
	windowID int
	home     string
	editor   string
	// cwd resolves lazily: reading it touches the window's folder, which may
	// sit on a mount that is not answering.
	cwd func() string

	// mu guards r, which the UI loop and the search goroutines both use.
	mu sync.Mutex
	// r starts out blind to the disk and is replaced as the scans finish.
	r       *resolve.Resolver
	refined bool
	// commits caches which checkout holds each commit, "" for none.
	commits map[string]string
}

func newSession(text string, width, windowID int) *session {
	home, _ := os.UserHomeDir()
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	return &session{
		text: text, width: width, windowID: windowID, home: home, editor: editor,
		cwd:     sync.OnceValue(func() string { cwd, _ := os.Getwd(); return cwd }),
		r:       resolve.New(resolve.NoDisk(home)),
		commits: map[string]string{},
	}
}

// Textual scans the screen without touching the disk, so the first frame can
// be drawn at once. Paths show up by shape alone.
func (s *session) Textual() []detect.Match {
	return detect.ScanWidth(s.text, s.r, s.width)
}

// Quick checks paths against the window's folder and the home folder.
func (s *session) Quick(ctx context.Context) []detect.Match {
	env := resolve.NewGuard(quickBudget).Wrap(probe.Fast())
	r := resolve.New(env)
	ms := detect.ScanWidth(s.text, r, s.width)
	s.publish(r, false)
	return ms
}

// Refine rescans with every folder the window's processes sit in.
func (s *session) Refine(ctx context.Context) []detect.Match {
	r := resolve.New(resolve.NewGuard(fullBudget).Wrap(s.fullEnv(ctx)))
	ms := detect.ScanWidth(s.text, r, s.width)
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
	searcher := lookup.Searcher{Roots: s.roots(ms)}
	return searcher.Index(ctx, lookup.Queries(ms))
}

func (s *session) Deep(ctx context.Context, id int, m detect.Match) <-chan lookup.Result {
	q, ok := lookup.QueryFor(id, m)
	if !ok || s.home == "" {
		ch := make(chan lookup.Result)
		close(ch)
		return ch
	}
	ctx, cancel := context.WithTimeout(ctx, deepTimeout)
	var searcher lookup.Searcher
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

func (s *session) Actions(m detect.Match) []action.Action {
	return action.For(m, action.Context{Cwd: s.cwd(), CommitRepo: s.commitRepo})
}

// commitRepo finds the checkout holding sha among the window's working
// folder and the project roots of its processes.
func (s *session) commitRepo(sha string) string {
	if repo, ok := s.commits[sha]; ok {
		return repo
	}
	roots := s.roots(nil)
	if root := action.RepoRoot(s.cwd(), os.Stat); root != "" {
		roots = append([]string{root}, roots...)
	}
	repo := ""
	for _, root := range roots {
		if exec.Command("git", "-C", root, "cat-file", "-e", sha+"^{commit}").Run() == nil {
			repo = root
			break
		}
	}
	s.commits[sha] = repo
	return repo
}

func (s *session) Preview(m detect.Match, rows int) preview.Preview {
	switch {
	case m.Kind == detect.KindPath && m.Exists && m.IsDir:
		return preview.Dir(m.Path, rows)
	case m.Kind == detect.KindPath && m.Exists:
		return preview.File(m.Path, m.Line, rows)
	case m.Kind == detect.KindSHA:
		repo := s.commitRepo(m.Text)
		if repo == "" {
			return preview.Preview{Note: "not a commit in any repository this window uses"}
		}
		out, err := exec.Command("git", "-C", repo, "show", "--stat", "--no-color", "--format=%h %s%n%an, %ar  ("+tildeHome(repo, s.home)+")%n", m.Text).Output()
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

func (s *session) Stat(p string) (fs.FileInfo, error) { return os.Stat(p) }
