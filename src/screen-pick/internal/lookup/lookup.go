// Package lookup resolves paths the direct checks missed, streaming results
// as each checkout's index loads, and on request by walking the disk.
package lookup

import (
	"context"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"screenpick/internal/decode"
	"screenpick/internal/detect"
	"screenpick/internal/index"
	"screenpick/internal/resolve"
)

// Query is one unresolved path, keyed by the caller's ID.
type Query struct {
	ID   int
	Text string
}

// Result is one file found for a query, or a root that failed to load (Err
// set, ID meaningless). Rank orders roots: lower is more trusted.
type Result struct {
	ID     int
	Path   string
	Source resolve.Source
	Root   string
	Rank   int
	Err    error
}

// Searcher finds files by path suffix. Nil functions use the index package.
type Searcher struct {
	// Roots are checkouts to index, most trusted first.
	Roots []string
	Load  func(ctx context.Context, root string) ([]string, error)
	Walk  func(ctx context.Context, roots []string, query string, found func(string)) error
}

func (s *Searcher) load(ctx context.Context, root string) ([]string, error) {
	if s.Load != nil {
		return s.Load(ctx, root)
	}
	return index.Git(ctx, root)
}

func (s *Searcher) walk(ctx context.Context, roots []string, query string, found func(string)) error {
	if s.Walk != nil {
		return s.Walk(ctx, roots, query, found)
	}
	return index.Walk(ctx, roots, query, found)
}

// Index loads every root's index concurrently and streams matches for each
// query. The channel closes once all roots are done or ctx ends.
func (s *Searcher) Index(ctx context.Context, queries []Query) <-chan Result {
	out := make(chan Result)
	send := sender(ctx, out)
	var wg sync.WaitGroup
	for rank, root := range s.Roots {
		wg.Go(func() {
			paths, err := s.load(ctx, root)
			if err != nil {
				send(Result{Root: root, Rank: rank, Err: err})
				return
			}
			ix := index.New(paths)
			for _, q := range queries {
				for _, rel := range ix.Match(q.Text) {
					if !send(Result{ID: q.ID, Path: filepath.Join(root, rel), Source: resolve.SourceIndex, Root: root, Rank: rank}) {
						return
					}
				}
			}
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// Deep walks roots for one query, streaming each hit. The channel closes when
// the walk finishes or ctx ends.
func (s *Searcher) Deep(ctx context.Context, q Query, roots []string) <-chan Result {
	out := make(chan Result)
	send := sender(ctx, out)
	go func() {
		defer close(out)
		var mu sync.Mutex
		seen := map[string]bool{}
		err := s.walk(ctx, roots, q.Text, func(p string) {
			mu.Lock()
			dup := seen[p]
			seen[p] = true
			mu.Unlock()
			if !dup {
				send(Result{ID: q.ID, Path: p, Source: resolve.SourceSearch})
			}
		})
		if err != nil {
			send(Result{ID: q.ID, Err: err})
		}
	}()
	return out
}

func sender(ctx context.Context, out chan<- Result) func(Result) bool {
	return func(r Result) bool {
		select {
		case out <- r:
			return true
		case <-ctx.Done():
			return false
		}
	}
}

// Queries picks the path matches worth searching for: unresolved, and not a
// Windows form, whose drive mapping is already exhaustive.
func Queries(ms []detect.Match) []Query {
	var qs []Query
	for i, m := range ms {
		if m.Kind != detect.KindPath || m.Exists {
			continue
		}
		if q := searchText(m.Text); q != "" {
			qs = append(qs, Query{ID: i, Text: q})
		}
	}
	return qs
}

// QueryFor builds the search for one match, reporting false when the token
// has no relative tail to look for.
func QueryFor(id int, m detect.Match) (Query, bool) {
	q := searchText(m.Text)
	return Query{ID: id, Text: q}, q != ""
}

// searchText reduces a token to the relative tail an index can match:
// decoded, with roots like /, ~/ and $VAR/ removed.
func searchText(token string) string {
	t := decode.Decode(token)[0]
	if decode.IsWindowsPath(t) || strings.HasPrefix(t, "%") {
		return ""
	}
	if strings.HasPrefix(t, "$") || strings.HasPrefix(t, "~") {
		_, t, _ = strings.Cut(t, "/")
	}
	t = strings.TrimLeft(t, "/")
	for strings.HasPrefix(t, "./") || strings.HasPrefix(t, "../") {
		_, t, _ = strings.Cut(t, "/")
	}
	return t
}

// Roots lists checkouts worth indexing: the project roots of the resolver's
// bases and of every existing path on screen, in that order.
func Roots(r *resolve.Resolver, ms []detect.Match) []string {
	var dirs []string
	for _, b := range r.Bases() {
		if b.Source != resolve.SourceHome {
			dirs = append(dirs, b.Dir)
		}
	}
	for _, m := range ms {
		if m.Kind != detect.KindPath || !m.Exists {
			continue
		}
		if m.IsDir {
			dirs = append(dirs, m.Path)
		} else {
			dirs = append(dirs, path.Dir(m.Path))
		}
	}
	var roots []string
	for _, d := range dirs {
		if root := r.ProjectRoot(d); root != "" && !slices.Contains(roots, root) {
			roots = append(roots, root)
		}
	}
	return roots
}

// Merger folds streamed results into resolutions. Direct-check results stay
// best; among index results the most trusted root wins. A disk search only
// adds alternatives once something was found, since its walk order says
// nothing about which copy was meant.
type Merger struct {
	rank map[int]int
}

// Apply records r against res, reporting whether res changed.
func (m *Merger) Apply(id int, res *resolve.Resolution, r Result) bool {
	if r.Err != nil || r.Path == res.Path || slices.ContainsFunc(res.Alternatives, func(c resolve.Candidate) bool { return c.Path == r.Path }) {
		return false
	}
	if m.rank == nil {
		m.rank = map[int]int{}
	}
	found := resolve.Candidate{Path: r.Path, Source: r.Source}
	best, searched := m.rank[id]
	switch {
	case !res.Exists:
		*res = resolve.Resolution{Path: r.Path, Source: r.Source, Exists: true}
		m.rank[id] = r.Rank
	case searched && r.Source == resolve.SourceIndex && r.Rank < best:
		res.Alternatives = slices.Insert(res.Alternatives, 0, resolve.Candidate{Path: res.Path, Source: res.Source})
		res.Path, res.Source = r.Path, r.Source
		m.rank[id] = r.Rank
	default:
		res.Alternatives = append(res.Alternatives, found)
	}
	return true
}
