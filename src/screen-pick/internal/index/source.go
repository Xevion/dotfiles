package index

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charlievieth/fastwalk"
)

// Git lists the files git knows in the checkout at root, tracked or untracked
// but not ignored, relative to root.
func Git(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w: %s", root, err, bytes.TrimSpace(stderr.Bytes()))
	}
	var paths []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// skipDirs are directory names a walk never enters: VCS internals, build
// output and caches, which are large and never what was printed.
var skipDirs = map[string]bool{
	".git": true, ".hg": true, "node_modules": true, "target": true, ".cache": true,
	"__pycache__": true, ".venv": true, ".npm": true, ".rustup": true, ".cargo": true,
	"proc": true, "sys": true,
}

// Walk searches roots for entries whose path ends with query, calling found
// for each (possibly concurrently) until ctx is done or the roots are covered.
func Walk(ctx context.Context, roots []string, query string, found func(string)) error {
	q := parseQuery(query)
	if len(q) == 0 || q[len(q)-1] == "" {
		return nil
	}
	base := q[len(q)-1]
	conf := fastwalk.DefaultConfig.Copy()
	for _, root := range roots {
		err := fastwalk.Walk(conf, root, func(p string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil
			}
			if d.IsDir() && p != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			if d.Name() == base && suffixMatches(strings.Split(p, "/"), q) {
				found(p)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("walk %s: %w", root, err)
		}
	}
	return nil
}
