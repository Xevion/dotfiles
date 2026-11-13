// Package action decides what can be done with a target and builds the
// commands that do it. Menus never offer two entries with the same effect.
package action

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	"screenpick/internal/detect"
	"screenpick/internal/filetype"
)

type Verb uint8

const (
	// Copy puts Arg on the clipboard.
	Copy Verb = iota + 1
	// Open hands Arg to the desktop's default handler.
	Open
	// Edit opens Arg in $EDITOR in this terminal, at Line when set.
	Edit
	// Tab opens a kitty tab with Arg as its working directory.
	Tab
	// Run starts Argv detached, in Dir.
	Run
	// Git runs git with Argv in this terminal, in Dir.
	Git
	// Search walks the disk for the target.
	Search
)

// Group sorts a menu into sections.
type Group uint8

const (
	GroupOpen Group = iota
	GroupGit
	GroupCopy
	GroupSearch
)

func (g Group) String() string {
	return [...]string{"open", "git", "copy", "search"}[g]
}

// Action is one menu entry. Key is its single-key accelerator.
type Action struct {
	Key   string
	Label string
	Group Group
	Verb  Verb
	// Arg is what the action is about, and what Copy copies.
	Arg  string
	Argv []string
	Dir  string
	Line int
	Col  int
}

// Context is what menus need beyond the match itself. Nil functions use the
// real system.
type Context struct {
	Cwd      string
	Stat     func(string) (fs.FileInfo, error)
	Head     func(string) ([]byte, error)
	LookPath func(string) (string, error)
	// CommitRepo finds the checkout that has a commit, or "".
	CommitRepo func(sha string) string
}

func (c *Context) defaults() {
	if c.Stat == nil {
		c.Stat = os.Stat
	}
	if c.Head == nil {
		c.Head = filetype.Head
	}
	if c.LookPath == nil {
		c.LookPath = exec.LookPath
	}
	if c.CommitRepo == nil {
		c.CommitRepo = func(string) string { return "" }
	}
}

// For lists the actions that make sense for m, without duplicate effects.
func For(m detect.Match, ctx Context) []Action {
	ctx.defaults()
	var b builder
	switch m.Kind {
	case detect.KindPath:
		forPath(&b, m, ctx)
	case detect.KindURL:
		if u, ok := detect.WebURL(m); ok && plainText(u) {
			b.add("o", "open in browser", GroupOpen, Open, u)
		}
		b.add("y", "copy", GroupCopy, Copy, m.Text)
	case detect.KindHost:
		host, port := splitHostPort(m.Text)
		if u, ok := detect.WebURL(m); ok && plainText(u) {
			b.add("o", "open in browser", GroupOpen, Open, u)
		}
		b.add("y", "copy", GroupCopy, Copy, m.Text)
		if port != "" {
			b.add("h", "copy host", GroupCopy, Copy, host)
		}
	case detect.KindSHA:
		if repo := ctx.CommitRepo(m.Text); repo != "" {
			b.put(Action{Key: "g", Label: "git show", Group: GroupGit, Verb: Git, Arg: m.Text, Argv: []string{"show", m.Text, "--"}, Dir: repo})
			b.add("R", "new tab at repo root", GroupGit, Tab, repo)
		}
		b.add("y", "copy", GroupCopy, Copy, m.Text)
	}
	return b.actions
}

// plainText reports whether s is free of control characters, which have no
// place in an address handed to the desktop's opener.
func plainText(s string) bool {
	return !strings.ContainsFunc(s, unicode.IsControl)
}

// tools are the programs that can open a binary, by the name they install as.
var tools = []struct{ key, label, bin string }{
	{"i", "open in IDA", "ida"},
	{"G", "open in Ghidra", "ghidraRun"},
}

func forPath(b *builder, m detect.Match, ctx Context) {
	p := m.Path
	shown := m.Text
	if !m.Exists {
		b.add("s", "copy as shown", GroupCopy, Copy, shown)
		if p != "" {
			b.add("y", "copy resolved path", GroupCopy, Copy, p)
			if near := nearestDir(path.Dir(p), ctx.Stat); near != "/" {
				b.add("f", "open nearest existing folder", GroupOpen, Open, near)
			}
		}
		b.add("/", "search the disk", GroupSearch, Search, shown)
		return
	}

	dir := p
	if !m.IsDir {
		dir = path.Dir(p)
		forFile(b, m, ctx)
		b.add("f", "open containing folder", GroupOpen, Open, dir)
		b.add("t", "new tab in folder", GroupOpen, Tab, dir)
	} else {
		b.add("o", "open folder", GroupOpen, Open, p)
		b.add("t", "new tab here", GroupOpen, Tab, p)
	}

	repo := RepoRoot(dir, ctx.Stat)
	inRepo := relativeTo(p, repo)
	if repo != "" {
		b.add("R", "new tab at repo root", GroupGit, Tab, repo)
		if inRepo != "" {
			b.put(Action{Key: "l", Label: "git log", Group: GroupGit, Verb: Git, Arg: inRepo, Argv: []string{"--literal-pathspecs", "log", "--follow", "--", inRepo}, Dir: repo})
		}
	}

	b.add("y", "copy path", GroupCopy, Copy, p)
	b.add("r", "copy relative path", GroupCopy, Copy, relativeTo(p, ctx.Cwd))
	b.add("p", "copy repo path", GroupCopy, Copy, inRepo)
	b.add("s", "copy as shown", GroupCopy, Copy, shown)
	if !m.IsDir {
		b.add("Y", "copy folder", GroupCopy, Copy, dir)
	}
	b.add("/", "search for more copies", GroupSearch, Search, shown)
}

// forFile offers what suits the file's type: an editor only for text, Wine and
// disassemblers for programs, the desktop handler for everything else, and
// nothing for a pipe or device, which an editor would wait on forever.
func forFile(b *builder, m detect.Match, ctx Context) {
	p := m.Path
	class := filetype.Text
	head, err := ctx.Head(p)
	switch {
	case errors.Is(err, filetype.ErrNotRegular):
		return
	case err == nil:
		class = filetype.Sniff(p, head).Class
	}
	switch class {
	case filetype.Text, filetype.Unknown:
		label := "edit"
		if m.Line > 0 {
			label += " at line " + strconv.Itoa(m.Line)
		}
		b.put(Action{Key: "e", Label: label, Group: GroupOpen, Verb: Edit, Arg: p, Line: m.Line, Col: m.Col})
		b.add("o", "open with default app", GroupOpen, Open, p)
	case filetype.Image, filetype.Document, filetype.Archive, filetype.Media:
		b.add("o", "open with default app", GroupOpen, Open, p)
	default:
		if wine, err := ctx.LookPath("wine"); err == nil && class == filetype.WindowsExe {
			b.put(Action{Key: "w", Label: "run with Wine", Group: GroupOpen, Verb: Run, Arg: p, Argv: []string{wine, p}, Dir: path.Dir(p)})
		}
		for _, t := range tools {
			if bin, err := ctx.LookPath(t.bin); err == nil {
				b.put(Action{Key: t.key, Label: t.label, Group: GroupOpen, Verb: Run, Arg: p, Argv: []string{bin, p}, Dir: path.Dir(p)})
			}
		}
	}
}

type builder struct {
	actions []Action
}

func (b *builder) add(key, label string, g Group, verb Verb, arg string) {
	b.put(Action{Key: key, Label: label, Group: g, Verb: verb, Arg: arg})
}

// put skips empty arguments and any effect already offered.
func (b *builder) put(a Action) {
	if a.Arg == "" {
		return
	}
	for _, have := range b.actions {
		if have.Verb == a.Verb && have.Arg == a.Arg && have.Dir == a.Dir && strings.Join(have.Argv, "\x00") == strings.Join(a.Argv, "\x00") {
			return
		}
	}
	b.actions = append(b.actions, a)
}

// RepoRoot returns the git checkout containing dir, or "".
func RepoRoot(dir string, stat func(string) (fs.FileInfo, error)) string {
	for d := dir; d != "" && d != "."; d = path.Dir(d) {
		if _, err := stat(path.Join(d, ".git")); err == nil {
			return d
		}
		if d == "/" {
			break
		}
	}
	return ""
}

// relativeTo returns p relative to dir when p is inside it, else "".
func relativeTo(p, dir string) string {
	if dir == "" || p == "" {
		return ""
	}
	rel, err := filepath.Rel(dir, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}
	return rel
}

func nearestDir(p string, stat func(string) (fs.FileInfo, error)) string {
	for p != "/" && p != "." {
		if fi, err := stat(p); err == nil && fi.IsDir() {
			return p
		}
		p = path.Dir(p)
	}
	return "/"
}

func splitHostPort(s string) (host, port string) {
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	return s, ""
}

// Command builds the process for a, or nil for verbs handled in-process
// (Copy, Search). Detached reports whether it must outlive the picker.
func (a Action) Command(editor string) (cmd *exec.Cmd, detached bool) {
	switch a.Verb {
	case Open:
		return exec.Command(opener(), a.Arg), true
	case Tab:
		return exec.Command("kitten", "@", "launch", "--type=tab", "--cwd="+a.Arg), true
	case Run:
		if len(a.Argv) == 0 {
			return nil, false
		}
		cmd = exec.Command(a.Argv[0], a.Argv[1:]...)
		cmd.Dir = a.Dir
		return cmd, true
	case Edit:
		argv := EditorArgs(editor, a.Arg, a.Line, a.Col)
		return exec.Command(argv[0], argv[1:]...), false
	case Git:
		cmd = exec.Command("git", a.Argv...)
		cmd.Dir = a.Dir
		return cmd, false
	}
	return nil, false
}

// opener names the program that hands a path or address to the desktop's
// default handler: xdg-open, or what stands in for it on WSL and macOS. On
// Linux "open" is a different program (openvt) and never a candidate.
func opener() string {
	candidates := []string{"xdg-open", "wslview"}
	if runtime.GOOS == "darwin" {
		candidates = []string{"open"}
	}
	for _, bin := range candidates {
		if _, err := exec.LookPath(bin); err == nil {
			return bin
		}
	}
	return "xdg-open"
}

// Detach starts cmd in its own session with no stdio, so closing the picker
// does not take it down.
func Detach(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// splitFields splits s on blanks the way a shell would for plain words:
// single and double quotes and backslashes group, nothing expands. An
// unterminated quote runs to the end.
func splitFields(s string) []string {
	var out []string
	var cur strings.Builder
	in := false
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case quote != 0:
			switch {
			case r == quote:
				quote = 0
			case r == '\\' && quote == '"':
				escaped = true
			default:
				cur.WriteRune(r)
			}
		case r == '\\':
			escaped, in = true, true
		case r == '\'' || r == '"':
			quote, in = r, true
		case unicode.IsSpace(r):
			if in {
				out = append(out, cur.String())
				cur.Reset()
				in = false
			}
		default:
			cur.WriteRune(r)
			in = true
		}
	}
	if in {
		out = append(out, cur.String())
	}
	return out
}

// EditorArgs builds an editor invocation that lands on line and col when the
// editor is known to accept them. editor may carry its own flags, quoted as a
// shell would.
func EditorArgs(editor, file string, line, col int) []string {
	fields := splitFields(editor)
	if len(fields) == 0 {
		fields = []string{"micro"}
	}
	if line <= 0 {
		return append(fields, file)
	}
	l := strconv.Itoa(line)
	lc := l
	if col > 0 {
		lc += ":" + strconv.Itoa(col)
	}
	switch filepath.Base(fields[0]) {
	case "vi", "vim", "nvim", "nano", "emacs", "emacsclient", "kak", "micro":
		return append(fields, "+"+l, file)
	case "hx", "helix", "zed", "subl":
		return append(fields, file+":"+lc)
	case "code", "codium", "cursor":
		return append(fields, "-g", file+":"+lc)
	}
	return append(fields, file)
}
