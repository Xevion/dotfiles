// Package action decides what can be done with a target and builds the
// commands that do it. Menus never offer two entries with the same effect.
package action

import (
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"screenpick/internal/detect"
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
	// Show runs `git show Arg` in this terminal.
	Show
	// Search walks the disk for the target.
	Search
)

// Action is one menu entry. Key is its single-key accelerator.
type Action struct {
	Key   string
	Label string
	Verb  Verb
	Arg   string
	Line  int
	Col   int
}

// Context is what menus need beyond the match itself.
type Context struct {
	Cwd string
	// Stat defaults to os.Stat.
	Stat func(string) (fs.FileInfo, error)
}

// For lists the actions that make sense for m, without duplicate effects.
func For(m detect.Match, ctx Context) []Action {
	if ctx.Stat == nil {
		ctx.Stat = os.Stat
	}
	var b builder
	switch m.Kind {
	case detect.KindPath:
		forPath(&b, m, ctx)
	case detect.KindURL:
		b.add("o", "open in browser", Open, m.Text)
		b.add("y", "copy", Copy, m.Text)
	case detect.KindHost:
		host, port := splitHostPort(m.Text)
		if port != "" {
			b.add("o", "open in browser", Open, "http://"+m.Text)
		}
		b.add("y", "copy", Copy, m.Text)
		if port != "" {
			b.add("h", "copy host", Copy, host)
		}
	case detect.KindSHA:
		b.add("g", "git show", Show, m.Text)
		b.add("y", "copy", Copy, m.Text)
	}
	return b.actions
}

func forPath(b *builder, m detect.Match, ctx Context) {
	p := m.Path
	shown := m.Text
	rel := relativeTo(p, ctx.Cwd)
	switch {
	case m.Exists && m.IsDir:
		b.add("o", "open folder", Open, p)
		b.add("t", "new tab here", Tab, p)
	case m.Exists:
		label := "edit"
		if m.Line > 0 {
			label += " at line " + strconv.Itoa(m.Line)
		}
		b.addAt("e", label, Edit, p, m.Line, m.Col)
		b.add("o", "open", Open, p)
		b.add("f", "open containing folder", Open, path.Dir(p))
		b.add("t", "new tab in folder", Tab, path.Dir(p))
	}
	if m.Exists {
		b.add("y", "copy path", Copy, p)
		b.add("r", "copy relative path", Copy, rel)
		b.add("s", "copy as shown", Copy, shown)
		if !m.IsDir {
			b.add("d", "copy folder", Copy, path.Dir(p))
		}
		b.add("/", "search for more copies", Search, shown)
		return
	}
	b.add("s", "copy as shown", Copy, shown)
	if p != "" {
		b.add("y", "copy resolved path", Copy, p)
		if near := nearestDir(path.Dir(p), ctx.Stat); near != "/" {
			b.add("f", "open nearest existing folder", Open, near)
		}
	}
	b.add("/", "search the disk", Search, shown)
}

type builder struct {
	actions []Action
}

func (b *builder) add(key, label string, verb Verb, arg string) {
	b.addAt(key, label, verb, arg, 0, 0)
}

// addAt skips empty arguments and any effect already offered.
func (b *builder) addAt(key, label string, verb Verb, arg string, line, col int) {
	if arg == "" {
		return
	}
	for _, a := range b.actions {
		if a.Verb == verb && a.Arg == arg {
			return
		}
	}
	b.actions = append(b.actions, Action{Key: key, Label: label, Verb: verb, Arg: arg, Line: line, Col: col})
}

// relativeTo returns p relative to dir when p is inside it, else "".
func relativeTo(p, dir string) string {
	if dir == "" || p == "" {
		return ""
	}
	rel, err := filepath.Rel(dir, p)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
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
		return exec.Command("xdg-open", a.Arg), true
	case Tab:
		return exec.Command("kitten", "@", "launch", "--type=tab", "--cwd="+a.Arg), true
	case Edit:
		argv := EditorArgs(editor, a.Arg, a.Line, a.Col)
		return exec.Command(argv[0], argv[1:]...), false
	case Show:
		return exec.Command("git", "show", a.Arg), false
	}
	return nil, false
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

// EditorArgs builds an editor invocation that lands on line and col when the
// editor is known to accept them. editor may carry its own flags.
func EditorArgs(editor, file string, line, col int) []string {
	fields := strings.Fields(editor)
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
