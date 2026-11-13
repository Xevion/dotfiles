package detect

import (
	"cmp"
	"slices"
	"sort"
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

// scanner holds one resolution pass over a lexed screen.
type scanner struct {
	s string
	r *resolve.Resolver
	// dec holds decode.Decode of each span's text, parallel to the spans.
	dec [][]string
	// dirs are existing directories named in the text, in text order.
	dirs []screenDir
	// memo keeps resolutions by token and by how many screen directories sit
	// above it, which is all a resolution depends on.
	memo map[memoKey]resolve.Resolution
}

type memoKey struct {
	token string
	above int
}

type screenDir struct {
	pos int
	dir string
}

// fileRef is a candidate that names a local file through a file:// URL, whose
// resolution waits for a Resolver.
type fileRef struct {
	idx  int
	path string
}

// Lexed is the part of a scan that does not depend on the disk: the cleaned
// text, its path spans and every link, URL, host and commit candidate. It is
// immutable, so one Lexed can be resolved against several Resolvers, also
// concurrently.
type Lexed struct {
	s     string
	offs  *offsets
	spans []span
	// dec holds decode.Decode of each span's text, parallel to spans.
	dec   [][]string
	cands []cand
	files []fileRef
}

// Lex runs the resolver-independent half of a scan. width is as in ScanWidth.
func Lex(text string, width int) *Lexed {
	s, offs, links := clean(text, width)
	l := &Lexed{s: s, offs: offs, spans: pathSpans(s)}
	l.dec = make([][]string, len(l.spans))
	arena := make([]string, 0, len(l.spans)+len(l.spans)/4)
	for i, sp := range l.spans {
		n := len(arena)
		arena = decode.AppendDecode(arena, s[sp.start:sp.end])
		l.dec[i] = arena[n:len(arena):len(arena)]
	}
	for _, lk := range links {
		if p, ok := fileURLPath(lk.target); ok {
			l.cands = append(l.cands, cand{KindPath, lk.start, lk.end, tierLink, resolve.Resolution{}})
			l.files = append(l.files, fileRef{len(l.cands) - 1, p})
		} else {
			l.cands = append(l.cands, cand{KindURL, lk.start, lk.end, tierLink, resolve.Resolution{}})
		}
	}
	urls, files := urlCands(s)
	base := len(l.cands)
	l.cands = append(l.cands, urls...)
	for _, f := range files {
		l.files = append(l.files, fileRef{base + f.idx, f.path})
	}
	l.cands = append(l.cands, hostCands(s)...)
	l.cands = append(l.cands, shaCands(s)...)
	return l
}

// Scan returns every target in text, in screen order. Paths are resolved
// with direct checks only; nothing here searches.
func Scan(text string, r *resolve.Resolver) []Match {
	return ScanWidth(text, r, 0)
}

// ScanWidth is Scan for a screen width columns wide, which lets it tell real
// line wraps from rows a full-screen program merely drew next to each other.
func ScanWidth(text string, r *resolve.Resolver, width int) []Match {
	return Lex(text, width).Resolve(r)
}

// Resolve places the lexed paths with r and returns every target, in screen
// order.
func (l *Lexed) Resolve(r *resolve.Resolver) []Match {
	s := l.s
	sc := &scanner{s: s, r: r, dec: l.dec, memo: map[memoKey]resolve.Resolution{}}
	sc.findDirs(l.spans)

	cands := make([]cand, len(l.cands), len(l.cands)+len(l.spans))
	copy(cands, l.cands)
	for _, f := range l.files {
		cands[f.idx].res = sc.resolve(f.path, nil, cands[f.idx].start)
	}
	for i, sp := range l.spans {
		if c, ok := sc.pathCand(sp, l.dec[i]); ok {
			cands = append(cands, c)
		}
	}

	picked := pick(cands)
	out := make([]Match, 0, len(picked))
	for _, c := range picked {
		m := Match{
			Kind:       c.kind,
			Start:      l.offs.at(c.start),
			End:        l.offs.at(c.end-1) + 1,
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
	// picked stays sorted by start and disjoint, so the first entry ending
	// after c starts is the only one c could overlap.
	var picked []cand
	for _, c := range cands {
		i := sort.Search(len(picked), func(i int) bool { return picked[i].end > c.start })
		if i < len(picked) && picked[i].start < c.end {
			continue
		}
		picked = slices.Insert(picked, i, c)
	}
	return picked
}

// findDirs records existing directories whose rooted paths appear in the
// text, as bases for relative paths further down.
func (sc *scanner) findDirs(spans []span) {
	for i, sp := range spans {
		t := sc.s[sp.start:sp.end]
		d := sc.dec[i][0]
		if sp.pre >= 0 || !strings.HasPrefix(d, "/") && !strings.HasPrefix(d, "~") && !strings.HasPrefix(d, "$") {
			continue
		}
		if res := sc.lookup(t, sc.dec[i], 0, nil); res.Exists && res.IsDir {
			sc.dirs = append(sc.dirs, screenDir{sp.start, res.Path})
		}
	}
	slices.SortStableFunc(sc.dirs, func(a, b screenDir) int { return cmp.Compare(a.pos, b.pos) })
}

// resolve resolves t with the directories named above pos, nearest first.
// variants are t's decode.Decode readings, or nil to compute them.
func (sc *scanner) resolve(t string, variants []string, pos int) resolve.Resolution {
	k := sort.Search(len(sc.dirs), func(i int) bool { return sc.dirs[i].pos >= pos })
	var above []string
	if k > 0 {
		above = make([]string, k)
		for i := range above {
			above[i] = sc.dirs[k-1-i].dir
		}
	}
	return sc.lookup(t, variants, k, above)
}

// lookup is the memoized Resolver call: the screen directories above are
// fixed by their count, so equal tokens with equal counts resolve alike.
func (sc *scanner) lookup(t string, variants []string, k int, above []string) resolve.Resolution {
	key := memoKey{t, k}
	if res, ok := sc.memo[key]; ok {
		res.Alternatives = slices.Clone(res.Alternatives)
		return res
	}
	if variants == nil {
		variants = decode.Decode(t)
	}
	res := sc.r.ResolveDecoded(variants, above)
	sc.memo[key] = res
	res.Alternatives = slices.Clone(res.Alternatives)
	return res
}

func (sc *scanner) pathCand(sp span, dec []string) (cand, bool) {
	t := sc.s[sp.start:sp.end]
	d := dec[0]
	// Roots and dot-directories alone (/, ./, ~) are never worth offering.
	if !strings.ContainsFunc(d, unicode.IsLetter) {
		return cand{}, false
	}
	if sp.pre >= 0 && !sc.interiorPlausible(sp, d) {
		return cand{}, false
	}
	res := sc.resolve(t, dec, sp.start)
	if res.Exists {
		// A bare word naming a directory is too often just a word.
		if res.IsDir && !strings.ContainsAny(d, `/\`) {
			return cand{}, false
		}
		return cand{KindPath, sp.start, sp.end, tierPathExists, res}, true
	}
	if sp.strict || !looksLikeDecoded(t, d) || isCall(sc.s, sp, t) {
		return cand{}, false
	}
	return cand{KindPath, sp.start, sp.end, tierPathShape, res}, true
}

// interiorPlausible vets a reading that starts at a root inside a word, where
// the text before it might be a real directory rather than junk: a lone
// directory name (/etc in foo/etc) is too often just a word, and a prefix that
// is itself a directory means the whole word was the relative path.
func (sc *scanner) interiorPlausible(sp span, d string) bool {
	comps := strings.FieldsFunc(d, func(c rune) bool { return c == '/' || c == '\\' })
	if len(comps) == 0 || len(comps) < 2 && ext(comps[len(comps)-1]) == "" {
		return false
	}
	prefix := sc.s[sp.pre:sp.start]
	if res := sc.resolve(prefix, nil, sp.pre); res.Exists && res.IsDir {
		return false
	}
	// A prefix of plain directory names in front of a path-shaped word is that
	// word's relative path, which a coincidental absolute match must not hijack.
	if strings.Trim(prefix, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-/") == "" &&
		looksLikePath(sc.s[sp.pre:sp.end]) {
		return false
	}
	return true
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
	knownExtList = strings.Fields(`
		rs go py pyi js mjs cjs ts tsx jsx mts cts json jsonc json5 yaml yml toml ini cfg conf
		md mdx txt log csv tsv xml html htm css scss sass less svg png jpg jpeg gif webp ico bmp
		pdf c h cc cpp cxx hpp hh cs java kt kts scala groovy gradle rb php lua sh bash zsh fish
		nu ps1 psm1 bat cmd sql db sqlite lock tmpl tpl env diff patch zip tar gz tgz bz2 xz zst
		7z rar iso bin exe dll so dylib lib obj wasm mp3 mp4 mkv webm wav flac ogg mov avi swift
		mm vue svelte astro dart ex exs erl hs ml clj zig nim tf hcl proto graphql gql ipynb rst
		adoc tex bib el vim nix mk cmake properties plist reg lnk dmp pak sav dat cache pem crt
		key pub asc gpg age kdl ron jar war apk deb rpm msi appimage img vhd vhdx qcow2 iml
		sln csproj vcxproj pdb map snap bak orig rej swp tmp
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
	return looksLikeDecoded(raw, decode.Decode(raw)[0])
}

// looksLikeDecoded is looksLikePath for a raw token whose first decoded
// reading d is already known.
func looksLikeDecoded(raw, d string) bool {
	if raw == "" {
		return false
	}
	if c := raw[0]; c < utf8.RuneSelf && !isAlnum(c) && strings.IndexByte(`._~$%\/`, c) < 0 {
		return false
	}
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
	case isWindowsVar(d):
		return true
	}
	comps := strings.FieldsFunc(d, func(c rune) bool { return c == '/' || c == '\\' })
	if len(comps) == 0 {
		return false
	}
	last := comps[len(comps)-1]
	if isUserRoot(d) {
		// ~user/proj could be prose in a way ~/proj could not.
		return ext(last) != "" || len(comps) >= 3
	}
	if n := pathRootLen(d); n > 0 {
		// Every root but / counts as a component: ~/proj reads as a path,
		// /proj could be prose.
		if n == 1 {
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
	name = strings.TrimSuffix(name, "~")
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || strings.Trim(name[:i], ".") == "" {
		return ""
	}
	e := name[i+1:]
	if !isExtName(e) || e != strings.ToLower(e) && e != strings.ToUpper(e) {
		return ""
	}
	return e
}

func isAlnum(c byte) bool { return isASCIILetter(c) || c >= '0' && c <= '9' }

func isWordByte(c byte) bool { return isAlnum(c) || c == '_' }
