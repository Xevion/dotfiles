package index

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
