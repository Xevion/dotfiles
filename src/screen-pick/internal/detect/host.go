package detect

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"screenpick/internal/resolve"
)

// opaqueRe finds the schemes that carry no "//", which a URL scan would miss.
var opaqueRe = regexp.MustCompile("(?i)\\b(?:mailto|tel|sms|magnet|geo|urn|xmpp|sip|sips|bitcoin):[^\\s<>\"'`]+")

// scpRe is user@host:path, where the host has a dot or the path names a repo.
var scpRe = regexp.MustCompile(`\b[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[A-Za-z0-9._~/-]+`)

// scpBareRe is host:path.git with no user, as git prints a remote.
var scpBareRe = regexp.MustCompile(`\b(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}:[A-Za-z0-9._~/-]+\.git\b`)

// bodyEnd returns where a URL body starting at i stops: at whitespace or a
// character that wraps URLs in text and never belongs to one.
func bodyEnd(s string, i int) int {
	for i < len(s) && strings.IndexByte(" \t\n\v\f\r<>\"'`", s[i]) < 0 {
		i++
	}
	return i
}

// schemeStart returns where the scheme ending at the "://" at j begins, or -1
// when what precedes it is not an RFC 3986 scheme worth taking for one.
func schemeStart(s string, j int) int {
	start := j
	for start > 0 && (isAlnum(s[start-1]) || strings.IndexByte("+.-", s[start-1]) >= 0) {
		start--
	}
	for p := j - 1; p > start; p-- {
		if !isAlnum(s[p]) && !isAlnum(s[p-1]) {
			start = p + 1
			break
		}
	}
	for start < j && !isAlnum(s[start]) {
		start++
	}
	if j-start < 2 || !isASCIILetter(s[start]) || !isAlnum(s[j-1]) {
		return -1
	}
	return start
}

func (sc *scanner) urlCands() []cand {
	s := sc.s
	var out []cand
	add := func(a, b int) {
		u := s[a:b]
		if p, ok := fileURLPath(u); ok {
			out = append(out, cand{KindPath, a, b, tierURL, sc.resolve(p, a)})
			return
		}
		out = append(out, cand{KindURL, a, b, tierURL, resolve.Resolution{}})
	}

	for i := 0; i < len(s); {
		j := strings.Index(s[i:], "://")
		if j < 0 {
			break
		}
		j += i
		i = j + 3
		a := schemeStart(s, j)
		end := bodyEnd(s, j+3)
		if a < 0 || end == j+3 {
			continue
		}
		b := trimURL(s, a, cutAtSymbol(s, a, end))
		if b <= j+3 {
			continue
		}
		add(a, b)
		i = b
	}
	for _, m := range opaqueRe.FindAllStringIndex(s, -1) {
		if b := trimURL(s, m[0], cutAtSymbol(s, m[0], m[1])); b > m[0] {
			add(m[0], b)
		}
	}
	for _, m := range scpRe.FindAllStringIndex(s, -1) {
		a, b := m[0], trimURL(s, m[0], m[1])
		if a > 0 && strings.IndexByte("/:@", s[a-1]) >= 0 || b < len(s) && strings.IndexByte("$#%", s[b]) >= 0 {
			continue
		}
		login, repo, _ := strings.Cut(s[a:b], ":")
		_, host, _ := strings.Cut(login, "@")
		if scpHost(host) && (strings.HasSuffix(repo, ".git") || strings.Contains(host, ".") && strings.Contains(repo, "/")) {
			out = append(out, cand{KindURL, a, b, tierURL, resolve.Resolution{}})
		}
	}
	for _, m := range scpBareRe.FindAllStringIndex(s, -1) {
		if m[0] == 0 || strings.IndexByte("/@:.-_", s[m[0]-1]) < 0 {
			out = append(out, cand{KindURL, m[0], m[1], tierURL, resolve.Resolution{}})
		}
	}
	return out
}

// scpHost accepts a name with letters in it, and for a dotted name an
// alphabetic last label, which turns version strings away.
func scpHost(h string) bool {
	if !strings.ContainsFunc(h, unicode.IsLetter) {
		return false
	}
	i := strings.LastIndexByte(h, '.')
	return i < 0 || i+1 < len(h) && alphaTLD(h[i+1:])
}

// fileURLPath returns the decoded local path of a file:// URL.
func fileURLPath(u string) (string, bool) {
	if len(u) < 7 || !strings.EqualFold(u[:7], "file://") {
		return "", false
	}
	rest := u[7:]
	if !strings.HasPrefix(rest, "/") {
		i := strings.IndexByte(rest, '/')
		if i < 0 {
			return "", false
		}
		rest = rest[i:]
	}
	if p, err := url.PathUnescape(rest); err == nil {
		return p, true
	}
	return rest, true
}

// cutAtSymbol ends a span at the first non-ASCII rune that is not a letter,
// digit or mark.
func cutAtSymbol(s string, a, b int) int {
	for i, r := range s[a:b] {
		if r >= utf8.RuneSelf && !isWordRune(r) {
			return a + i
		}
	}
	return b
}

// trimURL ends a URL at the first bracket it never opened, then drops
// trailing punctuation that belongs to the sentence around it.
func trimURL(s string, a, b int) int {
	var round, square, curly int
scan:
	for i := a; i < b; i++ {
		switch s[i] {
		case '(':
			round++
		case '[':
			square++
		case '{':
			curly++
		case ')':
			if round == 0 {
				b = i
				break scan
			}
			round--
		case ']':
			if square == 0 {
				b = i
				break scan
			}
			square--
		case '}':
			if curly == 0 {
				b = i
				break scan
			}
			curly--
		}
	}
	for b > a && strings.IndexByte(".,;:!?'\"*", s[b-1]) >= 0 {
		b--
	}
	return b
}

var (
	ipv4Re     = regexp.MustCompile(`\d{1,3}(?:\.\d{1,3}){3}`)
	ipv6Br     = regexp.MustCompile(`\[([0-9A-Fa-f:.]+)\](?::\d{1,5})?`)
	ipv6Re     = regexp.MustCompile(`[0-9A-Fa-f]{0,4}(?::[0-9A-Fa-f]{0,4}){2,7}(?:(?:\.\d{1,3}){3})?`)
	nameLabel  = `[\p{L}\p{N}](?:[\p{L}\p{N}\p{M}-]*[\p{L}\p{N}\p{M}])?`
	nameRe     = regexp.MustCompile(nameLabel + `(?:\.` + nameLabel + `)*`)
	cidrSuffix = regexp.MustCompile(`^/\d{1,2}`)
	// Abbreviated hashes run 7 to 16 digits; 32 is an MD5 or a machine ID.
	shaRe = regexp.MustCompile(`\b(?:[0-9a-f]{40}|[0-9a-f]{7,16})\b`)
)

func wordSet(list string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(list) {
		m[w] = true
	}
	return m
}

var (
	allTLD = wordSet(tldList)
	// Names that only resolve inside a network, or nowhere: always hosts.
	specialTLD = wordSet("local lan internal localhost test example invalid localdomain")
	// bareTLD are the endings trusted on a name with nothing else around it
	// (no scheme, port, www. or path). Everything else in the IANA list is
	// either a plausible file extension or an everyday word (obj.group,
	// self.name), so it needs one of those anchors.
	bareTLD = wordSet(`
		com net org edu gov mil int biz io dev app ai co me tv gg xyz cloud tech
		uk de fr nl be ch se dk fi es it pt eu ie us ca au nz jp cn kr tw hk sg ru ua br mx ar
		za il tr gr cz sk hu ro bg hr si lt lv ee fm ly ws vc li
	`)
	// extTLD are endings that a file name more likely carries than a host.
	extTLD = func() map[string]bool {
		m := wordSet("pl pm ps cm sv cr am ac in mo gd re st")
		for e := range knownExt {
			m[e] = true
		}
		return m
	}()
	// stemWords are file names that read as name.dev or name.app.
	stemWords = wordSet(`
		dockerfile makefile compose docker-compose config settings env environment application
		appsettings webpack vite rollup tsconfig package secrets credentials
	`)
	// keyWords precede a colon and a number in logs without naming a host.
	keyWords = wordSet(`
		port id pid ppid uid gid tid size count limit timeout line lines col column row error errno
		err code status ver version total time step index offset len length max min num number
		rev seq user group mode level rc exit
	`)
	// lowPorts are the privileged ports worth reading as a port on a bare name.
	lowPorts = wordSet("21 22 23 25 53 80 110 143 443 465 587 993 995")
	// codeBefore are words that put a dotted name in source code, not prose.
	codeBefore = wordSet("import package namespace extends implements")
	reverseDNS = wordSet("com org net io edu gov")
	// numberWords precede a dotted number that is a section or version.
	numberWords = wordSet("section sec chapter version ver release rev")
)

func isNameRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) }

// alphaTLD reports a plausible top-level label: letters only, or punycode.
func alphaTLD(t string) bool {
	if strings.HasPrefix(strings.ToLower(t), "xn--") {
		return len(t) > 4
	}
	n := 0
	for _, r := range t {
		if !unicode.IsLetter(r) {
			return false
		}
		n++
	}
	return n >= 2
}

// tldCase returns the lower-case form of t, or "" when t mixes case the way
// an identifier does (Foo.Bar, os.File).
func tldCase(t string) string {
	lower := strings.ToLower(t)
	if t != lower && t != strings.ToUpper(t) {
		return ""
	}
	return lower
}

func isIANA(lower string) bool {
	return allTLD[lower] || specialTLD[lower] || strings.HasPrefix(lower, "xn--")
}

func nonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return true
		}
	}
	return false
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// trimCJK takes a name out of the CJK text it is written into, which has no
// spaces to end it: an ASCII run touching CJK letters on its first or last
// label belongs to the surrounding sentence only up to the CJK side. cutL and
// cutR report which ends moved.
func trimCJK(s string, a, b int) (na, nb int, cutL, cutR bool) {
	na, nb = a, b
	if !strings.ContainsFunc(s[a:b], isCJK) {
		return
	}
	first := s[a:b]
	if i := strings.IndexByte(first, '.'); i >= 0 {
		first = first[:i]
	}
	if k := strings.LastIndexFunc(first, isCJK); k >= 0 {
		_, size := utf8.DecodeRuneInString(first[k:])
		if k+size < len(first) && first[k+size] < utf8.RuneSelf {
			na, cutL = a+k+size, true
		}
	}
	last := s[na:nb]
	lastStart := na
	if i := strings.LastIndexByte(last, '.'); i >= 0 {
		last, lastStart = last[i+1:], na+i+1
	}
	if k := strings.IndexFunc(last, isCJK); k > 0 && last[k-1] < utf8.RuneSelf {
		nb, cutR = lastStart+k, true
	}
	return
}

// nameStarts spots a name that begins where one can: not glued to a word, a
// path, a flag, an email address or a variable on its left.
func nameStarts(s string, a int) bool {
	if a == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:a])
	if r >= utf8.RuneSelf {
		return !isNameRune(r)
	}
	return !isAlnum(byte(r)) && strings.IndexByte(`@/\~%.-_$`, byte(r)) < 0
}

// nameEnds spots a name that ends where one can. A period closes a sentence
// unless a letter follows it.
func nameEnds(s string, b int) bool {
	if b >= len(s) {
		return true
	}
	if r, _ := utf8.DecodeRuneInString(s[b:]); r >= utf8.RuneSelf {
		return !isNameRune(r)
	}
	switch c := s[b]; {
	case isWordByte(c), c == '-', c == '@':
		return false
	case c == '.':
		if b+1 >= len(s) {
			return true
		}
		r, _ := utf8.DecodeRuneInString(s[b+1:])
		return !isNameRune(r)
	}
	return true
}

func wordBefore(s string, a int) string {
	i := a
	for i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
		i--
	}
	j := i
	for j > 0 && isASCIILetter(s[j-1]) {
		j--
	}
	if i == a {
		return ""
	}
	return strings.ToLower(s[j:i])
}

func wordAfter(s string, b int) string {
	i := b
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	j := i
	for j < len(s) && isASCIILetter(s[j]) {
		j++
	}
	if i == b {
		return ""
	}
	return strings.ToLower(s[i:j])
}

// pathEnd returns where the URL path starting at the slash at i stops, for a
// URL that began at a.
func pathEnd(s string, a, i int) int {
	return trimURL(s, a, cutAtSymbol(s, i, bodyEnd(s, i)))
}

// splitAuthority reads an optional :port after a name or address ending at b.
// ok is false for a port that is not one, or that is really a location.
func splitAuthority(s string, b int) (port string, end int, ok bool) {
	end = b
	if b >= len(s) || s[b] != ':' {
		return "", b, true
	}
	n := 0
	for b+1+n < len(s) && s[b+1+n] >= '0' && s[b+1+n] <= '9' {
		n++
	}
	if n == 0 {
		return "", b, true
	}
	port = s[b+1 : b+1+n]
	end = b + 1 + n
	if n > 5 || !validPort(port) || end+1 < len(s) && s[end] == ':' && s[end+1] >= '0' && s[end+1] <= '9' {
		return "", b, false
	}
	return port, end, true
}

// indexed spots a slice like x[::1], whose subscript is not an address.
func indexed(s string, a int) bool {
	return a >= 2 && s[a-1] == '[' && (isWordByte(s[a-2]) || s[a-2] == ')' || s[a-2] == ']')
}

func hostCands(s string) []cand {
	var out []cand
	add := func(a, b int) { out = append(out, cand{KindHost, a, b, tierHost, resolve.Resolution{}}) }
	addURL := func(a, b int) { out = append(out, cand{KindURL, a, b, tierHost, resolve.Resolution{}}) }

	for _, m := range ipv4Re.FindAllStringIndex(s, -1) {
		a, b := m[0], m[1]
		if a > 0 && (isWordByte(s[a-1]) || s[a-1] == '.') || net.ParseIP(s[a:b]) == nil {
			continue
		}
		port, end, ok := splitAuthority(s, b)
		if !ok || end == b && numberWords[wordBefore(s, a)] {
			continue
		}
		if end < len(s) && s[end] == '/' {
			if cidr := cidrSuffix.FindString(s[end:]); port == "" && cidr != "" && hostEnds(s, end+len(cidr)) {
				add(a, end+len(cidr))
			} else {
				addURL(a, pathEnd(s, a, end))
			}
			continue
		}
		if hostEnds(s, end) {
			add(a, end)
		}
	}
	for _, m := range ipv6Br.FindAllStringSubmatchIndex(s, -1) {
		a, b := m[0], m[1]
		_, port, _ := strings.Cut(s[m[3]:b], "]:")
		if net.ParseIP(s[m[2]:m[3]]) == nil || !validPort(port) || b < len(s) && isAlnum(s[b]) ||
			a > 0 && (isWordByte(s[a-1]) || s[a-1] == ')' || s[a-1] == ']') {
			continue
		}
		if b < len(s) && s[b] == '/' {
			addURL(a, pathEnd(s, a, b))
			continue
		}
		add(a, b)
	}
	for _, m := range ipv6Re.FindAllStringIndex(s, -1) {
		a, b := m[0], m[1]
		t := s[a:b]
		if a > 0 && (isWordByte(s[a-1]) || s[a-1] == ':' || s[a-1] == '.') ||
			b < len(s) && (isWordByte(s[b]) || s[b] == ':') || indexed(s, a) {
			continue
		}
		if (strings.Contains(t, "::") || strings.Count(t, ":") == 7) &&
			strings.ContainsFunc(t, unicode.IsDigit) && net.ParseIP(t) != nil {
			add(a, b)
		}
	}
	for _, m := range nameRe.FindAllStringIndex(s, -1) {
		a, b, cutL, cutR := trimCJK(s, m[0], m[1])
		if !cutL && !nameStarts(s, a) {
			continue
		}
		labels := strings.Split(s[a:b], ".")
		port, end, ok := splitAuthority(s, b)
		if !ok || !cutR && !nameEnds(s, end) {
			continue
		}
		switch {
		case port != "":
			if !portedName(labels, port) {
				continue
			}
			if end < len(s) && s[end] == '/' {
				addURL(a, pathEnd(s, a, end))
			} else {
				add(a, end)
			}
		case end < len(s) && s[end] == '/':
			e := pathEnd(s, a, end)
			if pathedName(labels) && (e > end+1 || bareName(s, a, b, labels)) && !fileLikePath(labels, s[end:e]) {
				addURL(a, e)
			}
		case bareName(s, a, b, labels):
			add(a, b)
		}
	}
	return out
}

// portedName judges a name that carries a port. A dotted name needs a plausible
// top-level label that is not a file extension; a single label (devbox:3000)
// is also a key in logs (id:12345), so it needs a port that looks deliberate.
func portedName(labels []string, port string) bool {
	if len(labels) == 1 {
		l := labels[0]
		if strings.EqualFold(l, "localhost") {
			return true
		}
		n, _ := strconv.Atoi(port)
		if len(l) < 2 || !isASCIILetter(l[0]) || nonASCII(l) || keyWords[strings.ToLower(l)] {
			return false
		}
		return n >= 1024 || lowPorts[port]
	}
	tld := labels[len(labels)-1]
	return alphaTLD(tld) && !extTLD[strings.ToLower(tld)]
}

// pathedName judges a name followed by a slash: a real top-level label, so
// docs.rs/serde reads as a URL while a.b.c/d does not.
func pathedName(labels []string) bool {
	if len(labels) == 1 {
		return strings.EqualFold(labels[0], "localhost")
	}
	lower := tldCase(labels[len(labels)-1])
	return lower != "" && isIANA(lower)
}

// fileLikePath spots CLAUDE.md/AGENTS.md: a name whose ending doubles as a file
// extension, followed by a path that names a file.
func fileLikePath(labels []string, path string) bool {
	if !extTLD[strings.ToLower(labels[len(labels)-1])] {
		return false
	}
	path, _, _ = strings.Cut(path, "?")
	path, _, _ = strings.Cut(path, "#")
	for _, comp := range strings.Split(path, "/") {
		if knownExt[strings.ToLower(ext(comp))] {
			return true
		}
	}
	return false
}

// bareName judges a name with nothing around it to vouch for it.
func bareName(s string, a, b int, labels []string) bool {
	if len(labels) == 1 {
		return strings.EqualFold(labels[0], "localhost") && nameNotCode(s, a, b)
	}
	lower := tldCase(labels[len(labels)-1])
	if lower == "" || !nameNotCode(s, a, b) {
		return false
	}
	first := strings.ToLower(labels[0])
	switch {
	case stemWords[strings.ToLower(labels[len(labels)-2])]:
		return false
	case len(labels) >= 3 && reverseDNS[first]:
		return false
	case specialTLD[lower]:
		return len(labels[len(labels)-2]) > 1
	case strings.HasPrefix(lower, "xn--") || bareTLD[lower] || homeArpa(labels):
		return true
	case nonASCII(lower) && allTLD[lower]:
		return true
	case len(labels) >= 3 && first == "www":
		return allTLD[lower]
	}
	return false
}

func homeArpa(labels []string) bool {
	n := len(labels)
	return n >= 3 && strings.EqualFold(labels[n-1], "arpa") && strings.EqualFold(labels[n-2], "home")
}

// nameNotCode rejects a name that looks like it belongs to source code or
// markup: called, indexed, imported or used as a link's text.
func nameNotCode(s string, a, b int) bool {
	if b < len(s) && (s[b] == '(' || s[b] == '[') || strings.HasPrefix(s[b:], "](") {
		return false
	}
	return !codeBefore[wordBefore(s, a)] && wordAfter(s, b) != "import"
}

// hostEnds allows a sentence-ending period after an address but not another
// dotted component.
func hostEnds(s string, b int) bool {
	if b >= len(s) {
		return true
	}
	if isWordByte(s[b]) {
		return false
	}
	return s[b] != '.' || b+1 >= len(s) || !isAlnum(s[b+1])
}

func validPort(p string) bool {
	if p == "" {
		return true
	}
	n, err := strconv.Atoi(p)
	return err == nil && n > 0 && n <= 65535
}

func shaCands(s string) []cand {
	var out []cand
	for _, m := range shaRe.FindAllStringIndex(s, -1) {
		a, b := m[0], m[1]
		t := s[a:b]
		if !strings.ContainsAny(t, "0123456789") || !strings.ContainsAny(t, "abcdef") {
			continue
		}
		if a > 0 && (s[a-1] == '-' || s[a-1] == ':') || b < len(s) && s[b] == '-' {
			continue
		}
		out = append(out, cand{KindSHA, a, b, tierSHA, resolve.Resolution{}})
	}
	return out
}

var opaqueSchemes = wordSet("mailto tel sms magnet geo urn xmpp sip sips bitcoin")

// schemeOf returns the URL scheme text starts with, or "".
func schemeOf(text string) string {
	i := strings.IndexByte(text, ':')
	if i < 2 {
		return ""
	}
	for j := 0; j < i; j++ {
		if c := text[j]; !isAlnum(c) && strings.IndexByte("+.-", c) < 0 || j == 0 && !isASCIILetter(c) {
			return ""
		}
	}
	if strings.HasPrefix(text[i:], "://") || opaqueSchemes[strings.ToLower(text[:i])] {
		return text[:i]
	}
	return ""
}

func splitHostPort(s string) (host, port string) {
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	return s, ""
}

func isIP(host string) bool {
	return net.ParseIP(strings.Trim(host, "[]")) != nil
}

// localName reports a host that only resolves on a local network.
func localName(host string) bool {
	lower := strings.ToLower(host)
	i := strings.LastIndexByte(lower, '.')
	return i < 0 || specialTLD[lower[i+1:]] || strings.HasSuffix(lower, ".home.arpa")
}

// WebURL returns the address that opens a host or URL match in a browser, or
// false when the match is not something a browser can open: a bare IP
// address, a network range, an scp-style remote.
func WebURL(m Match) (string, bool) {
	switch m.Kind {
	case KindURL:
		if schemeOf(m.Text) != "" {
			return m.Text, true
		}
		auth := m.Text
		if i := strings.IndexAny(auth, "/?#"); i >= 0 {
			auth = auth[:i]
		}
		if _, port := splitHostPort(auth); strings.Contains(auth, "@") || !validPort(port) || port != "" && strings.Trim(port, "0123456789") != "" {
			return "", false
		}
		return webScheme(auth) + "://" + m.Text, true
	case KindHost:
		host, port := splitHostPort(m.Text)
		if port == "" && (isIP(host) || strings.Contains(host, "/") || !strings.ContainsFunc(host, unicode.IsLetter)) {
			return "", false
		}
		return webScheme(m.Text) + "://" + m.Text, true
	}
	return "", false
}

// webScheme picks https for a public name and the TLS ports, http for the
// rest: local names and addresses rarely serve a certificate.
func webScheme(hostport string) string {
	host, port := splitHostPort(hostport)
	switch {
	case port == "443" || port == "8443":
		return "https"
	case port != "" || isIP(host) || localName(host):
		return "http"
	}
	return "https"
}
