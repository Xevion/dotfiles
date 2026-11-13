package detect

import (
	"regexp"
	"screenpick/internal/decode"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// link is an OSC 8 hyperlink over a range of cleaned text.
type link struct {
	start, end int
	target     string
}

// clean strips escape sequences and soft-wrap markers. offs[i] is the index
// in text of cleaned byte i, so spans map back onto the screen.
//
// With width > 0, a wrap marker after a row narrower than width is read as a
// hard break: a real wrap fills the row, while full-screen programs draw with
// cursor moves that kitty records as wraps on every row.
func clean(text string, width int) (string, []int, []link) {
	var b strings.Builder
	offs := make([]int, 0, len(text)+1)
	var links []link
	open, target := -1, ""
	rowStart := 0
	for i := 0; i < len(text); {
		c := text[i]
		switch {
		case c == 0x1b && i+1 < len(text) && text[i+1] == '[':
			j := i + 2
			for j < len(text) && text[j] >= 0x20 && text[j] <= 0x3f {
				j++
			}
			if j >= len(text) || text[j] < 0x40 || text[j] > 0x7e {
				// Truncated: only the introducer is escape, what follows is text.
				i += 2
				break
			}
			if strings.IndexByte(cursorFinals, text[j]) >= 0 {
				// Cursor movement parts the text on either side of it.
				b.WriteByte(' ')
				offs = append(offs, i)
			}
			i = j + 1
		case c == 0x1b && i+1 < len(text) && text[i+1] == ']':
			end, next := len(text), len(text)
			for j := i + 2; j < len(text); j++ {
				if text[j] == 0x07 {
					end, next = j, j+1
					break
				}
				if text[j] == 0x1b && j+1 < len(text) && text[j+1] == '\\' {
					end, next = j, j+2
					break
				}
			}
			if rest, ok := strings.CutPrefix(text[i+2:end], "8;"); ok {
				_, uri, _ := strings.Cut(rest, ";")
				if open >= 0 && b.Len() > open {
					links = append(links, link{open, b.Len(), target})
				}
				open = -1
				if uri != "" {
					open, target = b.Len(), uri
				}
			}
			i = next
		case c == 0x1b:
			i = skipEscape(text, i)
		case c == '\r' || c == 0:
			// kitty marks a soft wrap with CR (launch) or NUL (hints), each
			// followed by the newline it introduced.
			hard := width > 0 && ansi.StringWidth(text[rowStart:i]) < width
			i++
			if i < len(text) && text[i] == '\n' {
				if hard {
					b.WriteByte('\n')
					offs = append(offs, i)
				}
				i++
			} else {
				// A lone CR overwrites the line, so what follows is not a
				// continuation of what came before.
				b.WriteByte(' ')
				offs = append(offs, i-1)
			}
			rowStart = i
		case c == '\n':
			b.WriteByte(c)
			offs = append(offs, i)
			i++
			rowStart = i
		default:
			b.WriteByte(c)
			offs = append(offs, i)
			i++
		}
	}
	offs = append(offs, len(text))
	return b.String(), offs, links
}

// cursorFinals are the CSI final bytes that move the cursor or edit cells.
const cursorFinals = "ABCDEFGHdfX@P"

// skipEscape returns the index after the non-CSI, non-OSC escape sequence at
// text[i]. It consumes intermediates and one final byte, as in ESC ( B, but
// leaves alone anything that is not a plausible final, so a stray ESC never
// takes the first character of the text after it.
func skipEscape(text string, i int) int {
	j := i + 1
	for j < len(text) && strings.IndexByte(" #%()*+-.", text[j]) >= 0 {
		j++
	}
	if j >= len(text) {
		return j
	}
	f := text[j]
	if !(f >= 0x30 && f <= 0x5f || strings.IndexByte("cno|}~", f) >= 0) {
		return j
	}
	if j == i+1 && strings.IndexByte("P_^X", f) >= 0 {
		// DCS, APC, PM and SOS strings run to a string terminator.
		if end := strings.Index(text[j+1:], "\x1b\\"); end >= 0 {
			return j + 1 + end + 2
		}
	}
	return j + 1
}

// isWordRune keeps printable ASCII and non-ASCII letters, digits and marks.
// Other non-ASCII runes (box drawing, smart quotes, statusline glyphs) never
// belong to a target.
func isWordRune(r rune) bool {
	if r < utf8.RuneSelf {
		return r > ' ' && r != 0x7f
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

type word struct{ start, end int }

func splitWords(s string) []word {
	var words []word
	start := -1
	for i, r := range s {
		in := isWordRune(r)
		// A backslash-escaped space continues the word, except in a Windows path
		// where the backslash is a separator.
		if r == ' ' && start >= 0 && escaped(s, i) && !decode.IsWindowsPath(s[start:i]) {
			in = true
		}
		switch {
		case in && start < 0:
			start = i
		case !in && start >= 0:
			words = append(words, word{start, i})
			start = -1
		}
	}
	if start >= 0 {
		words = append(words, word{start, len(s)})
	}
	return words
}

// escaped reports whether s[i] is preceded by an odd run of backslashes.
func escaped(s string, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

// splitChars end a path segment unless escaped. They can still sit inside a
// path that exists on disk, which the unsplit candidates cover.
const splitChars = "()[]{}<>\"'`,;|="

func segments(s string, a, b int) [][2]int {
	var segs [][2]int
	start := a
	for i := a; i < b; i++ {
		c := s[i]
		if c == '\\' {
			i++
			continue
		}
		if c == '$' && i+1 < b && s[i+1] == '{' {
			if j := strings.IndexByte(s[i:b], '}'); j > 0 {
				i += j
				continue
			}
		}
		if strings.IndexByte(splitChars, c) >= 0 {
			if i > start {
				segs = append(segs, [2]int{start, i})
			}
			start = i + 1
		}
	}
	if b > start {
		segs = append(segs, [2]int{start, b})
	}
	return segs
}

// span is a stretch of cleaned text that might be a path. A strict span is
// accepted only when it exists on disk.
//
// pre is where the word holding an interior span began, or -1. Such a span
// starts at a root found inside a word, with whatever came before it unproven
// as junk, so the scanner checks it harder.
type span struct {
	start, end int
	strict     bool
	pre        int
}

type spanSet struct {
	s     string
	out   []span
	index map[[2]int]int
}

// add normalises [a,b) and records it, along with a git a/ or b/ stripped form
// and every reading that starts at a root inside it. wordStart is where the
// enclosing word began, for spotting a leading ….
func (ss *spanSet) add(a, b int, strict bool, wordStart int) {
	ss.addPre(a, b, strict, wordStart, -1)
}

func (ss *spanSet) addPre(a, b int, strict bool, wordStart, pre int) {
	ss.addInterior(a, b, strict)
	a, b = normalize(ss.s, a, b, wordStart)
	if a >= b || strict && simplyWrapped(ss.s[a:b]) {
		return
	}
	t := ss.s[a:b]
	// A reading with something shed from its end is the real one when it is
	// on disk or shaped like a path; the unshed text counts only if it exists.
	if k := fragmentStart(t); k > 0 {
		ss.put(a, a+k, strict, pre)
		strict = true
	}
	if n := trailingEscape(t); n > 0 && n < len(t) {
		ss.put(a, b-n, strict, pre)
		strict = true
	}
	if len(t) > 1 && t[len(t)-1] == '@' {
		ss.put(a, b-1, true, pre)
	}
	if len(t) > 1 && strings.IndexByte("$#", t[len(t)-1]) >= 0 {
		// A prompt's terminator: user@host:~/dir$
		ss.put(a, b-1, strict, pre)
		strict = true
	}
	if strings.IndexByte(t, '-') > 0 {
		if m := contextLine.FindStringSubmatch(t); m != nil {
			ss.put(a, a+len(m[1]), true, pre)
		}
	}
	ss.put(a, b, strict, pre)
	if len(t) > 2 && strings.IndexByte(gitSides, t[0]) >= 0 && t[1] == '/' {
		ss.put(a+2, b, strict, pre)
	}
}

// gitSides are the one-letter directory prefixes git puts on diff paths:
// a/ and b/, and the mnemonic i/ w/ c/ o/.
const gitSides = "abciwo"

var (
	contextLine = regexp.MustCompile(`^(.+\.[A-Za-z][A-Za-z0-9]*)-\d+-(?:\D|$)`)
	fragmentRe  = regexp.MustCompile(`#L\d+(?:C\d+)?(?:-L?\d+(?:C\d+)?)?$`)
	hostPortEnd = regexp.MustCompile(`:\d+$`)
)

// fragmentStart is the index of a #L12 line fragment ending t, or -1.
func fragmentStart(t string) int {
	if !strings.Contains(t, "#L") {
		return -1
	}
	if loc := fragmentRe.FindStringIndex(t); loc != nil {
		return loc[0]
	}
	return -1
}

// trailingEscape is how many bytes of literal escape end t: a backslash
// continuation, a \n \r \t \" \' printed inside a JSON or shell string, or
// a ^C echoed after the path.
// Windows paths keep their trailing backslash.
func trailingEscape(t string) int {
	if decode.IsWindowsPath(t) {
		return 0
	}
	n := 0
	for len(t) > n {
		u := t[:len(t)-n]
		switch {
		case len(u) >= 2 && u[len(u)-2] == '\\' && strings.IndexByte(`nrt"'`, u[len(u)-1]) >= 0 && !escaped(u, len(u)-2):
			n += 2
		case u[len(u)-1] == '\\' && !escaped(u, len(u)-1):
			n++
		case len(u) >= 3 && u[len(u)-2] == '^' && strings.IndexByte(caretPair, u[len(u)-1]) >= 0:
			n += 2
		default:
			return n
		}
	}
	return n
}

// addInterior adds the readings of [a,b) that start at a root inside it. They
// stand in for junk glued to the front of a path: caret notation such as ^C,
// a compiler flag, a prompt, a styling reset that left two words touching.
// The junk is unknown, so these readings count only when the path is on disk,
// except after a colon in a list of paths, where every element is trusted if
// the first was.
func (ss *spanSet) addInterior(a, b int, strict bool) {
	s := ss.s
	rooted := rootAt(s[a+junkLen(s[a:b]) : b])
	colonDone := false
	for i := a + 1; i < b; i++ {
		if c := s[i-1]; c == ' ' || c == '\t' || c == '\n' {
			return
		}
		if c := s[i]; s[i-1] != ':' && !mayStartRoot(c) {
			continue
		}
		prefix := s[a:i]
		t := s[i:b]
		if s[i-1] == ':' {
			if colonDone || !(unixRootAt(t) || junkLen(t) > 0) || isDriveColon(s, a, i-1) {
				continue
			}
			colonDone = true
			if rooted || junkLen(t) > 0 {
				ss.addPre(i, b, strict, -1, -1)
			} else {
				ss.addPre(i, b, true, -1, a)
			}
			continue
		}
		if rooted || !rootAt(t) || !interiorOK(prefix, t, s[i-1]) {
			continue
		}
		ss.addPre(i, b, true, -1, a)
	}
}

// isDriveColon reports whether the colon at s[c] follows a lone letter that
// starts the word, which reads as a drive.
func isDriveColon(s string, a, c int) bool {
	return c-1 == a && isASCIILetter(s[c-1])
}

// interiorOK says whether a root t, found after prefix and a byte prev, may
// start a path.
func interiorOK(prefix, t string, prev byte) bool {
	switch {
	case strings.Contains(prefix, "://"), strings.ContainsAny(prefix, "*?"),
		strings.Trim(prefix, ".") == "", hostPortEnd.MatchString(prefix):
		return false
	case t[0] == '/':
		return prev != '\\' && prev != '/' && !(len(prefix) == 1 && strings.IndexByte(gitSides, prefix[0]) >= 0)
	case t[0] == '.':
		return !isWordByte(prev) && prev != '.'
	case t[0] == '\\':
		return prev != '\\'
	case isASCIILetter(t[0]):
		return !isWordByte(prev)
	}
	return true
}

func (ss *spanSet) put(a, b int, strict bool, pre int) {
	key := [2]int{a, b}
	if i, ok := ss.index[key]; ok {
		o := &ss.out[i]
		o.strict = o.strict && strict
		if pre < 0 || !strict {
			o.pre = -1
		}
		return
	}
	ss.index[key] = len(ss.out)
	ss.out = append(ss.out, span{a, b, strict, pre})
}

var (
	userRoot  = regexp.MustCompile(`^~[A-Za-z_][\w.-]*/`)
	caretPair = "@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_?"
)

// rootAt reports whether t starts with something a path begins with: /, ~/,
// ~user/, ./, ../, $VAR/, a drive letter, a UNC share or %VAR%.
func rootAt(t string) bool {
	switch {
	case t == "":
		return false
	case t[0] == '/':
		return len(t) > 1 && t[1] != '/'
	case strings.HasPrefix(t, `\\`):
		return len(t) > 2 && t[2] != '\\'
	case t[0] == '%':
		return windowsVar.MatchString(t)
	}
	return decode.IsDrivePath(t) || unixRootAt(t)
}

// mayStartRoot is a cheap first-byte test for rootAt, which a regexp makes
// costly to run at every position of every word.
func mayStartRoot(c byte) bool {
	return strings.IndexByte(`/\~.$%`, c) >= 0 || isASCIILetter(c)
}

// unixRootAt is rootAt without the Windows forms.
func unixRootAt(t string) bool {
	switch {
	case t == "":
		return false
	case t[0] == '/':
		return len(t) > 1 && t[1] != '/'
	case t[0] == '~':
		return pathRoot.MatchString(t) || userRoot.MatchString(t)
	case t[0] == '.' || t[0] == '$':
		return pathRoot.MatchString(t)
	}
	return false
}

// junkLen is how many bytes of meaningless prefix t starts with, when a root
// follows it, else 0. Junk is caret notation (^C, ^[), an escaped quote, a
// one-letter flag (-I, -o), or combining marks and selectors left stranded
// next to a glyph.
func junkLen(t string) int {
	n := 0
	for n < len(t) {
		u := t[n:]
		var m int
		switch {
		case len(u) >= 2 && u[0] == '^' && strings.IndexByte(caretPair, u[1]) >= 0:
			m = 2
		case len(u) >= 2 && u[0] == '\\' && (u[1] == '"' || u[1] == '\''):
			m = 2
		case n == 0 && len(u) >= 2 && u[0] == '-' && isASCIILetter(u[1]):
			m = 2
		default:
			if r, size := utf8.DecodeRuneInString(u); r >= utf8.RuneSelf && unicode.IsMark(r) {
				m = size
			}
		}
		if m == 0 {
			break
		}
		n += m
		if rootAt(t[n:]) {
			return n
		}
	}
	return 0
}

func normalize(s string, a, b, wordStart int) (int, int) {
	if strings.HasPrefix(s[a:b], ".../") {
		a += 4
	} else if a == wordStart && a < b && s[a] == '/' {
		if r, _ := utf8.DecodeLastRuneInString(s[:a]); r == '…' {
			a++
		}
	}
	a += junkLen(s[a:b])
	for a < b && s[a] == '*' {
		a++
	}
	if i := colonCut(s[a:b]); i >= 0 {
		b = a + i
	}
	for a < b && strings.IndexByte("*.,;:!?", s[b-1]) >= 0 {
		b--
	}
	return a, b
}

// colonCut returns the first colon that is not a drive letter's, or -1. What
// follows it is a location, a URL remainder or prose, never more path.
func colonCut(t string) int {
	for i := 0; i < len(t); i++ {
		if t[i] != ':' {
			continue
		}
		drive := i >= 1 && isASCIILetter(t[i-1]) &&
			(i == 1 || strings.IndexByte(`\/ `, t[i-2]) >= 0) &&
			i+1 < len(t) && (t[i+1] == '\\' || t[i+1] == '/')
		if !drive {
			return i
		}
	}
	return -1
}

const (
	openWrappers  = "([{<'\"`"
	closeWrappers = ")]}>'\"`"
)

func trimWrappers(s string, a, b int) (int, int) {
	for a < b && strings.IndexByte(openWrappers, s[a]) >= 0 {
		a++
	}
	for a < b && strings.IndexByte(closeWrappers, s[b-1]) >= 0 {
		b--
	}
	return a, b
}

// simplyWrapped reports a word that is one quoted or bracketed run, like
// '/x' or (/x), whose inner span is the only reading worth keeping.
func simplyWrapped(w string) bool {
	if len(w) < 2 {
		return false
	}
	i := strings.IndexByte(openWrappers, w[0])
	if i < 0 || w[len(w)-1] != closeWrappers[i] {
		return false
	}
	return !strings.ContainsAny(w[1:len(w)-1], openWrappers[i:i+1]+closeWrappers[i:i+1])
}

// maxJoin is how many space-separated words one path may span.
const maxJoin = 5

func pathSpans(s string) []span {
	ss := &spanSet{s: s, index: map[[2]int]int{}}
	words := splitWords(s)
	for wi, w := range words {
		segs := segments(s, w.start, w.end)
		for _, sg := range segs {
			// A root right after a closing bracket continues the text before
			// it ({old => new}/main.rs), so only an existing file counts.
			glued := sg[0] > w.start && s[sg[0]] == '/' && strings.IndexByte(")]}", s[sg[0]-1]) >= 0
			ss.add(sg[0], sg[1], glued, w.start)
		}
		if len(segs) != 1 || segs[0] != [2]int{w.start, w.end} {
			ss.add(w.start, w.end, true, w.start)
			a, b := trimWrappers(s, w.start, w.end)
			ss.add(a, b, true, a)
		}

		// Unquoted names with spaces: joined words count only if they exist.
		slashy := strings.ContainsAny(s[w.start:w.end], `/\`)
		for wj := wi + 1; wj < len(words) && wj < wi+maxJoin; wj++ {
			if strings.Trim(s[words[wj-1].end:words[wj].start], " \t") != "" {
				break
			}
			last := s[words[wj].start:words[wj].end]
			slashy = slashy || strings.ContainsAny(last, `/\`)
			if !slashy && !strings.Contains(last, ".") {
				continue
			}
			ss.add(w.start, words[wj].end, true, w.start)
			a, b := trimWrappers(s, w.start, words[wj].end)
			ss.add(a, b, true, a)
		}
	}
	addQuoted(ss)
	return ss.out
}

// addQuoted records the contents of quotes that open after a separator and
// close before one, which may hold spaces.
func addQuoted(ss *spanSet) {
	s := ss.s
	for i := 0; i < len(s); i++ {
		q := s[i]
		if q != '\'' && q != '"' && q != '`' || escaped(s, i) {
			continue
		}
		if i > 0 && strings.IndexByte(" \t\n([{=:,", s[i-1]) < 0 {
			continue
		}
		for j := i + 1; j < len(s) && s[j] != '\n'; j++ {
			if s[j] != q || escaped(s, j) {
				continue
			}
			if j+1 < len(s) && strings.IndexByte(" \t\n)]},;:.", s[j+1]) < 0 {
				continue
			}
			if j > i+1 {
				ss.add(i+1, j, false, i+1)
			}
			i = j
			break
		}
	}
}

func isASCIILetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
