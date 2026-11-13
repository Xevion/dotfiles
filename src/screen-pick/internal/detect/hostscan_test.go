package detect

import (
	"fmt"
	"regexp"
	"testing"
)

var (
	refOpaqueRe  = regexp.MustCompile("(?i)\\b(?:mailto|tel|sms|magnet|geo|urn|xmpp|sip|sips|bitcoin):[^\\s<>\"'`]+")
	refSCPRe     = regexp.MustCompile(`\b[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[A-Za-z0-9._~/-]+`)
	refSCPBareRe = regexp.MustCompile(`\b(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}:[A-Za-z0-9._~/-]+\.git\b`)
	refIPv4Re    = regexp.MustCompile(`\d{1,3}(?:\.\d{1,3}){3}`)
	refIPv6Br    = regexp.MustCompile(`\[([0-9A-Fa-f:.]+)\](?::\d{1,5})?`)
	refIPv6Re    = regexp.MustCompile(`[0-9A-Fa-f]{0,4}(?::[0-9A-Fa-f]{0,4}){2,7}(?:(?:\.\d{1,3}){3})?`)
	refNameLabel = `[\p{L}\p{N}](?:[\p{L}\p{N}\p{M}-]*[\p{L}\p{N}\p{M}])?`
	refNameRe    = regexp.MustCompile(refNameLabel + `(?:\.` + refNameLabel + `)*`)
	refSHARe     = regexp.MustCompile(`\b(?:[0-9a-f]{40}|[0-9a-f]{7,16})\b`)
)

// hostPass pairs a hand-written scanner with the regexp it replaces.
type hostPass struct {
	name string
	re   *regexp.Regexp
	next func(s string, pos int) (a, b int)
}

var hostPasses = []hostPass{
	{"opaque", refOpaqueRe, nextOpaque},
	{"scp", refSCPRe, nextSCP},
	{"scpBare", refSCPBareRe, nextSCPBare},
	{"ipv4", refIPv4Re, nextIPv4},
	{"ipv6Bracket", refIPv6Br, func(s string, pos int) (int, int) {
		a, b, _ := nextIPv6Br(s, pos)
		return a, b
	}},
	{"ipv6", refIPv6Re, nextIPv6},
	{"name", refNameRe, func(s string, pos int) (int, int) {
		a, b, _ := nextName(s, pos)
		return a, b
	}},
	{"sha", refSHARe, nextSHA},
}

func allMatches(next func(string, int) (int, int), s string) [][]int {
	var out [][]int
	for pos := 0; pos < len(s); {
		a, b := next(s, pos)
		if a < 0 {
			break
		}
		out = append(out, []int{a, b})
		pos = b
	}
	return out
}

func sameMatches(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i][0] != b[i][0] || a[i][1] != b[i][1] {
			return false
		}
	}
	return true
}

// checkPasses reports every scanner whose matches differ from its regexp's.
func checkPasses(s string) error {
	for _, p := range hostPasses {
		want := p.re.FindAllStringIndex(s, -1)
		got := allMatches(p.next, s)
		if !sameMatches(want, got) {
			return fmt.Errorf("%s on %q:\n regexp %v\n    new %v", p.name, s, want, got)
		}
	}
	for _, m := range refIPv6Br.FindAllStringSubmatchIndex(s, -1) {
		a, b, c := -1, -1, -1
		for pos := 0; pos < len(s); {
			a, b, c = nextIPv6Br(s, pos)
			if a < 0 || a >= m[0] {
				break
			}
			pos = b
		}
		if a != m[0] || b != m[1] || a+1 != m[2] || c != m[3] {
			return fmt.Errorf("ipv6Bracket group on %q: regexp %v, new %d %d %d", s, m, a, b, c)
		}
	}
	return nil
}

var hostPieces = []string{
	"example.com", "Example.COM", "www.example.com", "a.b.c", "a.b", "foo.bar.baz.qux", "xn--bcher-kva.example", "xn--", "例え.jp", "日本語.com", "日本a.b", "a.日本", "naïve.fr", "é.é", "①②.com", "²x.y", "x²", "ａｂ.com", "e\u0301.com", "\u0301a.b",
	"localhost", "localhost:3000", "LocalHost", "devbox:3000", "id:12345", "host:22", "host:99999", "host:0", "host:65536", "a.b:80:81",
	"10.0.0.1", "10.0.0.1:80", "255.255.255.255", "256.1.1.1", "1.2.3", "1.2.3.4.5", "12345.1.1.1", "1.2.3.12345", "1.2.3.4/24", "1.2.3.4/123", "1.2.3.4/", "0.0.0.0", "1.1.1.1.", "1..1.1", "01.02.03.04", "section 1.2.3.4",
	"::1", "::", ":::", "fe80::1", "fe80::1%eth0", "1:2:3:4:5:6:7:8", "1:2:3:4:5:6:7:8:9", "1:2:3:4:5:6:7:8:9:a", "::ffff:1.2.3.4", "::1.2.3", "abcde::", "12345::1", "0:0:0:0:0:0:0:1", "::g", "a::b::c", "x[::1]", "[::1]", "[::1]:80", "[::1]:123456", "[::1]/x", "[fe80::1]", "[abc]", "[]", "[::1", "::1]", "([::1]:8080)", "a[1::2]", "[1.2.3.4]", "[1.2.3.4]:80",
	"git@github.com:o/r.git", "git@github.com:o/r", "github.com:o/r.git", "github.com:o/r.git.git", "github.com:o/r.gitx", "github.com:o/r.git-x", "github.com:o/r.git/", "a.b:x.git", "..a.b:x.git", "a..b.c:x.git", "a.b1:x.git", "a.bc:.git", "a.bc:x.git", "-a.bc:x.git", "-.com:x.git",
	"user@host:/p", "user@host:p/q", "u@h.com:a/b", "u@1.2.3:x/y.git", "a@b@c:d", "a@b:c@d:e", "@host:x.git", "a@:x", "a@b.co:", "a@b.co: x", "x/a@b.co:p/q", "$a@b.co:p/q", "a@b.co:p/q$", "ssh://git@host.com/o/r.git", "ssh://git@host.com:22/o/r.git",
	"mailto:a@b.co", "MAILTO:x", "MailTo:x", "ſms:123", "aſms:123", "sms:", "sms: x", "tel:+1", "xtel:+1", "sips:x", "sip:x", "sipsx:y", "urn:isbn:1", "magnet:?xt=urn", "bitcoin:1abc", "geo:1,2", "xmpp:a@b", "xmp:a", "mailto:<x>", "mailto:'x'", "tel::", "a:mailto:b", "TEL:1\x0b2",
	"https://example.com/a", "http://x", "ftp://h/p", "file:///etc/hosts", "file://host/x%20y", "FILE:///a", "://x", "a://b", "1a://b", "a+b://c", "a.-b://c", "x://(y)z", "http://a/(b)c)", "http://a/b.", "http://a/b,", "http://a/日本",
	"9c8d7e6", "9c8d7e", "9c8d7e6f", "abcdef1234567890", "abcdef12345678901", "3f2a1b49c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3", "3f2a1b49c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4f30", "3F2A1B4", "1234567", "abcdefg", "deadbeef", "DEADBEEF", "deadBEEF", "-deadbeef", "deadbeef-", ":deadbeef", "deadbeef:", "dead_beef", "_deadbeef", "0123456789abcdef0123456789abcdef", "0123456789abcdef0123456789abcdef01234567",
	"(", ")", "[", "]", "{", "}", "<", ">", "\"", "'", "`", ",", ";", ":", ".", "-", "_", "/", "\\", "@", "$", "#", "%", "~", "*", "?", "=", "+", "&", "|", "!",
	" ", "  ", "\t", "\n", "\x0b", "\x0c", "\r", "\x00", "\xff", "\xc3", "\xe3\x81", "é", "日", "…", "│", "\u200b", "😀",
	"Host: ", "import ", "package ", "see ", "version ", "section ", "http", "git", "com", "dev", "pid:", "port:",
}

func TestHostPassesAgreeWithRegexps(t *testing.T) {
	for _, s := range hostPieces {
		if err := checkPasses(s); err != nil {
			t.Fatal(err)
		}
	}
}

func fuzzSeeds(f *testing.F) {
	for _, s := range hostPieces {
		f.Add(s)
	}
}

func FuzzHostPasses(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, s string) {
		if err := checkPasses(s); err != nil {
			t.Fatal(err)
		}
	})
}
