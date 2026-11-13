package detect

import "strconv"

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isSpace is the regexp \s class.
func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r' }

// digitsEnd returns the index after the run of digits starting at i.
func digitsEnd(t string, i int) int {
	for i < len(t) && isDigit(t[i]) {
		i++
	}
	return i
}

// envVarLen returns the length of a $NAME or ${NAME} prefix of t, or 0.
func envVarLen(t string) int {
	if len(t) < 2 || t[0] != '$' {
		return 0
	}
	if t[1] == '{' {
		i := 2
		for i < len(t) && isWordByte(t[i]) {
			i++
		}
		if i > 2 && i < len(t) && t[i] == '}' {
			return i + 1
		}
		return 0
	}
	if !isASCIILetter(t[1]) && t[1] != '_' {
		return 0
	}
	i := 2
	for i < len(t) && isWordByte(t[i]) {
		i++
	}
	return i
}

// pathRootLen returns the length of the root t starts with: /, ~/, ~+/, ~-/,
// ./, ../ or $VAR/, or 0 when it has none.
func pathRootLen(t string) int {
	if t == "" {
		return 0
	}
	switch t[0] {
	case '/':
		return 1
	case '~':
		i := 1
		if i < len(t) && (t[i] == '+' || t[i] == '-') {
			i++
		}
		if i < len(t) && t[i] == '/' {
			return i + 1
		}
	case '.':
		if len(t) >= 2 && t[1] == '/' {
			return 2
		}
		if len(t) >= 3 && t[1] == '.' && t[2] == '/' {
			return 3
		}
	case '$':
		if n := envVarLen(t); n > 0 && n < len(t) && t[n] == '/' {
			return n + 1
		}
	}
	return 0
}

// isWindowsVar reports a %VAR% followed by a separator and one more character.
func isWindowsVar(t string) bool {
	if len(t) == 0 || t[0] != '%' {
		return false
	}
	i := 1
	for i < len(t) && isWordByte(t[i]) {
		i++
	}
	if i == 1 || i+2 >= len(t) || t[i] != '%' || t[i+1] != '\\' && t[i+1] != '/' {
		return false
	}
	return t[i+2] != '\n'
}

// isExtName reports a plausible extension: a letter then up to eight letters
// or digits.
func isExtName(e string) bool {
	if len(e) == 0 || len(e) > 9 || !isASCIILetter(e[0]) {
		return false
	}
	for i := 1; i < len(e); i++ {
		if !isAlnum(e[i]) {
			return false
		}
	}
	return true
}

// isUserRoot reports ~user/ at the start of t.
func isUserRoot(t string) bool {
	if len(t) < 3 || t[0] != '~' || !isASCIILetter(t[1]) && t[1] != '_' {
		return false
	}
	i := 2
	for i < len(t) && (isWordByte(t[i]) || t[i] == '.' || t[i] == '-') {
		i++
	}
	return i < len(t) && t[i] == '/'
}

// contextEnd finds a grep context line, name.ext-12-, at the start of t, and
// returns where the name ends, or -1. The name is the longest that fits.
func contextEnd(t string) int {
	limit := len(t)
	for i := 0; i < len(t); i++ {
		if t[i] == '\n' {
			limit = i
			break
		}
	}
	for p := limit - 1; p >= 1; p-- {
		if t[p] != '.' || p+1 >= len(t) || !isASCIILetter(t[p+1]) {
			continue
		}
		e := p + 2
		for e < len(t) && isAlnum(t[e]) {
			e++
		}
		if e >= len(t) || t[e] != '-' {
			continue
		}
		f := digitsEnd(t, e+1)
		if f == e+1 || f >= len(t) || t[f] != '-' {
			continue
		}
		if f+1 >= len(t) || !isDigit(t[f+1]) {
			return e
		}
	}
	return -1
}

// fragmentTail matches the part of a #L12C3-L20C4 fragment after "#L", up to
// the end of t.
func fragmentTail(t string) bool {
	i := digitsEnd(t, 0)
	if i == 0 {
		return false
	}
	if i < len(t) && t[i] == 'C' {
		j := digitsEnd(t, i+1)
		if j == i+1 {
			return false
		}
		i = j
	}
	if i == len(t) {
		return true
	}
	if t[i] != '-' {
		return false
	}
	i++
	if i < len(t) && t[i] == 'L' {
		i++
	}
	j := digitsEnd(t, i)
	if j == i {
		return false
	}
	i = j
	if i < len(t) && t[i] == 'C' {
		j := digitsEnd(t, i+1)
		if j == i+1 {
			return false
		}
		i = j
	}
	return i == len(t)
}

// fragmentIndex is the index of the first #L12 line fragment that runs to the
// end of t, or -1.
func fragmentIndex(t string) int {
	for i := 0; i+1 < len(t); i++ {
		if t[i] == '#' && t[i+1] == 'L' && fragmentTail(t[i+2:]) {
			return i
		}
	}
	return -1
}

// endsInPort reports a trailing :digits.
func endsInPort(t string) bool {
	i := len(t)
	for i > 0 && isDigit(t[i-1]) {
		i--
	}
	return i < len(t) && i > 0 && t[i-1] == ':'
}

// location reads the position that follows a path: :12:4, (12,4), #L12C4,
// `", line 12`, ` on line 12`, the -12- of a grep context line, or |12 col 4|.
func location(rest string) (int, int) {
	line, col := locationDigits(rest)
	l, _ := strconv.Atoi(line)
	c, _ := strconv.Atoi(col)
	return l, c
}

func locationDigits(t string) (line, col string) {
	if t == "" {
		return
	}
	switch t[0] {
	case ':':
		if j := digitsEnd(t, 1); j > 1 {
			line = t[1:j]
			if j < len(t) && t[j] == ':' {
				if k := digitsEnd(t, j+1); k > j+1 {
					col = t[j+1 : k]
				}
			}
			return
		}
	case '(':
		if j := digitsEnd(t, 1); j > 1 {
			i := j
			if i < len(t) && t[i] == ',' {
				i++
				for i < len(t) && isSpace(t[i]) {
					i++
				}
				k := digitsEnd(t, i)
				if k == i {
					return "", ""
				}
				col = t[i:k]
				i = k
			}
			if i < len(t) && t[i] == ')' {
				return t[1:j], col
			}
			return "", ""
		}
	case '#':
		if len(t) >= 2 && t[1] == 'L' {
			if j := digitsEnd(t, 2); j > 2 {
				line = t[2:j]
				if j < len(t) && t[j] == 'C' {
					if k := digitsEnd(t, j+1); k > j+1 {
						col = t[j+1 : k]
					}
				}
				return
			}
		}
	case '-':
		if j := digitsEnd(t, 1); j > 1 && j < len(t) && t[j] == '-' && (j+1 >= len(t) || !isDigit(t[j+1])) {
			return t[1:j], ""
		}
	case '|':
		if j := digitsEnd(t, 1); j > 1 {
			i := j
			if len(t)-i >= 5 && t[i:i+5] == " col " {
				if k := digitsEnd(t, i+5); k > i+5 {
					col = t[i+5 : k]
					i = k
				}
			}
			if i < len(t) && t[i] == '|' {
				return t[1:j], col
			}
			return "", ""
		}
	}
	return lineKeyword(t)
}

// lineKeyword reads `line 12` after a quote and comma, " on", or a colon.
func lineKeyword(t string) (string, string) {
	i := 0
	if t[0] == '"' || t[0] == '\'' {
		i++
	}
	switch {
	case i < len(t) && t[i] == ',':
		i++
		for i < len(t) && isSpace(t[i]) {
			i++
		}
	case len(t)-i >= 3 && t[i:i+3] == " on":
		i += 3
		j := i
		for i < len(t) && isSpace(t[i]) {
			i++
		}
		if i == j {
			return "", ""
		}
	case i < len(t) && t[i] == ':':
		i++
	default:
		return "", ""
	}
	if len(t)-i < 4 || t[i:i+4] != "line" {
		return "", ""
	}
	i += 4
	j := i
	for i < len(t) && isSpace(t[i]) {
		i++
	}
	if i == j {
		return "", ""
	}
	k := digitsEnd(t, i)
	if k == i {
		return "", ""
	}
	return t[i:k], ""
}
