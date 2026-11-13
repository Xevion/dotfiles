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
// of at least one component. Each run of literals between gaps is taken at its
// rightmost place, which leaves the most room for the runs before it, so one
// right-to-left pass decides the match.
func suffixMatches(comps, q []string) bool {
	end := len(comps)
	gap := false
	for i := len(q); i > 0; {
		if q[i-1] == "" {
			gap = true
			i--
			continue
		}
		j := i
		for j > 0 && q[j-1] != "" {
			j--
		}
		run := q[j:i]
		start := end - len(run)
		if gap {
			start--
			for start >= 0 && !slices.Equal(comps[start:start+len(run)], run) {
				start--
			}
		} else if start >= 0 && !slices.Equal(comps[start:end], run) {
			return false
		}
		if start < 0 {
			return false
		}
		end, gap, i = start, false, j
	}
	return !gap || end > 0
}

func sortShallowFirst(paths []string) {
	slices.SortStableFunc(paths, func(a, b string) int {
		return cmp.Or(cmp.Compare(strings.Count(a, "/"), strings.Count(b, "/")), strings.Compare(a, b))
	})
}
