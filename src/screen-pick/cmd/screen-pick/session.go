package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
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

// session is the picker's Backend over the real system.
type session struct {
	text     string
	width    int
	windowID int
	cwd      string
	home     string
	editor   string
	// r is replaced by Refine; later calls happen after it on the UI loop.
	r *resolve.Resolver
	// commits caches which checkout holds each commit, "" for none.
	commits map[string]string
}

func newSession(text string, width, windowID int, r *resolve.Resolver) *session {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	return &session{text: text, width: width, windowID: windowID, cwd: cwd, home: home, editor: editor, r: r, commits: map[string]string{}}
}

func (s *session) Refine(ctx context.Context) []detect.Match {
	s.r = resolve.New(probe.Full(ctx, s.windowID))
	return detect.ScanWidth(s.text, s.r, s.width)
}

func (s *session) Index(ctx context.Context, ms []detect.Match) <-chan lookup.Result {
	searcher := lookup.Searcher{Roots: lookup.Roots(s.r, ms)}
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
	return action.For(m, action.Context{Cwd: s.cwd, CommitRepo: s.commitRepo})
}

// commitRepo finds the checkout holding sha among the window's working
// folder and the project roots of its processes.
func (s *session) commitRepo(sha string) string {
	if repo, ok := s.commits[sha]; ok {
		return repo
	}
	roots := lookup.Roots(s.r, nil)
	if root := action.RepoRoot(s.cwd, os.Stat); root != "" {
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
		cmd.Dir = s.cwd
	}
	return cmd
}

func (s *session) Stat(p string) (fs.FileInfo, error) { return os.Stat(p) }
