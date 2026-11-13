package index

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// tree writes files (relative to a fresh temp dir) and returns the dir.
func tree(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := tree(t, "src/main.rs", "README.md", "untracked.txt", "build/out.o", ".gitignore")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("build/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "src", "README.md", ".gitignore"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	got, err := Git(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{".gitignore", "README.md", "src/main.rs", "untracked.txt"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestGitOutsideCheckout(t *testing.T) {
	if _, err := Git(context.Background(), t.TempDir()); err == nil {
		t.Error("want an error outside a checkout")
	}
}

func TestWalk(t *testing.T) {
	root := tree(t,
		"a/src/main.rs",
		"b/deep/src/main.rs",
		"c/main.rs",
		"node_modules/x/src/main.rs",
		".git/src/main.rs",
		"target/src/main.rs",
	)
	var mu sync.Mutex
	var got []string
	err := Walk(context.Background(), []string{root}, "src/main.rs", func(p string) {
		rel, _ := filepath.Rel(root, p)
		mu.Lock()
		got = append(got, rel)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{"a/src/main.rs", "b/deep/src/main.rs"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestWalkStopsWhenCancelled(t *testing.T) {
	root := tree(t, "a/x.txt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	if err := Walk(ctx, []string{root}, "x.txt", func(string) { called = true }); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("found a file after cancellation")
	}
}

func walkRel(t *testing.T, root, query string) []string {
	t.Helper()
	var mu sync.Mutex
	var got []string
	err := Walk(context.Background(), []string{root}, query, func(p string) {
		rel, _ := filepath.Rel(root, p)
		mu.Lock()
		got = append(got, rel)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	return got
}

// A skipped name is still searched when the query itself goes through it.
func TestWalkEntersSkippedDirNamedByQuery(t *testing.T) {
	root := tree(t, "proj/target/debug/app", "proj/src/target/debug/app", "other/target/debug/app")
	got := walkRel(t, root, "target/debug/app")
	want := []string{"other/target/debug/app", "proj/src/target/debug/app", "proj/target/debug/app"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	if got := walkRel(t, root, "debug/app"); len(got) != 0 {
		t.Errorf("a query not naming target should skip it, got %v", got)
	}
}

// proc and sys only mean the kernel's trees at the top of the filesystem.
func TestWalkSearchesProjectDirsNamedSys(t *testing.T) {
	root := tree(t, "proj/sys/clock.c", "proj/proc/exec.c")
	got := walkRel(t, root, "clock.c")
	if diff := cmp.Diff([]string{"proj/sys/clock.c"}, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestGitDeduplicatesUnmergedPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := tree(t)
	run := func(stdin string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("", "init", "-q")
	blob := run("x", "hash-object", "-w", "--stdin")
	var info strings.Builder
	for stage := 1; stage <= 3; stage++ {
		fmt.Fprintf(&info, "100644 %s %d\tconflict.txt\n", blob, stage)
	}
	run(info.String(), "update-index", "--index-info")
	got, err := Git(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"conflict.txt"}, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}
