// Package fakefs is an in-memory tree for tests that need Lstat, Readlink and
// ReadDir without touching the disk.
package fakefs

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"
)

type info struct {
	name string
	mode fs.FileMode
}

func (i info) Name() string       { return i.name }
func (i info) Size() int64        { return 0 }
func (i info) ModTime() time.Time { return time.Time{} }
func (i info) IsDir() bool        { return i.mode.IsDir() }
func (i info) Sys() any           { return nil }
func (i info) Mode() fs.FileMode  { return i.mode }

// FS holds files, directories and symlinks. Links are never followed, which
// is what Lstat and Readlink would see.
type FS struct {
	modes    map[string]fs.FileMode
	links    map[string]string
	children map[string][]string
}

// New builds a tree from entries plus every parent directory. An entry ending
// in "/" is an empty directory, and "name -> target" is a symlink.
func New(entries ...string) *FS {
	f := &FS{
		modes:    map[string]fs.FileMode{"/": fs.ModeDir | 0o755},
		links:    map[string]string{},
		children: map[string][]string{},
	}
	for _, e := range entries {
		name, target, isLink := strings.Cut(e, " -> ")
		p := path.Clean(name)
		var parents []string
		for d := path.Dir(p); d != "/"; d = path.Dir(d) {
			parents = append(parents, d)
		}
		for _, d := range slices.Backward(parents) {
			f.add(d, fs.ModeDir|0o755)
		}
		switch {
		case isLink:
			f.add(p, fs.ModeSymlink|0o777)
			f.links[p] = target
		case strings.HasSuffix(name, "/"):
			f.add(p, fs.ModeDir|0o755)
		default:
			f.add(p, 0o644)
		}
	}
	return f
}

func (f *FS) add(p string, mode fs.FileMode) {
	if _, ok := f.modes[p]; ok {
		return
	}
	f.modes[p] = mode
	f.children[path.Dir(p)] = append(f.children[path.Dir(p)], path.Base(p))
}

func (f *FS) Lstat(name string) (fs.FileInfo, error) {
	p := path.Clean(name)
	if mode, ok := f.modes[p]; ok {
		return info{path.Base(p), mode}, nil
	}
	return nil, fs.ErrNotExist
}

func (f *FS) Readlink(name string) (string, error) {
	if t, ok := f.links[path.Clean(name)]; ok {
		return t, nil
	}
	return "", fs.ErrInvalid
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	p := path.Clean(name)
	if mode, ok := f.modes[p]; !ok || !mode.IsDir() {
		return nil, fs.ErrNotExist
	}
	out := make([]fs.DirEntry, 0, len(f.children[p]))
	for _, c := range f.children[p] {
		i, _ := f.Lstat(path.Join(p, c))
		out = append(out, fs.FileInfoToDirEntry(i))
	}
	return out, nil
}

// Files lists every regular file under root, relative to it and sorted, the
// way `git ls-files` would.
func (f *FS) Files(root string) []string {
	prefix := strings.TrimSuffix(path.Clean(root), "/") + "/"
	var out []string
	for p, mode := range f.modes {
		if !mode.IsRegular() {
			continue
		}
		if rel, ok := strings.CutPrefix(p, prefix); ok {
			out = append(out, rel)
		}
	}
	slices.Sort(out)
	return out
}
