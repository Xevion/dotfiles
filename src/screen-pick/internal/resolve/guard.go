package resolve

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"
)

// ErrSlow is what a guarded disk call returns when it outlasted its budget, or
// when it sits under a mount that already did.
var ErrSlow = errors.New("resolve: slow mount")

// Guard bounds disk checks so one dead network mount costs one budget, not one
// per path. A call that outlasts the budget is abandoned (it finishes in the
// background) and its whole mount is skipped from then on.
type Guard struct {
	budget time.Duration

	mu     sync.Mutex
	slow   []string
	mounts []string
	parsed bool
}

func NewGuard(budget time.Duration) *Guard { return &Guard{budget: budget} }

// Wrap returns env with its disk calls guarded.
func (g *Guard) Wrap(env Env) Env {
	lstat, readlink, readDir := env.Lstat, env.Readlink, env.ReadDir
	if lstat == nil {
		lstat = os.Lstat
	}
	if readlink == nil {
		readlink = os.Readlink
	}
	if readDir == nil {
		readDir = os.ReadDir
	}
	env.Lstat = func(p string) (fs.FileInfo, error) { return guarded(g, p, lstat) }
	env.Readlink = func(p string) (string, error) { return guarded(g, p, readlink) }
	env.ReadDir = func(p string) ([]fs.DirEntry, error) { return guarded(g, p, readDir) }
	return env
}

func guarded[T any](g *Guard, p string, call func(string) (T, error)) (T, error) {
	var zero T
	if g.isSlow(p) {
		return zero, ErrSlow
	}
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := call(p)
		done <- result{v, err}
	}()
	t := time.NewTimer(g.budget)
	defer t.Stop()
	select {
	case r := <-done:
		return r.v, r.err
	case <-t.C:
		g.markSlow(p)
		return zero, ErrSlow
	}
}

func (g *Guard) isSlow(p string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, s := range g.slow {
		if underPath(p, s) {
			return true
		}
	}
	return false
}

// markSlow skips p's mount from now on, or only p itself when it is on the
// root filesystem, which cannot be skipped wholesale.
func (g *Guard) markSlow(p string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.parsed {
		g.parsed = true
		if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
			g.mounts = mountPoints(data)
		}
	}
	best := p
	longest := 0
	for _, m := range g.mounts {
		if m != "/" && len(m) > longest && underPath(p, m) {
			best, longest = m, len(m)
		}
	}
	g.slow = append(g.slow, best)
}

func underPath(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}

// mountPoints lists the mount points in /proc/self/mountinfo.
func mountPoints(data []byte) []string {
	var out []string
	for line := range bytes.Lines(data) {
		fields := bytes.Fields(line)
		if len(fields) > 4 {
			out = append(out, unescapeMount(string(fields[4])))
		}
	}
	return out
}

// unescapeMount decodes the octal escapes (\040 for a space) the kernel uses.
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			n := 0
			ok := true
			for _, c := range []byte(s[i+1 : i+4]) {
				if c < '0' || c > '7' {
					ok = false
					break
				}
				n = n*8 + int(c-'0')
			}
			if ok {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// NoDisk is an Env that sees an empty disk: everything is missing, so a scan
// with it is purely textual.
func NoDisk(home string) Env {
	return Env{
		Home:     home,
		Lstat:    func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist },
		Readlink: func(string) (string, error) { return "", fs.ErrNotExist },
		ReadDir:  func(string) ([]fs.DirEntry, error) { return nil, fs.ErrNotExist },
	}
}
