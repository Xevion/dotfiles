package detect

import (
	"cmp"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"screenpick/internal/decode"
	"screenpick/internal/resolve"
)

// Tiers decide which candidate keeps a stretch of text when several overlap.
const (
	tierLink = iota
	tierURL
	tierPathExists
	tierHost
	tierPathShape
	tierSHA
)

type cand struct {
	kind       Kind
	start, end int
	tier       int
	res        resolve.Resolution
}

// scanner holds one pass over cleaned text.
type scanner struct {
	s string
	r *resolve.Resolver
	// dirs are existing directories named in the text, in text order.
	dirs []screenDir
}

type screenDir struct {
	pos int
	dir string
}

// Scan returns every target in text, in screen order. Paths are resolved
// with direct checks only; nothing here searches.
func Scan(text string, r *resolve.Resolver) []Match {
	return ScanWidth(text, r, 0)
}

// ScanWidth is Scan for a screen width columns wide, which lets it tell real
// line wraps from rows a full-screen program merely drew next to each other.
func ScanWidth(text string, r *resolve.Resolver, width int) []Match {
	s, offs, links := clean(text, width)
	sc := &scanner{s: s, r: r}
	spans := pathSpans(s)
	sc.findDirs(spans)

	var cands []cand
	for _, l := range links {
		cands = append(cands, sc.linkCand(l))
	}
	cands = append(cands, sc.urlCands()...)
	cands = append(cands, hostCands(s)...)
	cands = append(cands, shaCands(s)...)
	for _, sp := range spans {
		if c, ok := sc.pathCand(sp); ok {
			cands = append(cands, c)
		}
	}

	picked := pick(cands)
	out := make([]Match, 0, len(picked))
	for _, c := range picked {
		m := Match{
			Kind:       c.kind,
			Start:      offs[c.start],
			End:        offs[c.end-1] + 1,
			Text:       s[c.start:c.end],
			Resolution: c.res,
		}
		if c.kind == KindPath {
			m.Line, m.Col = location(s[c.end:])
		}
		out = append(out, m)
	}
	return out
}

// pick keeps the best candidate for each stretch of text, in text order.
func pick(cands []cand) []cand {
	slices.SortStableFunc(cands, func(a, b cand) int {
		return cmp.Or(
			cmp.Compare(a.tier, b.tier),
			cmp.Compare(b.end-b.start, a.end-a.start),
			cmp.Compare(a.start, b.start),
		)
	})
	var picked []cand
	for _, c := range cands {
		if !slices.ContainsFunc(picked, func(p cand) bool { return c.start < p.end && p.start < c.end }) {
			picked = append(picked, c)
		}
	}
	slices.SortFunc(picked, func(a, b cand) int { return cmp.Compare(a.start, b.start) })
	return picked
}

// findDirs records existing directories whose rooted paths appear in the
// text, as bases for relative paths further down.
func (sc *scanner) findDirs(spans []span) {
	for _, sp := range spans {
		t := sc.s[sp.start:sp.end]
		d := decode.Decode(t)[0]
		if !strings.HasPrefix(d, "/") && !strings.HasPrefix(d, "~") && !strings.HasPrefix(d, "$") {
			continue
		}
		if res := sc.r.Resolve(t, nil); res.Exists && res.IsDir {
			sc.dirs = append(sc.dirs, screenDir{sp.start, res.Path})
		}
	}
	slices.SortStableFunc(sc.dirs, func(a, b screenDir) int { return cmp.Compare(a.pos, b.pos) })
}

// resolve resolves t with the directories named above pos, nearest first.
func (sc *scanner) resolve(t string, pos int) resolve.Resolution {
	var above []string
	for i := len(sc.dirs) - 1; i >= 0; i-- {
		if sc.dirs[i].pos < pos {
			above = append(above, sc.dirs[i].dir)
		}
	}
	return sc.r.Resolve(t, above)
}

func (sc *scanner) pathCand(sp span) (cand, bool) {
	t := sc.s[sp.start:sp.end]
	d := decode.Decode(t)[0]
	// Roots and dot-directories alone (/, ./, ~) are never worth offering.
	if !strings.ContainsFunc(d, unicode.IsLetter) {
		return cand{}, false
	}
	res := sc.resolve(t, sp.start)
	if res.Exists {
		// A bare word naming a directory is too often just a word.
		if res.IsDir && !strings.ContainsAny(d, `/\`) {
			return cand{}, false
		}
		return cand{KindPath, sp.start, sp.end, tierPathExists, res}, true
	}
	if sp.strict || !looksLikePath(t) || isCall(sc.s, sp, t) {
		return cand{}, false
	}
	return cand{KindPath, sp.start, sp.end, tierPathShape, res}, true
}

// isCall spots a dotted name used as a function, like console.log(x).
func isCall(s string, sp span, t string) bool {
	if sp.end >= len(s) || s[sp.end] != '(' || strings.ContainsAny(t, `/\`) {
		return false
	}
	line, _ := location(s[sp.end:])
	return line == 0
}

var (
	pathRoot     = regexp.MustCompile(`^(?:/|~/|\./|\.\./|\$(?:\{\w+\}|[A-Za-z_]\w*)/)`)
	windowsVar   = regexp.MustCompile(`^%\w+%[\\/].`)
	extPattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,8}$`)
	locationRe   = regexp.MustCompile(`^(?::(\d+)(?::(\d+))?|\((\d+)(?:,\s*(\d+))?\))`)
	knownExtList = strings.Fields(`
		rs go py pyi js mjs cjs ts tsx jsx mts cts json jsonc json5 yaml yml toml ini cfg conf
		md mdx txt log csv tsv xml html htm css scss sass less svg png jpg jpeg gif webp ico bmp
		pdf c h cc cpp cxx hpp hh cs java kt kts scala groovy gradle rb php lua sh bash zsh fish
		nu ps1 psm1 bat cmd sql db sqlite lock tmpl tpl env diff patch zip tar gz tgz bz2 xz zst
		7z rar iso bin exe dll so dylib lib obj wasm mp3 mp4 mkv webm wav flac ogg mov avi swift
		mm vue svelte astro dart ex exs erl hs ml clj zig nim tf hcl proto graphql gql ipynb rst
		adoc tex bib el vim nix mk cmake properties plist reg lnk dmp pak sav dat cache pem crt
		key pub asc gpg age kdl ron jar war apk deb rpm msi appimage img vhd vhdx qcow2 iml
		sln csproj vcxproj pdb map snap
	`)
	knownExt = func() map[string]bool {
		m := make(map[string]bool, len(knownExtList))
		for _, e := range knownExtList {
			m[e] = true
		}
		return m
	}()
)

// looksLikePath judges a token that is not on disk by its shape alone.
func looksLikePath(raw string) bool {
	if raw == "" {
		return false
	}
	if c := raw[0]; c < utf8.RuneSelf && !isAlnum(c) && strings.IndexByte(`._~$%\/`, c) < 0 {
		return false
	}
	d := decode.Decode(raw)[0]
	if !strings.ContainsFunc(d, unicode.IsLetter) || strings.Contains(d, "*") {
		return false
	}
	if strings.Contains(d, "?") && !strings.HasPrefix(d, `\\?\`) {
		return false
	}
	if strings.Contains(d, "@") && !strings.ContainsAny(d, `/\`) {
		return false
	}
	switch {
	case decode.IsDrivePath(d):
		return len(d) > 3
	case strings.HasPrefix(d, `\\`):
		return strings.Count(d, `\`) >= 3
	case windowsVar.MatchString(d):
		return true
	}
	comps := strings.FieldsFunc(d, func(c rune) bool { return c == '/' || c == '\\' })
	if len(comps) == 0 {
		return false
	}
	last := comps[len(comps)-1]
	if root := pathRoot.FindString(d); root != "" {
		// Every root but / counts as a component: ~/proj reads as a path,
		// /proj could be prose.
		if root == "/" {
			return ext(last) != "" || len(comps) >= 2
		}
		return ext(last) != "" || len(comps) >= 2 && last != "..." && last != "."
	}
	return knownExt[strings.ToLower(ext(last))]
}

// ext returns a name's extension, or "" when the name has none, is all stem
// (a dotfile), or mixes case the way an identifier does (exec.Cmd, os.File):
// real extensions are lower or upper case throughout.
func ext(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || strings.Trim(name[:i], ".") == "" {
		return ""
	}
	e := name[i+1:]
	if !extPattern.MatchString(e) || e != strings.ToLower(e) && e != strings.ToUpper(e) {
		return ""
	}
	return e
}

func location(rest string) (int, int) {
	m := locationRe.FindStringSubmatch(rest)
	if m == nil {
		return 0, 0
	}
	line, col := cmp.Or(m[1], m[3]), cmp.Or(m[2], m[4])
	l, _ := strconv.Atoi(line)
	c, _ := strconv.Atoi(col)
	return l, c
}

func isAlnum(c byte) bool { return isASCIILetter(c) || c >= '0' && c <= '9' }

func isWordByte(c byte) bool { return isAlnum(c) || c == '_' }

func (sc *scanner) linkCand(l link) cand {
	if p, ok := fileURLPath(l.target); ok {
		return cand{KindPath, l.start, l.end, tierLink, sc.resolve(p, l.start)}
	}
	return cand{KindURL, l.start, l.end, tierLink, resolve.Resolution{}}
}

var (
	urlRe = regexp.MustCompile("(?i)\\b(?:https?|ftp|ssh|git|wss?|file)://[^\\s<>\"'`]+")
	scpRe = regexp.MustCompile(`\b[A-Za-z0-9._-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}:[A-Za-z0-9._~-][A-Za-z0-9._~/-]*`)
)

func (sc *scanner) urlCands() []cand {
	s := sc.s
	var out []cand
	for _, m := range urlRe.FindAllStringIndex(s, -1) {
		a, b := m[0], cutAtSymbol(s, m[0], m[1])
		b = trimURL(s, a, b)
		u := s[a:b]
		if strings.HasSuffix(u, "://") {
			continue
		}
		if p, ok := fileURLPath(u); ok {
			out = append(out, cand{KindPath, a, b, tierURL, sc.resolve(p, a)})
			continue
		}
		out = append(out, cand{KindURL, a, b, tierURL, resolve.Resolution{}})
	}
	for _, m := range scpRe.FindAllStringIndex(s, -1) {
		a, b := m[0], trimURL(s, m[0], m[1])
		if a > 0 && strings.IndexByte("/:@", s[a-1]) >= 0 {
			continue
		}
		_, repo, _ := strings.Cut(s[a:b], ":")
		if strings.Contains(repo, "/") || strings.HasSuffix(repo, ".git") {
			out = append(out, cand{KindURL, a, b, tierURL, resolve.Resolution{}})
		}
	}
	return out
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

// trimURL drops trailing punctuation and closing brackets the URL never opened.
func trimURL(s string, a, b int) int {
	for b > a {
		u := s[a:b]
		switch c := u[len(u)-1]; {
		case strings.IndexByte(".,;:!?'\"*", c) >= 0:
			b--
		case c == ')' && strings.Count(u, "(") < strings.Count(u, ")"):
			b--
		case c == ']' && strings.Count(u, "[") < strings.Count(u, "]"):
			b--
		default:
			return b
		}
	}
	return b
}

var (
	ipv4Re     = regexp.MustCompile(`\d{1,3}(?:\.\d{1,3}){3}(?::\d{1,5}|/\d{1,2})?`)
	ipv6Br     = regexp.MustCompile(`\[([0-9A-Fa-f:.]+)\](?::\d{1,5})?`)
	ipv6Re     = regexp.MustCompile(`[0-9A-Fa-f]{0,4}(?::[0-9A-Fa-f]{0,4}){2,7}`)
	hostPortRe = regexp.MustCompile(`(?i)\b(localhost|(?:[a-z0-9-]+\.)+[a-z]{2,}):(\d{1,5})`)
	// Abbreviated hashes run 7 to 16 digits; 32 is an MD5 or a machine ID.
	shaRe = regexp.MustCompile(`\b(?:[0-9a-f]{40}|[0-9a-f]{7,16})\b`)
)

func hostCands(s string) []cand {
	var out []cand
	add := func(a, b int) { out = append(out, cand{KindHost, a, b, tierHost, resolve.Resolution{}}) }

	for _, m := range ipv4Re.FindAllStringIndex(s, -1) {
		a, b := m[0], m[1]
		if a > 0 && (isWordByte(s[a-1]) || s[a-1] == '.') || !hostEnds(s, b) {
			continue
		}
		addr, _, _ := strings.Cut(s[a:b], "/")
		ip, port, _ := strings.Cut(addr, ":")
		if net.ParseIP(ip) != nil && validPort(port) {
			add(a, b)
		}
	}
	for _, m := range ipv6Br.FindAllStringSubmatchIndex(s, -1) {
		_, port, _ := strings.Cut(s[m[3]:m[1]], "]:")
		if net.ParseIP(s[m[2]:m[3]]) != nil && validPort(port) && !(m[1] < len(s) && isAlnum(s[m[1]])) {
			add(m[0], m[1])
		}
	}
	for _, m := range ipv6Re.FindAllStringIndex(s, -1) {
		a, b := m[0], m[1]
		t := s[a:b]
		if a > 0 && (isWordByte(s[a-1]) || s[a-1] == ':' || s[a-1] == '.') ||
			b < len(s) && (isWordByte(s[b]) || s[b] == ':') {
			continue
		}
		if (strings.Contains(t, "::") || strings.Count(t, ":") == 7) &&
			strings.ContainsFunc(t, unicode.IsDigit) && net.ParseIP(t) != nil {
			add(a, b)
		}
	}
	for _, m := range hostPortRe.FindAllStringSubmatchIndex(s, -1) {
		a, b := m[0], m[1]
		host := s[m[2]:m[3]]
		if a > 0 && strings.IndexByte("@/.-", s[a-1]) >= 0 || b < len(s) && isAlnum(s[b]) {
			continue
		}
		if !strings.EqualFold(host, "localhost") && knownExt[strings.ToLower(host[strings.LastIndexByte(host, '.')+1:])] {
			continue
		}
		if validPort(s[m[4]:m[5]]) {
			add(a, b)
		}
	}
	return out
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
