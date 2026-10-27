package main

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"time"

	"screenpick/internal/action"
	"screenpick/internal/detect"
	"screenpick/internal/lookup"
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
}

func newSession(text string, width, windowID int, r *resolve.Resolver) *session {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	return &session{text: text, width: width, windowID: windowID, cwd: cwd, home: home, editor: editor, r: r}
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
	return action.For(m, action.Context{Cwd: s.cwd})
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
	if cmd != nil {
		cmd.Dir = s.cwd
	}
	return cmd
}

func (s *session) Stat(p string) (fs.FileInfo, error) { return os.Stat(p) }
