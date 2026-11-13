package lookup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/goleak"

	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
	"screenpick/internal/resolve"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func collect(ch <-chan Result) []Result {
	var out []Result
	for r := range ch {
		out = append(out, r)
	}
	return out
}

// disk backs the files the fake indexes below list.
var disk = fakefs.New("/p/src/main.rs", "/p/README.md", "/q/crates/x/src/main.rs", "/p/pkg/")

func TestIndex(t *testing.T) {
	indexes := map[string][]string{
		"/p": {"src/main.rs", "README.md"},
		"/q": {"crates/x/src/main.rs"},
	}
	s := &Searcher{
		Stat:  disk.Lstat,
		Roots: []string{"/p", "/q", "/broken"},
		Load: func(_ context.Context, root string) ([]string, error) {
			if paths, ok := indexes[root]; ok {
				return paths, nil
			}
			return nil, errors.New("not a checkout")
		},
	}
	got := collect(s.Index(context.Background(), []Query{{ID: 4, Text: "src/main.rs"}, {ID: 9, Text: "nope.rs"}}))
	slices.SortFunc(got, func(a, b Result) int { return a.Rank - b.Rank })

	if len(got) != 3 {
		t.Fatalf("got %d results, want 3: %+v", len(got), got)
	}
	want := []Result{
		{ID: 4, Path: "/p/src/main.rs", Source: resolve.SourceIndex, Root: "/p", Rank: 0},
		{ID: 4, Path: "/q/crates/x/src/main.rs", Source: resolve.SourceIndex, Root: "/q", Rank: 1},
	}
	if diff := cmp.Diff(want, got[:2]); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	if got[2].Root != "/broken" || got[2].Err == nil {
		t.Errorf("want an error for /broken, got %+v", got[2])
	}
}

// A consumer that stops reading must not strand the loaders.
func TestIndexCancelled(t *testing.T) {
	release := make(chan struct{})
	s := &Searcher{
		Roots: []string{"/a", "/b"},
		Load: func(ctx context.Context, _ string) ([]string, error) {
			<-release
			return []string{"x.rs", "y/x.rs"}, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := s.Index(ctx, []Query{{ID: 1, Text: "x.rs"}})
	cancel()
	close(release)
	for range ch {
	}
}

// An index lists what git tracks, including files deleted since; those are
// not copies the user could open.
func TestIndexDropsFilesMissingFromDisk(t *testing.T) {
	s := &Searcher{
		Roots: []string{"/p"},
		Stat:  disk.Lstat,
		Load: func(context.Context, string) ([]string, error) {
			return []string{"src/main.rs", "old/main.rs"}, nil
		},
	}
	got := collect(s.Index(context.Background(), []Query{{ID: 1, Text: "main.rs"}}))
	if len(got) != 1 || got[0].Path != "/p/src/main.rs" || got[0].IsDir {
		t.Errorf("want only the file on disk, got %+v", got)
	}
}

// A stat that timed out says the mount is slow, not that the file is gone.
func TestIndexKeepsHitsItCouldNotCheck(t *testing.T) {
	s := &Searcher{
		Roots: []string{"/slow"},
		Stat:  func(string) (fs.FileInfo, error) { return nil, resolve.ErrSlow },
		Load:  func(context.Context, string) ([]string, error) { return []string{"src/main.rs"}, nil },
	}
	got := collect(s.Index(context.Background(), []Query{{ID: 1, Text: "main.rs"}}))
	if len(got) != 1 || got[0].Path != "/slow/src/main.rs" {
		t.Errorf("want the listed file kept, got %+v", got)
	}
}

// A directory found by name must not be offered as a file.
func TestDeepReportsDirectories(t *testing.T) {
	s := &Searcher{
		Stat: disk.Lstat,
		Walk: func(_ context.Context, _ []string, _ string, found func(string)) error {
			found("/p/pkg")
			found("/p/README.md")
			found("/p/gone")
			return nil
		},
	}
	got := collect(s.Deep(context.Background(), Query{ID: 2, Text: "x"}, []string{"/p"}))
	slices.SortFunc(got, func(a, b Result) int { return strings.Compare(a.Path, b.Path) })
	want := []Result{
		{ID: 2, Path: "/p/README.md", Source: resolve.SourceSearch},
		{ID: 2, Path: "/p/pkg", Source: resolve.SourceSearch, IsDir: true},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// A screen listing many checkouts must not start a git process for each at
// once, and the most trusted roots still go first.
func TestIndexBoundsConcurrentLoads(t *testing.T) {
	var roots []string
	for i := range 40 {
		roots = append(roots, fmt.Sprintf("/r%d", i))
	}
	var mu sync.Mutex
	var active, peak int
	var started []string
	s := &Searcher{
		Roots: roots,
		Stat:  func(string) (fs.FileInfo, error) { return disk.Lstat("/p/README.md") },
		Load: func(_ context.Context, root string) ([]string, error) {
			mu.Lock()
			active++
			peak = max(peak, active)
			started = append(started, root)
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
			return []string{"x.rs"}, nil
		},
	}
	got := collect(s.Index(context.Background(), []Query{{ID: 1, Text: "x.rs"}}))
	if len(got) != len(roots) {
		t.Errorf("got %d results, want %d", len(got), len(roots))
	}
	if peak > maxLoaders || peak < 2 {
		t.Errorf("peak concurrent loads = %d, want between 2 and %d", peak, maxLoaders)
	}
	if !slices.Contains(started[:maxLoaders], "/r0") {
		t.Errorf("most trusted root did not start first: %v", started[:maxLoaders])
	}
}

// A walk blocked handing over a hit must end when the search is cancelled.
func TestDeepCancelled(t *testing.T) {
	s := &Searcher{
		Stat: disk.Lstat,
		Walk: func(ctx context.Context, _ []string, _ string, found func(string)) error {
			found("/p/README.md")
			found("/p/pkg")
			<-ctx.Done()
			return ctx.Err()
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := s.Deep(ctx, Query{ID: 1, Text: "x"}, []string{"/p"})
	<-ch
	cancel()
	for range ch {
	}
}

func TestDeepDedupes(t *testing.T) {
	s := &Searcher{Stat: func(string) (fs.FileInfo, error) { return disk.Lstat("/p/README.md") }, Walk: func(_ context.Context, roots []string, q string, found func(string)) error {
		for range roots {
			found("/home/u/proj/src/main.rs")
		}
		return nil
	}}
	got := collect(s.Deep(context.Background(), Query{ID: 2, Text: "main.rs"}, []string{"/home/u", "/home/u/proj"}))
	want := []Result{{ID: 2, Path: "/home/u/proj/src/main.rs", Source: resolve.SourceSearch}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestDeepReportsWalkError(t *testing.T) {
	s := &Searcher{Walk: func(context.Context, []string, string, func(string)) error { return errors.New("boom") }}
	got := collect(s.Deep(context.Background(), Query{ID: 3}, []string{"/"}))
	if len(got) != 1 || got[0].Err == nil || got[0].ID != 3 {
		t.Errorf("want one error result for ID 3, got %+v", got)
	}
}

func TestSearchText(t *testing.T) {
	cases := map[string]string{
		"src/main.rs":         "src/main.rs",
		"./main.go":           "main.go",
		"../../lib/x.rs":      "lib/x.rs",
		"/app/index.js":       "app/index.js",
		"~/.config/x.toml":    ".config/x.toml",
		"$HOME/x.rs":          "x.rs",
		"${XDG}/fish/a.fish":  "fish/a.fish",
		`My\ Docs/a.txt`:      "My Docs/a.txt",
		`C:\Users\x.txt`:      "",
		`\\srv\share\x.txt`:   "",
		`%APPDATA%\App\c.ini`: "",
	}
	for in, want := range cases {
		if got := searchText(in); got != want {
			t.Errorf("searchText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueries(t *testing.T) {
	ms := []detect.Match{
		{Kind: detect.KindPath, Text: "gone.rs"},
		{Kind: detect.KindPath, Text: "here.rs", Resolution: resolve.Resolution{Exists: true}},
		{Kind: detect.KindURL, Text: "https://x.y/a.rs"},
		{Kind: detect.KindPath, Text: `C:\x.rs`},
		{Kind: detect.KindPath, Text: "./sub/lost.go"},
	}
	want := []Query{{ID: 0, Text: "gone.rs"}, {ID: 4, Text: "sub/lost.go"}}
	if diff := cmp.Diff(want, Queries(ms)); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestRoots(t *testing.T) {
	disk := fakefs.New(
		"/home/u/proj/.git/",
		"/home/u/proj/src/main.rs",
		"/home/u/other/.git/",
		"/home/u/other/a.txt",
		"/home/u/loose/b.txt",
	)
	r := resolve.New(resolve.Env{
		Bases: []resolve.Base{{Dir: "/home/u/proj/src", Source: resolve.SourceProcess}},
		Home:  "/home/u", Lstat: disk.Lstat, Readlink: disk.Readlink, ReadDir: disk.ReadDir,
	})
	ms := []detect.Match{
		{Kind: detect.KindPath, Resolution: resolve.Resolution{Path: "/home/u/other/a.txt", Exists: true}},
		{Kind: detect.KindPath, Resolution: resolve.Resolution{Path: "/home/u/loose/b.txt", Exists: true}},
		{Kind: detect.KindPath, Resolution: resolve.Resolution{Path: "/home/u/proj", Exists: true, IsDir: true}},
	}
	want := []string{"/home/u/proj", "/home/u/other"}
	if diff := cmp.Diff(want, Roots(r, ms)); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestMerger(t *testing.T) {
	cand := func(p string, s resolve.Source) resolve.Candidate { return resolve.Candidate{Path: p, Source: s} }
	idx := resolve.SourceIndex
	cases := []struct {
		name    string
		start   resolve.Resolution
		results []Result
		want    resolve.Resolution
	}{
		{"first hit resolves", resolve.Resolution{Path: "/p/gone.rs"}, []Result{{Path: "/p/a/gone.rs", Source: idx}},
			resolve.Resolution{Path: "/p/a/gone.rs", Source: idx, Exists: true}},
		{"later hits are alternatives", resolve.Resolution{}, []Result{
			{Path: "/p/a.rs", Source: idx}, {Path: "/p/b/a.rs", Source: idx},
		}, resolve.Resolution{Path: "/p/a.rs", Source: idx, Exists: true, Alternatives: []resolve.Candidate{cand("/p/b/a.rs", idx)}}},
		{"trusted root takes over", resolve.Resolution{}, []Result{
			{Path: "/win/a.rs", Source: idx, Rank: 2}, {Path: "/proc/a.rs", Source: idx, Rank: 0},
		}, resolve.Resolution{Path: "/proc/a.rs", Source: idx, Exists: true, Alternatives: []resolve.Candidate{cand("/win/a.rs", idx)}}},
		{"direct hit stays best", resolve.Resolution{Path: "/p/a.rs", Source: resolve.SourceProcess, Exists: true}, []Result{
			{Path: "/q/a.rs", Source: idx, Rank: 0},
		}, resolve.Resolution{Path: "/p/a.rs", Source: resolve.SourceProcess, Exists: true, Alternatives: []resolve.Candidate{cand("/q/a.rs", idx)}}},
		{"disk search never displaces a found file", resolve.Resolution{}, []Result{
			{Path: "/win/a.rs", Source: idx, Rank: 2}, {Path: "/home/a.rs", Source: resolve.SourceSearch},
		}, resolve.Resolution{Path: "/win/a.rs", Source: idx, Exists: true, Alternatives: []resolve.Candidate{cand("/home/a.rs", resolve.SourceSearch)}}},
		{"duplicates and errors ignored", resolve.Resolution{}, []Result{
			{Path: "/p/a.rs", Source: idx}, {Path: "/p/a.rs", Source: idx}, {Err: errors.New("x")},
		}, resolve.Resolution{Path: "/p/a.rs", Source: idx, Exists: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m Merger
			mt := pathMatch("a.rs", 7)
			mt.Resolution = tc.start
			for _, r := range tc.results {
				m.ApplyMatch(&mt, r)
			}
			if diff := cmp.Diff(tc.want, mt.Resolution); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func pathMatch(text string, start int) detect.Match {
	return detect.Match{Kind: detect.KindPath, Text: text, Start: start, End: start + len(text)}
}

// A rescan can replace a match's resolution with a direct hit while the
// merger still remembers the index rank of the one it replaced: that rank must
// not let a later index result displace the direct hit.
func TestMergerForgetsTheRankOfAReplacedResolution(t *testing.T) {
	var m Merger
	mt := pathMatch("a.rs", 10)
	m.ApplyMatch(&mt, Result{Path: "/win/a.rs", Source: resolve.SourceIndex, Rank: 2})
	if mt.Path != "/win/a.rs" {
		t.Fatalf("first hit should resolve: %+v", mt.Resolution)
	}
	mt.Resolution = resolve.Resolution{Path: "/p/a.rs", Source: resolve.SourceProcess, Exists: true}
	m.ApplyMatch(&mt, Result{Path: "/q/a.rs", Source: resolve.SourceIndex, Rank: 0})
	want := resolve.Resolution{Path: "/p/a.rs", Source: resolve.SourceProcess, Exists: true,
		Alternatives: []resolve.Candidate{{Path: "/q/a.rs", Source: resolve.SourceIndex}}}
	if diff := cmp.Diff(want, mt.Resolution); diff != "" {
		t.Errorf("a direct hit stays best (-want +got):\n%s", diff)
	}
}

// Matches are identified by what they are, not by where they sit in a list that
// a rescan may reorder or shrink.
func TestMergerKeepsMatchesApart(t *testing.T) {
	var m Merger
	idx := resolve.SourceIndex
	a, b := pathMatch("a.rs", 3), pathMatch("a.rs", 40)
	m.ApplyMatch(&a, Result{Path: "/win/a.rs", Source: idx, Rank: 2})
	m.ApplyMatch(&b, Result{Path: "/proc/a.rs", Source: idx, Rank: 0})
	m.ApplyMatch(&b, Result{Path: "/mid/a.rs", Source: idx, Rank: 1})
	m.ApplyMatch(&a, Result{Path: "/mid/a.rs", Source: idx, Rank: 1})
	if b.Path != "/proc/a.rs" || len(b.Alternatives) != 1 {
		t.Errorf("the second match's own rank 0 should hold: %+v", b.Resolution)
	}
	if a.Path != "/mid/a.rs" {
		t.Errorf("rank 1 beats this match's own rank 2: %+v", a.Resolution)
	}
}
