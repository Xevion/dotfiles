package detect

import (
	"strings"
	"testing"
	"time"

	"screenpick/internal/resolve"
)

// pathological words, each of which once cost time quadratic in its length
var pathologicalWords = []struct{ name, unit string }{
	{"roots inside a word", "/a"},
	{"colon-separated roots", "/a:"},
	{"tilde roots", "x~/a"},
	{"root after backslash", "a/b\\"},
	{"one-letter schemes", "a://"},
	{"schemes cut short", "ab://)"},
	{"unclosed quotes", "'a "},
	{"hosts with paths", "a.com/"},
	{"addresses with paths", "1.1.1.1/"},
	{"bracketed addresses", "[::1]/"},
	{"unclosed variables", "${"},
	{"unterminated strings", "\x1bP"},
}

func TestPathologicalInputsStayLinear(t *testing.T) {
	const size = 64 << 10
	for _, tc := range pathologicalWords {
		t.Run(tc.name, func(t *testing.T) {
			text := "x" + strings.Repeat(tc.unit, size/len(tc.unit))
			start := time.Now()
			Lex(text, 120).Resolve(resolve.New(screensEnv()))
			if d := time.Since(start); d > time.Second {
				t.Errorf("%d bytes took %v, want well under a second", len(text), d)
			}
		})
	}
}

func BenchmarkPathological(b *testing.B) {
	for _, tc := range pathologicalWords {
		text := "x" + strings.Repeat(tc.unit, (16<<10)/len(tc.unit))
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for b.Loop() {
				Lex(text, 120)
			}
		})
	}
}
