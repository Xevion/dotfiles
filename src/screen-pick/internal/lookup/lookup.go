// Package lookup resolves paths the direct checks missed, streaming results
// as each checkout's index loads, and on request by walking the disk.
package lookup

import (
	"context"
	"errors"
	"io/fs"
	"os"
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
	IsDir  bool
	Err    error
}

// Searcher finds files by path suffix. Nil functions use the index package and
// the real disk.
type Searcher struct {
	// Stat confirms each hit is still on disk and tells files from folders.
	Stat func(string) (fs.FileInfo, error)
	// Roots are checkouts to index, most trusted first.
	Roots []string
	Load  func(ctx context.Context, root string) ([]string, error)
	Walk  func(ctx context.Context, roots []string, query string, found func(string)) error
}

func (s *Searcher) stat(p string) (fs.FileInfo, error) {
	if s.Stat != nil {
		return s.Stat(p)
	}
	return os.Stat(p)
}

// confirm stats a hit to tell files from folders. A stat that timed out leaves
// the hit in: the file is listed, and only the mount is not answering.
func (s *Searcher) confirm(p string) (isDir, ok bool) {
	fi, err := s.stat(p)
	switch {
	case err == nil:
		return fi.IsDir(), true
	case errors.Is(err, resolve.ErrSlow):
		return false, true
	}
	return false, false
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

// maxLoaders bounds how many checkouts load at once.
const maxLoaders = 4

// Index loads the roots' indexes, a few at a time and most trusted first, and
// streams matches for each query. The channel closes once all roots are done
// or ctx ends.
func (s *Searcher) Index(ctx context.Context, queries []Query) <-chan Result {
	out := make(chan Result)
	send := sender(ctx, out)
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(maxLoaders, len(s.Roots)) {
		wg.Go(func() {
			for rank := range next {
				s.indexRoot(ctx, rank, queries, send)
			}
		})
	}
	go func() {
		defer close(next)
		for rank := range s.Roots {
			select {
			case next <- rank:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// indexRoot streams one checkout's matches for every query.
func (s *Searcher) indexRoot(ctx context.Context, rank int, queries []Query, send func(Result) bool) {
	root := s.Roots[rank]
	paths, err := s.load(ctx, root)
	if err != nil {
		send(Result{Root: root, Rank: rank, Err: err})
		return
	}
	ix := index.New(paths)
	for _, q := range queries {
		for _, rel := range ix.Match(q.Text) {
			p := filepath.Join(root, rel)
			isDir, ok := s.confirm(p)
			if !ok {
				continue
			}
			if !send(Result{ID: q.ID, Path: p, Source: resolve.SourceIndex, Root: root, Rank: rank, IsDir: isDir}) {
				return
			}
		}
	}
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
			if dup {
				return
			}
			if isDir, ok := s.confirm(p); ok {
				send(Result{ID: q.ID, Path: p, Source: resolve.SourceSearch, IsDir: isDir})
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

// Key identifies a match by what it is, so state kept about it survives a
// rescan that reorders or drops its neighbours.
type Key struct {
	Start int
	Text  string
}

// KeyOf is the identity of m.
func KeyOf(m detect.Match) Key { return Key{m.Start, m.Text} }

// ranked remembers which index root supplied a match's best path. It only
// applies while that path is still the best one: a rescan that resolves the
// match some other way leaves a rank that describes nothing.
type ranked struct {
	path   string
	source resolve.Source
	rank   int
}

// Merger folds streamed results into resolutions. Direct-check results stay
// best; among index results the most trusted root wins. A disk search only
// adds alternatives once something was found, since its walk order says
// nothing about which copy was meant.
type Merger struct {
	rank map[Key]ranked
}

// ApplyMatch records r against mt, reporting whether its resolution changed.
func (m *Merger) ApplyMatch(mt *detect.Match, r Result) bool {
	return m.apply(KeyOf(*mt), &mt.Resolution, r)
}

func (m *Merger) apply(key Key, res *resolve.Resolution, r Result) bool {
	if r.Err != nil || r.Path == res.Path || slices.ContainsFunc(res.Alternatives, func(c resolve.Candidate) bool { return c.Path == r.Path }) {
		return false
	}
	if m.rank == nil {
		m.rank = map[Key]ranked{}
	}
	found := resolve.Candidate{Path: r.Path, Source: r.Source}
	best, searched := m.rank[key]
	searched = searched && res.Exists && best.path == res.Path && best.source == res.Source
	switch {
	case !res.Exists:
		*res = resolve.Resolution{Path: r.Path, Source: r.Source, Exists: true, IsDir: r.IsDir}
		m.rank[key] = ranked{r.Path, r.Source, r.Rank}
	case searched && r.Source == resolve.SourceIndex && r.Rank < best.rank:
		res.Alternatives = slices.Insert(res.Alternatives, 0, resolve.Candidate{Path: res.Path, Source: res.Source})
		res.Path, res.Source, res.IsDir = r.Path, r.Source, r.IsDir
		m.rank[key] = ranked{r.Path, r.Source, r.Rank}
	default:
		res.Alternatives = append(res.Alternatives, found)
	}
	return true
}
