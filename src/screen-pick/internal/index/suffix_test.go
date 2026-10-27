package index

import (
	"fmt"
	"strings"
	"testing"

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
			if diff := cmp.Diff(tc.want, SuffixMatch(index, tc.query)); diff != "" {
				t.Errorf("SuffixMatch(%q) (-want +got):\n%s", tc.query, diff)
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
	b.Run("linear", func(b *testing.B) {
		q := parseQuery("sub3/file4250.go")
		for b.Loop() {
			for _, p := range paths {
				suffixMatches(strings.Split(p, "/"), q)
			}
		}
	})
	b.Run("build", func(b *testing.B) {
		for b.Loop() {
			New(paths)
		}
	})
}
