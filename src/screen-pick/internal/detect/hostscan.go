package detect

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Byte classes for the host and commit scanners.
const (
	cDigit uint8 = 1 << iota
	cAlpha
	cHex
	cLowHex
	cWord
	cUser // [A-Za-z0-9._-]
	cHost // [A-Za-z0-9.-]
	cPath // [A-Za-z0-9._~/-]
)

var classes = func() (t [256]uint8) {
	for c := range 256 {
		var k uint8
		switch {
		case c >= '0' && c <= '9':
			k = cDigit | cHex | cLowHex | cWord | cUser | cHost | cPath
		case c >= 'a' && c <= 'f':
			k = cAlpha | cHex | cLowHex | cWord | cUser | cHost | cPath
		case c >= 'A' && c <= 'F':
			k = cAlpha | cHex | cWord | cUser | cHost | cPath
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			k = cAlpha | cWord | cUser | cHost | cPath
		case c == '_':
			k = cWord | cUser | cPath
		case c == '.', c == '-':
			k = cUser | cHost | cPath
		case c == '~', c == '/':
			k = cPath
		}
		t[c] = k
	}
	return
}()

func isHex(c byte) bool       { return classes[c]&cHex != 0 }
func isWordClass(c byte) bool { return classes[c]&cWord != 0 }

// dotQuad matches (?:\.\d{1,3}){3} at i, the tail of a dotted quad, and
// returns where it ends. Only the last number may run into further digits.
func dotQuad(s string, i int) (int, bool) {
	for g := range 3 {
		if i >= len(s) || s[i] != '.' {
			return 0, false
		}
		k := digitsEnd(s, i+1)
		n := k - (i + 1)
		if n == 0 {
			return 0, false
		}
		if g < 2 {
			if n > 3 {
				return 0, false
			}
			i = k
		} else {
			i += 1 + min(n, 3)
		}
	}
	return i, true
}

// nextIPv4 finds the next \d{1,3}(?:\.\d{1,3}){3} at or after pos.
func nextIPv4(s string, pos int) (int, int) {
	n := len(s)
	for i := pos; i < n; {
		if !isDigit(s[i]) {
			i++
			continue
		}
		j := digitsEnd(s, i)
		if end, ok := dotQuad(s, j); ok {
			return max(i, j-3), end
		}
		i = j
	}
	return -1, -1
}

// nextIPv6Br finds the next \[([0-9A-Fa-f:.]+)\](?::\d{1,5})? at or after pos.
// close is where the bracketed text ends.
func nextIPv6Br(s string, pos int) (a, b, close int) {
	for pos < len(s) {
		k := strings.IndexByte(s[pos:], '[')
		if k < 0 {
			break
		}
		a = pos + k
		e := a + 1
		for e < len(s) && (isHex(s[e]) || s[e] == ':' || s[e] == '.') {
			e++
		}
		if e > a+1 && e < len(s) && s[e] == ']' {
			b = e + 1
			if b < len(s) && s[b] == ':' {
				j := b + 1
				for j < len(s) && j-b-1 < 5 && isDigit(s[j]) {
					j++
				}
				if j > b+1 {
					b = j
				}
			}
			return a, b, e
		}
		pos = a + 1
	}
	return -1, -1, -1
}

// nextIPv6 finds the next
// [0-9A-Fa-f]{0,4}(?::[0-9A-Fa-f]{0,4}){2,7}(?:(?:\.\d{1,3}){3})? at or after
// from. Every match starts at a run of hex digits touching a colon, so the
// colons are the triggers.
func nextIPv6(s string, from int) (int, int) {
	n := len(s)
	for pos := from; pos < n; {
		k := strings.IndexByte(s[pos:], ':')
		if k < 0 {
			break
		}
		j := pos + k
		i, groups := j, 0
		for groups < 7 && i < n && s[i] == ':' {
			i++
			for h := 0; h < 4 && i < n && isHex(s[i]); h++ {
				i++
			}
			groups++
		}
		if groups < 2 {
			pos = j + 1
			continue
		}
		st := j
		for h := 0; h < 4 && st > from && isHex(s[st-1]); h++ {
			st--
		}
		if end, ok := dotQuad(s, i); ok {
			i = end
		}
		return st, i
	}
	return -1, -1
}

// shaLen reports the lengths that make a word an abbreviated or full hash.
func shaLen(n int) bool { return n >= 7 && n <= 16 || n == 40 }

// nextSHA finds the next \b(?:[0-9a-f]{40}|[0-9a-f]{7,16})\b at or after pos:
// a whole word of lower-case hex digits of those lengths.
func nextSHA(s string, pos int) (int, int) {
	n := len(s)
	for i := pos; i < n; {
		if !isWordClass(s[i]) {
			i++
			continue
		}
		j := i
		for j < n && classes[s[j]]&cLowHex != 0 {
			j++
		}
		if j >= n || !isWordClass(s[j]) {
			if shaLen(j - i) {
				return i, j
			}
			i = j
			continue
		}
		for j < n && isWordClass(s[j]) {
			j++
		}
		i = j
	}
	return -1, -1
}

// nextSCP finds the next \b[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[A-Za-z0-9._~/-]+ at
// or after from. A match is fixed by its @: the user and host runs around it
// are maximal, and only the start can slide forward to the first word boundary.
func nextSCP(s string, from int) (int, int) {
	n := len(s)
	for pos := from; pos < n; {
		k := strings.IndexByte(s[pos:], '@')
		if k < 0 {
			break
		}
		at := pos + k
		pos = at + 1
		lo := at
		for lo > from && classes[s[lo-1]]&cUser != 0 {
			lo--
		}
		h := at + 1
		for h < n && classes[s[h]]&cHost != 0 {
			h++
		}
		if lo == at || h == at+1 || h >= n || s[h] != ':' {
			continue
		}
		e := h + 1
		for e < n && classes[s[e]]&cPath != 0 {
			e++
		}
		if e == h+1 {
			continue
		}
		for st := lo; st < at; st++ {
			if (st > 0 && isWordClass(s[st-1])) != isWordClass(s[st]) {
				return st, e
			}
		}
	}
	return -1, -1
}

// nextSCPBare finds the next
// \b(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}:[A-Za-z0-9._~/-]+\.git\b at or after from.
// Like nextSCP it is driven by the colon: the dotted name before it and the
// path after it are maximal runs.
func nextSCPBare(s string, from int) (int, int) {
	n := len(s)
	for pos := from; pos < n; {
		k := strings.IndexByte(s[pos:], ':')
		if k < 0 {
			break
		}
		q := pos + k
		pos = q + 1
		lo := q
		for lo > from && classes[s[lo-1]]&cHost != 0 {
			lo--
		}
		d := strings.LastIndexByte(s[lo:q], '.')
		if d < 0 {
			continue
		}
		d += lo
		if q-d-1 < 2 {
			continue
		}
		letters := true
		for i := d + 1; i < q; i++ {
			if classes[s[i]]&cAlpha == 0 {
				letters = false
				break
			}
		}
		if !letters {
			continue
		}
		r := q + 1
		for r < n && classes[s[r]]&cPath != 0 {
			r++
		}
		end := -1
		for p := r - 4; p >= q+2; p-- {
			if s[p] == '.' && s[p+1] == 'g' && s[p+2] == 'i' && s[p+3] == 't' && (p+4 >= n || !isWordClass(s[p+4])) {
				end = p + 4
				break
			}
		}
		if end < 0 {
			continue
		}
		dd := -1
		for i := d - 1; i >= lo; i-- {
			if s[i] == '.' && s[i+1] == '.' {
				dd = i
				break
			}
		}
		for st := max(lo, dd+1); st < d; st++ {
			if s[st] != '.' && (st > 0 && isWordClass(s[st-1])) != isWordClass(s[st]) {
				return st, end
			}
		}
	}
	return -1, -1
}

// opaqueStart returns where a scheme word that ends at the colon at c begins,
// or -1 when none does. Matching is case-insensitive the way a regexp's (?i)
// is, so a long s (U+017F) stands for s.
func opaqueStart(s string, c int) int {
	for _, w := range opaqueWords {
		j := c
		for i := len(w) - 1; i >= 0; i-- {
			switch {
			case j > 0 && s[j-1]|0x20 == w[i]:
				j--
			case w[i] == 's' && j >= 2 && s[j-2] == 0xC5 && s[j-1] == 0xBF:
				j -= 2
			default:
				j = -1
			}
			if j < 0 {
				break
			}
		}
		if j < 0 {
			continue
		}
		first := s[j] != 0xC5
		if (j > 0 && isWordClass(s[j-1])) != first {
			return j
		}
		return -1
	}
	return -1
}

var opaqueWords = [...]string{"mailto", "tel", "sms", "magnet", "geo", "urn", "xmpp", "sip", "sips", "bitcoin"}

func opaqueStop(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\f', '\r', '<', '>', '"', '\'', '`':
		return true
	}
	return false
}

// nextOpaque finds the next
// (?i)\b(?:mailto|tel|sms|magnet|geo|urn|xmpp|sip|sips|bitcoin):[^\s<>"'`]+ at
// or after from.
func nextOpaque(s string, from int) (int, int) {
	n := len(s)
	for pos := from; pos < n; {
		k := strings.IndexByte(s[pos:], ':')
		if k < 0 {
			break
		}
		c := pos + k
		pos = c + 1
		if c-from < 3 {
			continue
		}
		a := opaqueStart(s, c)
		if a < from {
			continue
		}
		b := c + 1
		for b < n && !opaqueStop(s[b]) {
			b++
		}
		if b > c+1 {
			return a, b
		}
	}
	return -1, -1
}

// cidrLen is the length of the /\d{1,2} prefix of s, or 0.
func cidrLen(s string) int {
	if len(s) < 2 || s[0] != '/' || !isDigit(s[1]) {
		return 0
	}
	if len(s) > 2 && isDigit(s[2]) {
		return 3
	}
	return 2
}

func isLetterOrNumber(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

func isLetterNumberMark(r rune) bool { return isLetterOrNumber(r) || unicode.IsMark(r) }

// Flags a name match reports about its text.
const (
	nameDot  uint8 = 1 << iota // holds a dot, so more than one label
	nameWide                   // holds a non-ASCII byte
)

// nameStartWidth is the width of the letter or number starting s, or 0.
func nameStartWidth(s string) int {
	if c := s[0]; c < utf8.RuneSelf {
		if classes[c]&(cDigit|cAlpha) != 0 {
			return 1
		}
		return 0
	}
	if r, size := utf8.DecodeRuneInString(s); isLetterOrNumber(r) {
		return size
	}
	return 0
}

// nextName finds the next dotted run of labels at or after pos, a label being
// letters and numbers with marks and inner hyphens:
// [\p{L}\p{N}](?:[\p{L}\p{N}\p{M}-]*[\p{L}\p{N}\p{M}])?(?:\.label)*.
func nextName(s string, pos int) (a, b int, flags uint8) {
	n := len(s)
	for i := pos; i < n; {
		size := 1
		if c := s[i]; c < utf8.RuneSelf {
			if classes[c]&(cDigit|cAlpha) == 0 {
				i++
				continue
			}
		} else {
			var r rune
			r, size = utf8.DecodeRuneInString(s[i:])
			if !isLetterOrNumber(r) {
				i += size
				continue
			}
			flags = nameWide
		}
		e := i + size
		for {
			last := e
			for e < n {
				c := s[e]
				if c < utf8.RuneSelf {
					if classes[c]&(cDigit|cAlpha) != 0 {
						e++
						last = e
						continue
					}
					if c == '-' {
						e++
						continue
					}
					break
				}
				r, sz := utf8.DecodeRuneInString(s[e:])
				if !isLetterNumberMark(r) {
					break
				}
				flags |= nameWide
				e += sz
				last = e
			}
			if last+1 < n && s[last] == '.' {
				if w := nameStartWidth(s[last+1:]); w > 0 {
					if w > 1 {
						flags |= nameWide
					}
					flags |= nameDot
					e = last + 1 + w
					continue
				}
			}
			return i, last, flags
		}
	}
	return -1, -1, 0
}
