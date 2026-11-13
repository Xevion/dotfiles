// Command screen-pick finds paths, URLs, hosts and commit hashes in a
// captured kitty screen and offers actions on the one you pick.
//
//	screen-pick hints|list [-window-id N]  < screen
//	screen-pick scan [-full] [-index] [-json] [-width N] < screen
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"screenpick/internal/detect"
	"screenpick/internal/lookup"
	"screenpick/internal/probe"
	"screenpick/internal/resolve"
	"screenpick/internal/tui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "screen-pick:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	verb, args := args[0], args[1:]
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	windowID := fs.Int("window-id", 0, "kitty window the screen came from")
	width := fs.Int("width", 0, "screen width in cells (default: the terminal's)")
	full := fs.Bool("full", false, "scan: probe kitty and /proc as the picker does")
	withIndex := fs.Bool("index", false, "scan: also search git indexes for unresolved paths")
	asJSON := fs.Bool("json", false, "scan: print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read screen: %w", err)
	}
	if *width == 0 {
		if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil {
			*width = w
		}
	}

	switch verb {
	case "hints", "list":
		mode := tui.Hints
		if verb == "list" {
			mode = tui.List
		}
		return pick(string(text), mode, *width, *windowID)
	case "scan":
		return scan(string(text), *width, *windowID, *full, *withIndex, *asJSON)
	}
	return usage()
}

func usage() error {
	return fmt.Errorf("usage: screen-pick hints|list|scan [flags] < screen-text")
}

func pick(text string, mode tui.Mode, width, windowID int) error {
	s := newSession(text, width, windowID)
	m := tui.New(tui.Config{
		Mode:    mode,
		Screen:  text,
		Matches: s.Textual(),
		Quick:   s.Quick,
		Backend: s,
		Home:    s.home,
	})
	// The renderer flushes on a timer, 120 per second at most; its first tick
	// is the earliest the first frame can appear.
	_, err := tea.NewProgram(m, tea.WithFPS(120)).Run()
	return err
}

type scanRow struct {
	Kind   string   `json:"kind"`
	Text   string   `json:"text"`
	Line   int      `json:"line,omitempty"`
	Col    int      `json:"col,omitempty"`
	Path   string   `json:"path,omitempty"`
	Source string   `json:"source,omitempty"`
	Exists bool     `json:"exists"`
	Alts   []string `json:"alternatives,omitempty"`
}

func scan(text string, width, windowID int, full, withIndex, asJSON bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	env := probe.Fast()
	if full {
		env = probe.Full(ctx, windowID)
	}
	r := resolve.New(env)
	ms := detect.ScanWidth(text, r, width)
	if withIndex {
		s := lookup.Searcher{Roots: lookup.Roots(r, ms)}
		var merger lookup.Merger
		for res := range s.Index(ctx, lookup.Queries(ms)) {
			if res.Err == nil {
				merger.Apply(res.ID, &ms[res.ID].Resolution, res)
			}
		}
	}
	elapsed := time.Since(start)

	rows := make([]scanRow, len(ms))
	for i, m := range ms {
		rows[i] = scanRow{Kind: m.Kind.String(), Text: m.Text, Line: m.Line, Col: m.Col, Path: m.Path, Exists: m.Exists}
		if m.Kind == detect.KindPath && m.Source != 0 {
			rows[i].Source = m.Source.String()
		}
		for _, a := range m.Alternatives {
			rows[i].Alts = append(rows[i].Alts, a.Path)
		}
	}
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(rows)
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, row := range rows {
		exists := ""
		if row.Kind == "path" {
			exists = strconv.FormatBool(row.Exists)
		}
		alts := ""
		if len(row.Alts) > 0 {
			alts = "+" + strconv.Itoa(len(row.Alts))
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", row.Kind, row.Text, exists, row.Source, row.Path, alts)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d bytes, %d matches, %v\n", len(text), len(ms), elapsed.Round(time.Microsecond))
	return nil
}
