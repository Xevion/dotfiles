package detect

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	"screenpick/internal/resolve"
)

// pickQuadratic is the straightforward form of pick that the sweep replaces.
func pickQuadratic(cands []cand) []cand {
	slices.SortStableFunc(cands, func(a, b cand) int {
		return cmp.Or(
			cmp.Compare(a.tier, b.tier),
			cmp.Compare(b.end-b.start, a.end-a.start),
			cmp.Compare(a.start, b.start),
		)
	})
	var picked []cand
	for _, c := range cands {
		if !slices.ContainsFunc(picked, func(p cand) bool { return c.start < p.end && p.start < c.end }) {
			picked = append(picked, c)
		}
	}
	slices.SortFunc(picked, func(a, b cand) int { return cmp.Compare(a.start, b.start) })
	return picked
}

func TestPickMatchesQuadratic(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 17))
	kinds := []Kind{KindPath, KindURL, KindHost, KindSHA}
	for range 2000 {
		n := rng.IntN(40)
		var in []cand
		for range n {
			a := rng.IntN(60)
			in = append(in, cand{kinds[rng.IntN(len(kinds))], a, a + 1 + rng.IntN(15), rng.IntN(tierSHA + 1), resolve.Resolution{Path: strconv.Itoa(rng.IntN(1000))}})
		}
		want := pickQuadratic(slices.Clone(in))
		got := pick(slices.Clone(in))
		if !slices.EqualFunc(want, got, func(a, b cand) bool {
			return a.kind == b.kind && a.start == b.start && a.end == b.end && a.tier == b.tier && a.res.Path == b.res.Path
		}) {
			t.Fatalf("pick(%v)\n got %v\nwant %v", in, got, want)
		}
	}
}
