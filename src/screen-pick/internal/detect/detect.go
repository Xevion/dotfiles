// Package detect finds openable targets (paths, URLs, hosts, commit hashes)
// in captured terminal text.
package detect

import "screenpick/internal/resolve"

type Kind uint8

const (
	KindPath Kind = iota + 1
	KindURL
	KindHost
	KindSHA
)

func (k Kind) String() string {
	switch k {
	case KindPath:
		return "path"
	case KindURL:
		return "url"
	case KindHost:
		return "host"
	case KindSHA:
		return "sha"
	}
	return "unknown"
}

// Match is one target found on screen. The embedded Resolution is set for
// KindPath only.
type Match struct {
	Kind Kind
	// Start and End are byte offsets into the scanned text, covering what the
	// user sees: wrappers, quotes and any :line:col suffix are outside it.
	Start, End int
	// Text is the visible span with escapes and wrap markers removed.
	Text string
	Line int // 0 when the screen gave no location
	Col  int
	resolve.Resolution
}
