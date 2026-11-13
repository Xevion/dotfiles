package detect

import (
	"cmp"
	"regexp"
	"strconv"
	"testing"
)

// The regexps below are the reference the byte matchers must agree with.
var (
	refPathRoot    = regexp.MustCompile(`^(?:/|~[+-]?/|\./|\.\./|\$(?:\{\w+\}|[A-Za-z_]\w*)/)`)
	refWindowsVar  = regexp.MustCompile(`^%\w+%[\\/].`)
	refExt         = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,8}$`)
	refUserRoot    = regexp.MustCompile(`^~[A-Za-z_][\w.-]*/`)
	refContextLine = regexp.MustCompile(`^(.+\.[A-Za-z][A-Za-z0-9]*)-\d+-(?:\D|$)`)
	refFragment    = regexp.MustCompile(`#L\d+(?:C\d+)?(?:-L?\d+(?:C\d+)?)?$`)
	refPortEnd     = regexp.MustCompile(`:\d+$`)
	refLocation    = regexp.MustCompile(`^(?::(\d+)(?::(\d+))?|\((\d+)(?:,\s*(\d+))?\)|#L(\d+)(?:C(\d+))?|["']?(?:,\s*| on\s+|:)line\s+(\d+)|-(\d+)-(?:\D|$)|\|(\d+)(?: col (\d+))?\|)`)
)

func refLocationOf(rest string) (int, int) {
	m := refLocation.FindStringSubmatch(rest)
	if m == nil {
		return 0, 0
	}
	line, col := cmp.Or(m[1], m[3], m[5], m[7], m[8], m[9]), cmp.Or(m[2], m[4], m[6], m[10])
	l, _ := strconv.Atoi(line)
	c, _ := strconv.Atoi(col)
	return l, c
}

var matchInputs = []string{
	"", "/", "//", "~", "~/", "~+/", "~-/", "~*/", "~~/", "./", "../", "...", ".", "..", "$", "$/", "$A/", "$_a1/", "$1/", "${A}/", "${}/", "${A", "${A}", "${A-}/", "$A-/",
	"~user/x", "~u/", "~_/", "~1/", "~a.b-c_d/", "~a b/", "~a", "~/x",
	"%A%\\x", "%A%/x", "%A%/", "%A%\\", "%A%\\\n", "%%\\x", "%A B%\\x", "%_1%/é", "%A%\\\xff",
	"a", "ab", "a1", "a12345678", "a123456789", "1a", "a-b", "é", "aé", "A", "ZZZZZZZZZ",
	"x.rs-12-", "x.rs-12-\n", "x.rs-12-a", "x.rs-12-1", "x.rs-12", "x.rs-12-é", "a.b-1-c.d-2-e", "a.b.c-12-x", ".rs-1-x", "a.1-1-x", "a.b-1-", "a.b--1-x", "a\n.b-1-x", "a.b\n-1-x", "xx.go-9-\nrest",
	"#L1", "#L12C3", "#L12-20", "#L12-L20", "#L12C3-L20C4", "#L12C3-20C4", "#L", "#L1C", "#L1-", "#L1-L", "#L1x", "x#L1", "a#L1b#L2", "a#L1#L2", "#L1#L2", "p.rs#L10-L20",
	"host:80", "host:", ":80", "h:80x", "h:8:0", "a:b:99", "1:2:3",
	":12", ":12:4", ":12:", ":12:x", ":1:2:3", "(12)", "(12,4)", "(12, 4)", "(12,\t4)", "(12,)", "(12", "(12,4", "(12;4)", "(x)", "(1,\n2)",
	"#L12", "#L12C4", "#L12Cx", `", line 12`, `', line 7`, ", line 12", " on line 12", " on  line 3", " on\tline 3", " online 3", ":line 12", ": line 12", "line 12", `"line 12`, ", line", ", line x", ",line 1", ", \tline  9x",
	"-12-x", "-12-", "-12-1", "-12", "-1-é", "--1-x", "-12--",
	"|12|", "|12 col 4|", "|12 col |", "|12 col 4", "|12col 4|", "|12  col 4|", "|x|", "|12|x", "|99999999999999999999 col 99999999999999999999|",
	":99999999999999999999", "(99999999999999999999,5)",
}

func TestMatchersAgreeWithRegexps(t *testing.T) {
	check := func(s string) {
		t.Helper()
		if want := refPathRoot.FindString(s); pathRootLen(s) != len(want) {
			t.Errorf("pathRootLen(%q) = %d, regexp %q", s, pathRootLen(s), want)
		}
		if want := refWindowsVar.MatchString(s); isWindowsVar(s) != want {
			t.Errorf("isWindowsVar(%q) = %v, regexp %v", s, isWindowsVar(s), want)
		}
		if want := refExt.MatchString(s); isExtName(s) != want {
			t.Errorf("isExtName(%q) = %v, regexp %v", s, isExtName(s), want)
		}
		if want := refUserRoot.MatchString(s); isUserRoot(s) != want {
			t.Errorf("isUserRoot(%q) = %v, regexp %v", s, isUserRoot(s), want)
		}
		want := -1
		if m := refContextLine.FindStringSubmatch(s); m != nil {
			want = len(m[1])
		}
		if got := contextEnd(s); got != want {
			t.Errorf("contextEnd(%q) = %d, regexp %d", s, got, want)
		}
		want = -1
		if loc := refFragment.FindStringIndex(s); loc != nil {
			want = loc[0]
		}
		if got := fragmentIndex(s); got != want {
			t.Errorf("fragmentIndex(%q) = %d, regexp %d", s, got, want)
		}
		if want := refPortEnd.MatchString(s); endsInPort(s) != want {
			t.Errorf("endsInPort(%q) = %v, regexp %v", s, endsInPort(s), want)
		}
		wl, wc := refLocationOf(s)
		if gl, gc := location(s); gl != wl || gc != wc {
			t.Errorf("location(%q) = %d,%d, regexp %d,%d", s, gl, gc, wl, wc)
		}
	}
	for _, s := range matchInputs {
		check(s)
	}
}
