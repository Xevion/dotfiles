// Package probe gathers where relative paths on a kitty window probably
// point: its processes' working directories and the Wine prefixes in play.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/prometheus/procfs"

	"screenpick/internal/resolve"
)

// Fast is what costs nothing to learn: the cwd kitty launched us in (the
// window's foreground process) and home. Wine prefixes are left for Full.
func Fast() resolve.Env {
	home, _ := os.UserHomeDir()
	env := resolve.Env{Home: home}
	if cwd, err := os.Getwd(); err == nil {
		env.Bases = []resolve.Base{{Dir: cwd, Source: resolve.SourceProcess}}
	}
	return env
}

// Full adds what the source window's processes reveal and the Wine prefixes
// on disk. windowID is the kitty window the screen came from; 0 skips kitty.
// Failures degrade to what Fast knows rather than erroring.
func Full(ctx context.Context, windowID int) resolve.Env {
	env := Fast()
	var pids []int
	var windowCwd string
	if windowID > 0 && os.Getenv("KITTY_LISTEN_ON") != "" {
		if out, err := kittyLS(ctx, windowID); err == nil {
			if w, err := parseKittyLS(out, windowID); err == nil {
				windowCwd = w.Cwd
				for _, p := range w.Foreground {
					pids = append(pids, p.PID)
				}
			}
		}
	}

	var envs []map[string]string
	if fs, err := procfs.NewDefaultFS(); err == nil && len(pids) > 0 {
		tree := processTree(fs)
		for _, pid := range descendants(tree, pids) {
			p, err := fs.Proc(pid)
			if err != nil {
				continue
			}
			if cwd, err := p.Cwd(); err == nil {
				env.Bases = append(env.Bases, resolve.Base{Dir: cwd, Source: resolve.SourceProcess})
			}
			if vars, err := p.Environ(); err == nil {
				envs = append(envs, wineVars(vars))
			}
		}
	}
	if windowCwd != "" {
		env.Bases = append(env.Bases, resolve.Base{Dir: windowCwd, Source: resolve.SourceWindow})
	}
	env.WinePrefixes = resolve.RankWinePrefixes(envs, PrefixesOnDisk(env.Home))
	return env
}

func kittyLS(ctx context.Context, windowID int) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "kitten", "@", "ls", "--match", "id:"+strconv.Itoa(windowID))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kitten @ ls: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}

// Window is the part of kitty's ls output probe uses.
type Window struct {
	ID         int    `json:"id"`
	Cwd        string `json:"cwd"`
	Foreground []struct {
		PID int    `json:"pid"`
		Cwd string `json:"cwd"`
	} `json:"foreground_processes"`
}

// parseKittyLS finds window id in `kitten @ ls` output.
func parseKittyLS(data []byte, id int) (Window, error) {
	var osWindows []struct {
		Tabs []struct {
			Windows []Window `json:"windows"`
		} `json:"tabs"`
	}
	if err := json.Unmarshal(data, &osWindows); err != nil {
		return Window{}, fmt.Errorf("parse kitty ls: %w", err)
	}
	for _, ow := range osWindows {
		for _, tab := range ow.Tabs {
			if i := slices.IndexFunc(tab.Windows, func(w Window) bool { return w.ID == id }); i >= 0 {
				return tab.Windows[i], nil
			}
		}
	}
	return Window{}, fmt.Errorf("window %d not in kitty ls output", id)
}

// processTree maps each pid to its children.
func processTree(fs procfs.FS) map[int][]int {
	procs, err := fs.AllProcs()
	if err != nil {
		return nil
	}
	tree := map[int][]int{}
	for _, p := range procs {
		if st, err := p.Stat(); err == nil {
			tree[st.PPID] = append(tree[st.PPID], p.PID)
		}
	}
	return tree
}

// descendants lists roots and everything below them, roots first, each once.
// A short-lived child (an agent's shell running a command) is often the only
// process sitting in the directory its output talks about.
func descendants(tree map[int][]int, roots []int) []int {
	seen := map[int]bool{}
	var out []int
	queue := slices.Clone(roots)
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if seen[pid] {
			continue
		}
		seen[pid] = true
		out = append(out, pid)
		queue = append(queue, tree[pid]...)
	}
	return out
}

var wineKeys = []string{"WINEPREFIX", "STEAM_COMPAT_DATA_PATH"}

// wineVars keeps the Wine-related entries of a KEY=VALUE environment.
func wineVars(environ []string) map[string]string {
	out := map[string]string{}
	for _, kv := range environ {
		for _, k := range wineKeys {
			if v, ok := strings.CutPrefix(kv, k+"="); ok {
				out[k] = v
			}
		}
	}
	return out
}

var vdfPath = regexp.MustCompile(`"path"\s+"((?:[^"\\]|\\.)*)"`)

// steamLibraries reads library roots from Steam's libraryfolders.vdf.
func steamLibraries(vdf []byte) []string {
	var out []string
	for _, m := range vdfPath.FindAllSubmatch(vdf, -1) {
		if p := string(bytes.ReplaceAll(m[1], []byte(`\\`), []byte(`\`))); !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// PrefixesOnDisk finds Wine prefixes under home: ~/.wine, then every Proton
// prefix in each Steam library. Only directories holding dosdevices count.
func PrefixesOnDisk(home string) []string {
	var out []string
	isPrefix := func(p string) bool {
		_, err := os.Stat(filepath.Join(p, "dosdevices"))
		return err == nil
	}
	if p := filepath.Join(home, ".wine"); isPrefix(p) {
		out = append(out, p)
	}
	libs := []string{filepath.Join(home, ".local/share/Steam")}
	if vdf, err := os.ReadFile(filepath.Join(home, ".local/share/Steam/steamapps/libraryfolders.vdf")); err == nil {
		libs = append(libs, steamLibraries(vdf)...)
	}
	seen := map[string]bool{}
	for _, lib := range libs {
		real, err := filepath.EvalSymlinks(lib)
		if err != nil || seen[real] {
			continue
		}
		seen[real] = true
		matches, _ := filepath.Glob(filepath.Join(real, "steamapps/compatdata/*/pfx"))
		for _, p := range matches {
			if isPrefix(p) {
				out = append(out, p)
			}
		}
	}
	return out
}
