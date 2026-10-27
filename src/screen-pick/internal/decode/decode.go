// Package decode reads escaped and encoded path tokens back into names.
package decode

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Decode returns the distinct readings of an escaped or encoded token, most
// likely first, with the raw token last when it differs from them.
func Decode(raw string) []string {
	var dec string
	if IsWindowsPath(raw) {
		// Backslashes are separators here, so the only escape worth undoing is
		// the doubling a JSON or C string literal adds.
		dec = raw
		if allBackslashesPaired(raw) {
			dec = strings.ReplaceAll(raw, `\\`, `\`)
		}
	} else {
		dec = unescape(raw)
	}
	if dec == raw {
		return []string{raw}
	}
	return []string{dec, raw}
}

func IsDrivePath(s string) bool {
	return len(s) >= 3 && isASCIILetter(s[0]) && s[1] == ':' && (s[2] == '\\' || s[2] == '/')
}

func IsWindowsPath(s string) bool {
	return IsDrivePath(s) || strings.HasPrefix(s, `\\`)
}

func allBackslashesPaired(s string) bool {
	found := false
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			continue
		}
		if i+1 >= len(s) || s[i+1] != '\\' {
			return false
		}
		found = true
		i++
	}
	return found
}

// unescape undoes shell quoting, C and JSON string escapes, and percent
// encoding in one pass. Anything malformed is kept literally.
func unescape(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '$' && i+1 < len(s) && s[i+1] == '\'':
			if j := closingQuote(s, i+2, '\''); j >= 0 {
				b = append(b, unescapeC(s[i+2:j])...)
				i = j + 1
				continue
			}
		case c == '\'':
			if j := strings.IndexByte(s[i+1:], '\''); j >= 0 {
				b = append(b, s[i+1:i+1+j]...)
				i += j + 2
				continue
			}
		case c == '"':
			if j := closingQuote(s, i+1, '"'); j >= 0 {
				b = append(b, unescapeC(s[i+1:j])...)
				i = j + 1
				continue
			}
		case c == '\\':
			if out, n := escape(s[i:]); n > 0 {
				b = append(b, out...)
				i += n
				continue
			}
		case c == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]):
			b = append(b, unhex(s[i+1])<<4|unhex(s[i+2]))
			i += 3
			continue
		}
		b = append(b, s[i])
		i++
	}
	return string(b)
}

func unescapeC(s string) []byte {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == '\\' {
			if out, n := escape(s[i:]); n > 0 {
				b = append(b, out...)
				i += n
				continue
			}
		}
		b = append(b, s[i])
		i++
	}
	return b
}

// closingQuote finds the next unescaped q at or after from, or -1.
func closingQuote(s string, from int, q byte) int {
	for j := from; j < len(s); j++ {
		if s[j] == '\\' {
			j++
			continue
		}
		if s[j] == q {
			return j
		}
	}
	return -1
}

// escape decodes the backslash sequence at the start of s, returning the bytes
// and how much of s it consumed, or 0 when it is not a recognised escape.
func escape(s string) ([]byte, int) {
	if len(s) < 2 {
		return nil, 0
	}
	switch c := s[1]; {
	case strings.IndexByte(" ()[]{}'\"&;|<>$`!*?#~=,/\\", c) >= 0:
		return []byte{c}, 2
	case c == 'n':
		return []byte{'\n'}, 2
	case c == 't':
		return []byte{'\t'}, 2
	case c == 'r':
		return []byte{'\r'}, 2
	case c == 'u':
		r, ok := hex4(s[2:])
		if !ok {
			return nil, 0
		}
		n := 6
		if utf16.IsSurrogate(r) {
			if len(s) < 12 || s[6] != '\\' || s[7] != 'u' {
				return nil, 0
			}
			lo, ok := hex4(s[8:])
			if !ok {
				return nil, 0
			}
			r = utf16.DecodeRune(r, lo)
			n = 12
		}
		return utf8.AppendRune(nil, r), n
	case c == 'x':
		if len(s) >= 4 && isHex(s[2]) && isHex(s[3]) {
			return []byte{unhex(s[2])<<4 | unhex(s[3])}, 4
		}
	case c >= '0' && c <= '7':
		if len(s) >= 4 && isOctal(s[2]) && isOctal(s[3]) {
			return []byte{(c-'0')<<6 | (s[2]-'0')<<3 | (s[3] - '0')}, 4
		}
	}
	return nil, 0
}

func hex4(s string) (rune, bool) {
	if len(s) < 4 {
		return 0, false
	}
	var r rune
	for i := range 4 {
		if !isHex(s[i]) {
			return 0, false
		}
		r = r<<4 | rune(unhex(s[i]))
	}
	return r, true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func isOctal(c byte) bool { return c >= '0' && c <= '7' }

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	}
	return c - '0'
}

func isASCIILetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
