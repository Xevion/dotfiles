// Package index finds files by the tail of their path: in git's index of a
// checkout, or by walking the disk.
package index

import (
	"cmp"
	"path"
	"slices"
	"strings"
)

// Index is a fixed list of slash-separated relative paths, keyed by basename
// so a query only examines entries that could end the right way.
type Index struct {
	paths  []string
	byBase map[string][]int
}

func New(paths []string) *Index {
	ix := &Index{paths: paths, byBase: make(map[string][]int, len(paths))}
	for i, p := range paths {
		b := path.Base(p)
		ix.byBase[b] = append(ix.byBase[b], i)
	}
	return ix
}

func (ix *Index) Len() int { return len(ix.paths) }

// Match returns entries that end with query on a component boundary,
// shallowest first. An elided component (... or …) stands for one or more
// components; a leading one is dropped.
func (ix *Index) Match(query string) []string {
	q := parseQuery(query)
	if len(q) == 0 || q[len(q)-1] == "" {
		return nil
	}
	var out []string
	for _, i := range ix.byBase[q[len(q)-1]] {
		if suffixMatches(strings.Split(ix.paths[i], "/"), q) {
			out = append(out, ix.paths[i])
		}
	}
	sortShallowFirst(out)
	return out
}

// SuffixMatch is Match over a one-off list.
func SuffixMatch(paths []string, query string) []string {
	return New(paths).Match(query)
}

// parseQuery splits query into components, with "" marking an elided run.
// Leading and repeated elisions collapse away.
func parseQuery(query string) []string {
	var q []string
	for _, c := range strings.Split(query, "/") {
		switch c {
		case "", ".":
		case "...", "…":
			if len(q) > 0 && q[len(q)-1] != "" {
				q = append(q, "")
			}
		default:
			q = append(q, c)
		}
	}
	return q
}

// suffixMatches matches q against the tail of comps, where "" in q is a gap
// of at least one component.
func suffixMatches(comps, q []string) bool {
	if len(q) == 0 {
		return true
	}
	last := q[len(q)-1]
	if last == "" {
		for k := len(comps) - 1; k >= 0; k-- {
			if suffixMatches(comps[:k], q[:len(q)-1]) {
				return true
			}
		}
		return false
	}
	return len(comps) > 0 && comps[len(comps)-1] == last && suffixMatches(comps[:len(comps)-1], q[:len(q)-1])
}

func sortShallowFirst(paths []string) {
	slices.SortStableFunc(paths, func(a, b string) int {
		return cmp.Or(cmp.Compare(strings.Count(a, "/"), strings.Count(b, "/")), strings.Compare(a, b))
	})
}
