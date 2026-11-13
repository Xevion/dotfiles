// Package preview reads a few lines of a file, or the entries of a folder, for
// the picker's detail pane.
package preview

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"
	"syscall"
	"unicode"

	"screenpick/internal/filetype"
)

// Preview is what the pane shows: Lines, or a Note when there is nothing to
// show line by line.
type Preview struct {
	Lines []Line
	Note  string
	// More counts folder entries beyond Lines.
	More int
}

// Line is one line of a file (No counts from 1) or one folder entry (No 0).
type Line struct {
	No    int
	Text  string
	Dir   bool
	Focus bool
}

// maxRead bounds how far into a file a preview reads.
const maxRead = 4 << 20

// File shows up to rows lines of the file at p, placing line focus a third of
// the way down when it is set.
func File(p string, focus, rows int) Preview {
	f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return Preview{Note: err.Error()}
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return Preview{Note: "not a regular file"}
	}
	br := bufio.NewReaderSize(f, filetype.HeadSize)
	head, _ := br.Peek(filetype.HeadSize)
	if t := filetype.Sniff(p, head); t.Class != filetype.Text {
		return Preview{Note: t.Mime}
	}
	if len(head) == 0 {
		return Preview{Note: "empty file"}
	}
	first := 1
	if focus > 0 {
		first = max(1, focus-rows/3)
	}
	sc := bufio.NewScanner(&limited{br, maxRead})
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var out Preview
	for n := 1; sc.Scan() && len(out.Lines) < rows; n++ {
		if n >= first {
			out.Lines = append(out.Lines, Line{No: n, Text: clean(sc.Text()), Focus: n == focus})
		}
	}
	if len(out.Lines) == 0 && focus > 0 {
		out.Note = fmt.Sprintf("the file has fewer than %d lines", focus)
	}
	return out
}

// Dir lists up to rows entries of the folder at p, folders first, and counts
// the rest in More.
func Dir(p string, rows int) Preview {
	es, err := os.ReadDir(p)
	if err != nil {
		return Preview{Note: err.Error()}
	}
	if len(es) == 0 {
		return Preview{Note: "empty folder"}
	}
	slices.SortStableFunc(es, func(a, b os.DirEntry) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		return cmp.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
	})
	out := Preview{More: max(len(es)-rows, 0)}
	for _, e := range es[:max(min(len(es), rows), 0)] {
		out.Lines = append(out.Lines, Line{Text: clean(e.Name()), Dir: e.IsDir()})
	}
	return out
}

// Text wraps command output, such as a commit summary, as a preview.
func Text(s string, rows int) Preview {
	var out Preview
	for l := range strings.Lines(s) {
		if len(out.Lines) == rows {
			break
		}
		out.Lines = append(out.Lines, Line{Text: clean(strings.TrimRight(l, "\n"))})
	}
	return out
}

// clean expands tabs and drops control characters, which would move the
// cursor or recolour the pane.
func clean(s string) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

type limited struct {
	r *bufio.Reader
	n int
}

func (l *limited) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, bytes.ErrTooLarge
	}
	p = p[:min(len(p), l.n)]
	n, err := l.r.Read(p)
	l.n -= n
	return n, err
}
