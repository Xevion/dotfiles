package detect

import (
	"os"
	"strings"
	"testing"

	"screenpick/internal/resolve"
)

// realisticScreen is 45 rows drawn from the testdata screens: a compiler
// session, a shell with a powerline prompt, and Go build output.
func realisticScreen(tb testing.TB) string {
	tb.Helper()
	var rows []string
	for _, name := range []string{"cargo-errors", "shell-powerline", "go-build-test"} {
		raw, err := os.ReadFile("testdata/screens/" + name + ".txt")
		if err != nil {
			tb.Fatal(err)
		}
		rows = append(rows, strings.SplitN(string(raw), "\n", 16)[:15]...)
	}
	return strings.Join(rows, "\n") + "\n"
}

func BenchmarkScreen(b *testing.B) {
	text := realisticScreen(b)
	b.SetBytes(int64(len(text)))
	b.Run("scan", func(b *testing.B) {
		for b.Loop() {
			ScanWidth(text, resolve.New(screensEnv()), 120)
		}
	})
}

func BenchmarkLexResolve(b *testing.B) {
	for _, tc := range []struct {
		name string
		text string
		env  func() resolve.Env
	}{
		{"screen", realisticScreen(b), screensEnv},
	} {
		b.Run(tc.name+"/lex", func(b *testing.B) {
			for b.Loop() {
				Lex(tc.text, 120)
			}
		})
		b.Run(tc.name+"/lex+3 resolves", func(b *testing.B) {
			for b.Loop() {
				l := Lex(tc.text, 120)
				for range 3 {
					l.Resolve(resolve.New(tc.env()))
				}
			}
		})
		l := Lex(tc.text, 120)
		b.Run(tc.name+"/resolve", func(b *testing.B) {
			for b.Loop() {
				l.Resolve(resolve.New(tc.env()))
			}
		})
	}
}
