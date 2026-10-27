package decode

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDecode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"plain", "/tmp/x", []string{"/tmp/x"}},
		{"backslash space", `My\ Documents/x`, []string{"My Documents/x", `My\ Documents/x`}},
		{"backslash parens", `\(draft\).txt`, []string{"(draft).txt", `\(draft\).txt`}},
		{"ansi-c newline", `'a'$'\n''b'`, []string{"a\nb", `'a'$'\n''b'`}},
		{"single quotes", `'My Documents'`, []string{"My Documents", `'My Documents'`}},
		{"quoted var", `"$HOME"/.bashrc`, []string{"$HOME/.bashrc", `"$HOME"/.bashrc`}},
		{"git octal", `\303\244.txt`, []string{"ä.txt", `\303\244.txt`}},
		{"json unicode", `\u00e9t\u00e9.txt`, []string{"été.txt", `\u00e9t\u00e9.txt`}},
		{"json surrogate pair", `\ud83d\ude00.txt`, []string{"😀.txt", `\ud83d\ude00.txt`}},
		{"json slashes", `\/tmp\/x`, []string{"/tmp/x", `\/tmp\/x`}},
		{"doubled backslashes", `C:\\Users\\x`, []string{`C:\Users\x`, `C:\\Users\\x`}},
		{"percent encoding", "My%20Documents/x", []string{"My Documents/x", "My%20Documents/x"}},
		{"plus is literal in a path", "a+b.txt", []string{"a+b.txt"}},
		{"drive path keeps backslashes", `C:\temp\new.txt`, []string{`C:\temp\new.txt`}},
		{"invalid percent left alone", "100%.txt", []string{"100%.txt"}},
		{"invalid octal left alone", `\9.txt`, []string{`\9.txt`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, Decode(tc.in)); diff != "" {
				t.Errorf("Decode(%q) (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}
