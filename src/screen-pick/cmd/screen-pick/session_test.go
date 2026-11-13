package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"screenpick/internal/detect"
	"screenpick/internal/resolve"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func repoSession(t *testing.T) (*session, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "first")
	s := newSession("", 80, 0)
	s.cwd = func() string { return dir }
	return s, dir
}

// A file named like the commit makes `git show <sha>` ambiguous unless the
// revision is closed off with --.
func TestPreviewOfACommitWithAHexNamedFile(t *testing.T) {
	s, dir := repoSession(t)
	sha := gitIn(t, dir, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, sha), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pv := s.Preview(context.Background(), detect.Match{Kind: detect.KindSHA, Text: sha}, 10)
	if pv.Note != "" || len(pv.Lines) == 0 || !strings.Contains(pv.Lines[0].Text, "first") {
		t.Errorf("preview should show the commit: %+v", pv)
	}
}

// The menu's action and preview loads ask for commits from separate
// goroutines.
func TestCommitRepoIsSafeConcurrently(t *testing.T) {
	s, dir := repoSession(t)
	sha := gitIn(t, dir, "rev-parse", "HEAD")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := s.commitRepo(context.Background(), sha); got != dir {
				t.Errorf("commitRepo = %q, want %q", got, dir)
			}
		}()
	}
	wg.Wait()
}

// A lookup cut short must not be remembered as "no repository has it".
func TestCommitRepoDoesNotCacheACancelledLookup(t *testing.T) {
	s, dir := repoSession(t)
	sha := gitIn(t, dir, "rev-parse", "HEAD")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := s.commitRepo(ctx, sha); got != "" {
		t.Errorf("a cancelled lookup found %q", got)
	}
	if got := s.commitRepo(context.Background(), sha); got != dir {
		t.Errorf("a later lookup should still find %q, got %q", dir, got)
	}
}

func countingIndex(n *atomic.Int32, gate <-chan struct{}) func(context.Context, string) ([]string, error) {
	return func(ctx context.Context, root string) ([]string, error) {
		n.Add(1)
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return []string{"a.txt"}, nil
	}
}

// The quick and the full rescan each search the same checkouts, so the second
// search must not list them again.
func TestIndexListsEachCheckoutOnce(t *testing.T) {
	s := newSession("", 80, 0)
	var n atomic.Int32
	gate := make(chan struct{})
	s.gitIndex = countingIndex(&n, gate)
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			if paths, err := s.loadIndex(context.Background(), "/r"); err != nil || len(paths) != 1 {
				t.Errorf("loadIndex = %v, %v", paths, err)
			}
		})
	}
	close(gate)
	wg.Wait()
	if _, err := s.loadIndex(context.Background(), "/r"); err != nil {
		t.Fatal(err)
	}
	if got := n.Load(); got != 1 {
		t.Errorf("listed %d times, want once", got)
	}
}

// A search replaced while it waits on a checkout's listing must not take the
// listing down with it: its successor reads the same one.
func TestReplacedSearchLeavesTheListingRunning(t *testing.T) {
	s := newSession("", 80, 0)
	var n atomic.Int32
	gate := make(chan struct{})
	s.gitIndex = countingIndex(&n, gate)

	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := s.loadIndex(ctx, "/r"); first <- err }()
	for n.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	second := make(chan []string, 1)
	go func() { p, _ := s.loadIndex(context.Background(), "/r"); second <- p }()
	cancel()
	if err := <-first; err == nil {
		t.Error("the replaced search should stop waiting")
	}
	close(gate)
	if p := <-second; len(p) != 1 || n.Load() != 1 {
		t.Errorf("the successor should get the one listing, got %v after %d listings", p, n.Load())
	}
}

// With every path already found there is nothing to search for, and no
// checkout is listed.
func TestIndexWithNothingUnresolvedDoesNoWork(t *testing.T) {
	s := newSession("", 80, 0)
	var n atomic.Int32
	s.gitIndex = countingIndex(&n, nil)
	found := detect.Match{Kind: detect.KindPath, Text: "a.txt"}
	found.Exists = true
	for range s.Index(context.Background(), []detect.Match{found}) {
		t.Error("no result expected")
	}
	if n.Load() != 0 {
		t.Errorf("listed %d checkouts for nothing", n.Load())
	}
}

// Before the window is probed the session knows only some of the checkouts, so
// a miss then must not be remembered as "no repository has it".
func TestCommitRepoDoesNotCacheAMissBeforeTheFinalScan(t *testing.T) {
	s, dir := repoSession(t)
	s.cwd = func() string { return t.TempDir() }
	sha := gitIn(t, dir, "rev-parse", "HEAD")
	if got := s.commitRepo(context.Background(), sha); got != "" {
		t.Fatalf("nothing points at the repository yet, found %q", got)
	}
	s.publish(resolve.New(resolve.Env{Home: "/nonexistent", Bases: []resolve.Base{{Dir: dir, Source: resolve.SourceProcess}}}), true)
	if got := s.commitRepo(context.Background(), sha); got != dir {
		t.Errorf("after the final scan the repository should be found, got %q", got)
	}
}
