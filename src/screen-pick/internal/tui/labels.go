package tui

import (
	"cmp"
	"slices"
)

// labelAlphabet is home row first, so the commonest labels are the easiest
// to type. It leaves out q, which quits.
const labelAlphabet = "asdfghjklwertyuiopzxcvbnm"

// labels returns n prefix-free labels, shortest first: once a label is fully
// typed no longer label can still be meant.
func labels(n int) []string {
	if n <= 0 {
		return nil
	}
	hints := []string{""}
	offset := 0
	for len(hints)-offset < n || len(hints) == 1 {
		h := hints[offset]
		offset++
		for _, c := range labelAlphabet {
			hints = append(hints, h+string(c))
		}
	}
	out := hints[offset : offset+n]
	slices.SortStableFunc(out, func(a, b string) int { return cmp.Compare(len(a), len(b)) })
	return out
}

// assignLabels labels n matches given in screen order, handing the shortest
// labels to those nearest the bottom, where the newest output is.
func assignLabels(n int) []string {
	ls := labels(n)
	out := make([]string, n)
	for i := range n {
		out[n-1-i] = ls[i]
	}
	return out
}
