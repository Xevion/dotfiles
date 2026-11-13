// Package resolve turns a path token, as printed, into the files it may name:
// across ranked base directories, Wine drive mappings and abbreviated forms.
package resolve

import (
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"screenpick/internal/decode"
)

// Source says where the directory a path resolved against came from.
type Source uint8

const (
	// SourceScreen is an existing directory whose path appears in the text
	// above the token.
	SourceScreen Source = iota + 1
	SourceProcess
	// SourceRoot is a project root found by walking up from another base.
	SourceRoot
	// SourceWindow is kitty's window cwd, which can lag behind the shell.
	SourceWindow
	SourceHome
	SourceWine
	SourceAbsolute
	// SourceIndex is a git-tracked file whose path ends with the token.
	SourceIndex
	// SourceSearch is a file found by walking the disk.
	SourceSearch
)

func (s Source) String() string {
	switch s {
	case SourceScreen:
		return "screen"
	case SourceProcess:
		return "process"
	case SourceRoot:
		return "root"
	case SourceWindow:
		return "window"
	case SourceHome:
		return "home"
	case SourceWine:
		return "wine"
	case SourceAbsolute:
		return "absolute"
	case SourceIndex:
		return "index"
	case SourceSearch:
		return "search"
	}
	return "unknown"
}

// Base is a directory relative paths may be resolved against.
type Base struct {
	Dir    string
	Source Source
}

// Candidate is one place a token resolved to.
type Candidate struct {
	Path   string
	Source Source
}

// Resolution is the outcome for one token. Without an existing reading, Path
// is where the token would live under the best base, and may be empty when
// nothing can place it (an unset variable, a UNC share).
type Resolution struct {
	Path   string
	Source Source
	Exists bool
	IsDir  bool
	// Expanded marks a path reached by widening abbreviated components.
	Expanded bool
	// Alternatives are other existing readings, best first.
	Alternatives []Candidate
}

// Env is everything a Resolver may consult. Nil functions fall back to the os
// package.
type Env struct {
	// Bases are directories for relative paths; their order within a Source
	// is kept.
	Bases    []Base
	Home     string
	Getenv   func(string) string
	Lstat    func(string) (fs.FileInfo, error)
	Readlink func(string) (string, error)
	ReadDir  func(string) ([]fs.DirEntry, error)
	// WinePrefixes are ranked prefix roots, each holding dosdevices/.
	WinePrefixes []string
}

// Resolver ranks Env's bases once and resolves tokens against them. It
// caches what it reads from disk, so it is not safe for concurrent use and
// should not outlive one screen.
type Resolver struct {
	env Env
	// static are the bases plus project roots and home, in rank order.
	static []Base
	stats  map[string]statResult
	dirs   map[string][]fs.DirEntry
}

type statResult struct {
	info fs.FileInfo
	err  error
}

var rootMarkers = []string{"go.mod", "Cargo.toml", "package.json"}

// New ranks bases as process, other, project roots, window, then home.
func New(env Env) *Resolver {
	if env.Getenv == nil {
		env.Getenv = os.Getenv
	}
	if env.Lstat == nil {
		env.Lstat = os.Lstat
	}
	if env.Readlink == nil {
		env.Readlink = os.Readlink
	}
	if env.ReadDir == nil {
		env.ReadDir = os.ReadDir
	}
	r := &Resolver{env: env, stats: map[string]statResult{}, dirs: map[string][]fs.DirEntry{}}

	given := map[string]bool{}
	for _, b := range env.Bases {
		given[path.Clean(b.Dir)] = true
	}
	var procs, windows, others, roots []Base
	for _, b := range env.Bases {
		switch b.Source {
		case SourceProcess:
			procs = append(procs, b)
		case SourceWindow:
			windows = append(windows, b)
		default:
			others = append(others, b)
		}
		// A root that is itself a given base keeps that base's rank.
		if root := r.ProjectRoot(b.Dir); root != "" && !given[root] {
			roots = append(roots, Base{root, SourceRoot})
		}
	}
	all := slices.Concat(procs, others, roots, windows)
	if env.Home != "" {
		all = append(all, Base{env.Home, SourceHome})
	}
	seen := map[string]bool{}
	for _, b := range all {
		d := path.Clean(b.Dir)
		if !seen[d] {
			seen[d] = true
			r.static = append(r.static, Base{d, b.Source})
		}
	}
	return r
}

// Bases returns the ranked static bases.
func (r *Resolver) Bases() []Base { return r.static }

// Lstat is Env.Lstat, cached.
func (r *Resolver) Lstat(p string) (fs.FileInfo, error) {
	if s, ok := r.stats[p]; ok {
		return s.info, s.err
	}
	info, err := r.env.Lstat(p)
	r.stats[p] = statResult{info, err}
	return info, err
}

func (r *Resolver) readDir(p string) ([]fs.DirEntry, error) {
	if entries, ok := r.dirs[p]; ok {
		return entries, nil
	}
	entries, err := r.env.ReadDir(p)
	if err == nil {
		r.dirs[p] = entries
	}
	return entries, err
}

func (r *Resolver) exists(p string) bool {
	_, err := r.Lstat(p)
	return err == nil
}

// ProjectRoot is the nearest enclosing git checkout, else the nearest
// directory holding a build manifest. Home and / never count.
func (r *Resolver) ProjectRoot(dir string) string {
	walk := func(found func(string) bool) string {
		for d := path.Clean(dir); d != "/" && d != "." && d != r.env.Home; d = path.Dir(d) {
			if found(d) {
				return d
			}
		}
		return ""
	}
	if d := walk(func(d string) bool { return r.exists(path.Join(d, ".git")) }); d != "" {
		return d
	}
	return walk(func(d string) bool {
		return slices.ContainsFunc(rootMarkers, func(m string) bool { return r.exists(path.Join(d, m)) })
	})
}

// Resolve finds every existing reading of token across its decodings and
// bases. screen lists directories shown above the token, nearest first; they
// outrank the static bases.
func (r *Resolver) Resolve(token string, screen []string) Resolution {
	bases := r.basesWith(screen)
	var res Resolution
	placed := false
	seen := map[string]bool{}
	consider := func(c Candidate, expanded bool) {
		if seen[c.Path] {
			return
		}
		seen[c.Path] = true
		fi, err := r.Lstat(c.Path)
		if err != nil {
			return
		}
		if !res.Exists {
			res = Resolution{Path: c.Path, Source: c.Source, Exists: true, IsDir: fi.IsDir(), Expanded: expanded}
		} else {
			res.Alternatives = append(res.Alternatives, c)
		}
	}

	variants := decode.Decode(token)
	for _, v := range variants {
		for _, c := range r.locate(v, bases) {
			if !placed {
				res.Path, res.Source, placed = c.Path, c.Source, true
			}
			consider(c, false)
		}
	}
	if res.Exists || !hasAbbreviation(variants[0]) {
		return res
	}
	for _, c := range r.locate(variants[0], bases) {
		for _, p := range r.expand(c.Path) {
			consider(Candidate{p, c.Source}, true)
		}
	}
	return res
}

func (r *Resolver) basesWith(screen []string) []Base {
	if len(screen) == 0 {
		return r.static
	}
	seen := map[string]bool{}
	out := make([]Base, 0, len(screen)+len(r.static))
	for _, d := range screen {
		if !seen[d] {
			seen[d] = true
			out = append(out, Base{d, SourceScreen})
		}
	}
	for _, b := range r.static {
		if !seen[b.Dir] {
			seen[b.Dir] = true
			out = append(out, b)
		}
	}
	return out
}

var envVarRoot = regexp.MustCompile(`^\$(?:\{\w+\}|[A-Za-z_]\w*)`)

// locate lists where a decoded token could live, best first.
func (r *Resolver) locate(v string, bases []Base) []Candidate {
	switch {
	case strings.HasPrefix(v, `\\?\`):
		return r.locate(v[4:], bases)
	case decode.IsDrivePath(v):
		return r.wine(v)
	case strings.HasPrefix(v, `\\`):
		parts := strings.Split(v[2:], `\`)
		if len(parts) >= 2 && (strings.EqualFold(parts[0], "wsl$") || strings.EqualFold(parts[0], "wsl.localhost")) {
			return []Candidate{{"/" + path.Join(parts[2:]...), SourceAbsolute}}
		}
		return nil
	case strings.HasPrefix(v, "%"):
		return nil
	case v == "~" || strings.HasPrefix(v, "~/"):
		if r.env.Home == "" {
			return nil
		}
		return []Candidate{{path.Join(r.env.Home, v[1:]), SourceHome}}
	case strings.HasPrefix(v, "~") && strings.Contains(v, "/"):
		// ~user/x: another user's home, assumed to sit beside ours. ~+ and ~-
		// name a directory only the shell knows.
		name, rest, _ := strings.Cut(v[1:], "/")
		switch {
		case name == "" || name == "+" || name == "-":
			return nil
		case name == "root":
			return []Candidate{{path.Join("/root", rest), SourceAbsolute}}
		case r.env.Home == "":
			return nil
		}
		return []Candidate{{path.Join(path.Dir(r.env.Home), name, rest), SourceAbsolute}}
	case envVarRoot.MatchString(v):
		missing := false
		p := os.Expand(v, func(k string) string {
			val := r.env.Getenv(k)
			missing = missing || val == ""
			return val
		})
		if missing || !path.IsAbs(p) {
			return nil
		}
		return []Candidate{{path.Clean(p), SourceAbsolute}}
	case strings.HasPrefix(v, "/"):
		return []Candidate{{path.Clean(v), SourceAbsolute}}
	}
	out := make([]Candidate, 0, len(bases))
	for _, b := range bases {
		out = append(out, Candidate{path.Join(b.Dir, v), b.Source})
	}
	return out
}

// wine maps a drive path through each prefix's dosdevices links.
func (r *Resolver) wine(v string) []Candidate {
	drive := strings.ToLower(v[:1]) + ":"
	rest := strings.FieldsFunc(v[3:], func(c rune) bool { return c == '\\' || c == '/' })
	var out []Candidate
	for _, pfx := range r.env.WinePrefixes {
		dev := path.Join(pfx, "dosdevices")
		target, err := r.env.Readlink(path.Join(dev, drive))
		if err != nil {
			continue
		}
		if !path.IsAbs(target) {
			target = path.Join(dev, target)
		}
		out = append(out, Candidate{r.caseFold(path.Clean(target), rest), SourceWine})
	}
	return out
}

// caseFold joins comps under root, matching each case-insensitively the way
// Wine does. When a component is missing it returns the join as written.
func (r *Resolver) caseFold(root string, comps []string) string {
	cur := root
	for _, c := range comps {
		next := path.Join(cur, c)
		if r.exists(next) {
			cur = next
			continue
		}
		entries, err := r.readDir(cur)
		if err != nil {
			return path.Join(append([]string{root}, comps...)...)
		}
		i := slices.IndexFunc(entries, func(e fs.DirEntry) bool { return strings.EqualFold(e.Name(), c) })
		if i < 0 {
			return path.Join(append([]string{root}, comps...)...)
		}
		cur = path.Join(cur, entries[i].Name())
	}
	return cur
}

// maxExpansions bounds how many paths one abbreviated token may widen to.
const maxExpansions = 8

// hasAbbreviation reports a rooted path with a component before the last
// that is one or two characters (plus an optional leading dot), the way a
// shortened prompt prints its working directory.
func hasAbbreviation(p string) bool {
	if !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "~/") {
		return false
	}
	comps := strings.Split(p, "/")
	for _, c := range comps[:len(comps)-1] {
		if c == "." || c == ".." || c == "~" || strings.HasPrefix(c, "$") {
			continue
		}
		if n := len(strings.TrimPrefix(c, ".")); n >= 1 && n <= 2 {
			return true
		}
	}
	return false
}

// expand widens each missing directory component of an absolute path to the
// entries it prefixes. The final component must match exactly.
func (r *Resolver) expand(p string) []string {
	comps := strings.Split(strings.TrimPrefix(path.Clean(p), "/"), "/")
	if len(comps) < 2 {
		return nil
	}
	paths := []string{"/"}
	for i, c := range comps {
		last := i == len(comps)-1
		var next []string
		for _, dir := range paths {
			if exact := path.Join(dir, c); r.exists(exact) {
				next = append(next, exact)
				continue
			}
			if last {
				continue
			}
			entries, err := r.readDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), c) && (e.IsDir() || e.Type()&fs.ModeSymlink != 0) {
					next = append(next, path.Join(dir, e.Name()))
				}
			}
		}
		if len(next) == 0 {
			return nil
		}
		if len(next) > maxExpansions {
			next = next[:maxExpansions]
		}
		paths = next
	}
	slices.Sort(paths)
	return paths
}

// RankWinePrefixes orders prefixes named by running processes' environments
// ahead of those found on disk, without duplicates.
func RankWinePrefixes(procEnvs []map[string]string, found []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		p = path.Clean(p)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, e := range procEnvs {
		add(e["WINEPREFIX"])
		if c := e["STEAM_COMPAT_DATA_PATH"]; c != "" {
			add(path.Join(c, "pfx"))
		}
	}
	for _, f := range found {
		add(f)
	}
	return out
}
