package lookup

import (
	"context"
	"errors"
	"slices"
	"testing"

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

func TestIndex(t *testing.T) {
	indexes := map[string][]string{
		"/p": {"src/main.rs", "README.md"},
		"/q": {"crates/x/src/main.rs"},
	}
	s := &Searcher{
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

func TestDeepDedupes(t *testing.T) {
	s := &Searcher{Walk: func(_ context.Context, roots []string, q string, found func(string)) error {
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
		{"duplicates and errors ignored", resolve.Resolution{}, []Result{
			{Path: "/p/a.rs", Source: idx}, {Path: "/p/a.rs", Source: idx}, {Err: errors.New("x")},
		}, resolve.Resolution{Path: "/p/a.rs", Source: idx, Exists: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m Merger
			res := tc.start
			for _, r := range tc.results {
				m.Apply(7, &res, r)
			}
			if diff := cmp.Diff(tc.want, res); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
