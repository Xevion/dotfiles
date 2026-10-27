package detect

import (
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
			for j < len(text) && (text[j] < 0x40 || text[j] > 0x7e) {
				j++
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
			i += 2
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
type span struct {
	start, end int
	strict     bool
}

type spanSet struct {
	s     string
	out   []span
	index map[[2]int]int
}

// add normalises [a,b) and records it, along with a git a/ or b/ stripped form.
// wordStart is where the enclosing word began, for spotting a leading ….
func (ss *spanSet) add(a, b int, strict bool, wordStart int) {
	a, b = normalize(ss.s, a, b, wordStart)
	if a >= b || strict && simplyWrapped(ss.s[a:b]) {
		return
	}
	ss.put(a, b, strict)
	if t := ss.s[a:b]; len(t) > 2 && (t[0] == 'a' || t[0] == 'b') && t[1] == '/' {
		ss.put(a+2, b, strict)
	}
}

func (ss *spanSet) put(a, b int, strict bool) {
	key := [2]int{a, b}
	if i, ok := ss.index[key]; ok {
		ss.out[i].strict = ss.out[i].strict && strict
		return
	}
	ss.index[key] = len(ss.out)
	ss.out = append(ss.out, span{a, b, strict})
}

func normalize(s string, a, b, wordStart int) (int, int) {
	if strings.HasPrefix(s[a:b], ".../") {
		a += 4
	} else if a == wordStart && a < b && s[a] == '/' {
		if r, _ := utf8.DecodeLastRuneInString(s[:a]); r == '…' {
			a++
		}
	}
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
			ss.add(sg[0], sg[1], false, w.start)
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
