package index

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestSuffixMatch(t *testing.T) {
	index := []string{
		"config/app/app.conf",
		"templates/app/app.conf.tmpl",
		"src/main.rs",
		"crates/x/src/main.rs",
		"src/a/b/main.rs",
		"webapp/app.conf",
	}
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"elided prefix", ".../app/app.conf", []string{"config/app/app.conf"}},
		{"unicode elided prefix", "…/app/app.conf", []string{"config/app/app.conf"}},
		{"plain suffix", "app/app.conf", []string{"config/app/app.conf"}},
		{"exact beats deeper", "src/main.rs", []string{"src/main.rs", "crates/x/src/main.rs"}},
		{"elided middle", "src/…/main.rs", []string{"src/a/b/main.rs"}},
		{"basename, shallowest then lexical", "main.rs", []string{"src/main.rs", "crates/x/src/main.rs", "src/a/b/main.rs"}},
		{"component boundary", "pp/app.conf", nil},
		{"case-sensitive", "SRC/MAIN.RS", nil},
		{"no match", "nope.rs", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, New(index).Match(tc.query)); diff != "" {
				t.Errorf("Match(%q) (-want +got):\n%s", tc.query, diff)
			}
		})
	}
}

func TestMatchQueryForms(t *testing.T) {
	ix := New([]string{"src/main.rs", "a/src/main.rs", "main.rs"})
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"dot-slash prefix", "./src/main.rs", []string{"src/main.rs", "a/src/main.rs"}},
		{"bare basename", "main.rs", []string{"main.rs", "src/main.rs", "a/src/main.rs"}},
		{"only an elision", "…", nil},
		{"trailing elision", "src/…", nil},
		{"empty", "", nil},
		{"doubled slashes", "src//main.rs", []string{"src/main.rs", "a/src/main.rs"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, ix.Match(tc.query)); diff != "" {
				t.Errorf("Match(%q) (-want +got):\n%s", tc.query, diff)
			}
		})
	}
}

// synthetic builds n paths spread over a few hundred directories, like a
// large monorepo checkout.
func synthetic(n int) []string {
	paths := make([]string, n)
	for i := range paths {
		paths[i] = fmt.Sprintf("pkg%d/sub%d/file%d.go", i%300, i%17, i)
	}
	return paths
}

func BenchmarkMatch(b *testing.B) {
	paths := synthetic(100_000)
	ix := New(paths)
	b.Run("indexed", func(b *testing.B) {
		for b.Loop() {
			ix.Match("sub3/file4250.go")
		}
	})
	b.Run("build", func(b *testing.B) {
		for b.Loop() {
			New(paths)
		}
	})
}

// suffixMatchesNaive is the direct recursive definition of a gap match, kept
// as the oracle for the linear matcher.
func suffixMatchesNaive(comps, q []string) bool {
	if len(q) == 0 {
		return true
	}
	last := q[len(q)-1]
	if last == "" {
		for k := len(comps) - 1; k >= 0; k-- {
			if suffixMatchesNaive(comps[:k], q[:len(q)-1]) {
				return true
			}
		}
		return false
	}
	return len(comps) > 0 && comps[len(comps)-1] == last && suffixMatchesNaive(comps[:len(comps)-1], q[:len(q)-1])
}

// A query with many elisions must not blow up on a path of repeated names.
func TestMatchManyElisionsIsFast(t *testing.T) {
	deep := strings.Repeat("a/", 60) + "z.go"
	ix := New([]string{deep})
	query := "b/…/" + strings.Repeat("a/…/", 30) + "z.go"
	done := make(chan []string, 1)
	go func() { done <- ix.Match(query) }()
	select {
	case got := <-done:
		if len(got) != 0 {
			t.Errorf("no b component exists, got %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("elided query did not finish in 2s")
	}
}

func TestSuffixMatchesAgreesWithDefinition(t *testing.T) {
	names := []string{"a", "b"}
	var comps [][]string
	var gen func(prefix []string, n int)
	gen = func(prefix []string, n int) {
		comps = append(comps, slices.Clone(prefix))
		if n == 0 {
			return
		}
		for _, c := range names {
			gen(append(prefix, c), n-1)
		}
	}
	gen(nil, 6)
	queries := [][]string{}
	var genQ func(prefix []string, n int)
	genQ = func(prefix []string, n int) {
		if len(prefix) > 0 && prefix[0] != "" {
			queries = append(queries, slices.Clone(prefix))
		}
		if n == 0 {
			return
		}
		for _, c := range []string{"a", "b", ""} {
			if c == "" && len(prefix) > 0 && prefix[len(prefix)-1] == "" {
				continue
			}
			genQ(append(prefix, c), n-1)
		}
	}
	genQ(nil, 5)
	for _, q := range queries {
		if q[len(q)-1] == "" {
			continue
		}
		for _, c := range comps {
			if got, want := suffixMatches(c, q), suffixMatchesNaive(c, q); got != want {
				t.Fatalf("suffixMatches(%q, %q) = %v, want %v", c, q, got, want)
			}
		}
	}
}

func FuzzMatch(f *testing.F) {
	f.Add("src/a/b/main.rs", "src/…/main.rs")
	f.Add("a/a/a/a", "a/.../a")
	f.Fuzz(func(t *testing.T, path, query string) {
		if len(path) > 200 || len(query) > 200 {
			return
		}
		ix := New([]string{path})
		got := len(ix.Match(query)) == 1
		q := parseQuery(query)
		want := len(q) > 0 && q[len(q)-1] != "" && suffixMatchesNaive(strings.Split(path, "/"), q) && pathBase(path) == q[len(q)-1]
		if got != want && len(q) < 8 {
			t.Fatalf("Match(%q) over %q = %v, want %v", query, path, got, want)
		}
	})
}

func pathBase(p string) string { return p[strings.LastIndexByte(p, '/')+1:] }
