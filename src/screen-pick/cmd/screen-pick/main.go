// Command screen-pick finds paths, URLs, hosts and commit hashes in a
// captured kitty screen and offers actions on the one you pick.
//
//	screen-pick hints|list [-window-id N]  < screen
//	screen-pick scan [-full] [-index] [-json] [-width N] < screen
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
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

type options struct {
	verb      string
	windowID  int
	width     int
	full      bool
	withIndex bool
	asJSON    bool
}

var verbs = []string{"hints", "list", "scan"}

// parseArgs reads the verb and its flags, rejecting the flags the verb does not
// take before any input is read.
func parseArgs(args []string, out io.Writer) (options, error) {
	if len(args) == 0 {
		return options{}, usage()
	}
	var o options
	o.verb = args[0]
	if !slices.Contains(verbs, o.verb) {
		return options{}, fmt.Errorf("unknown verb %q: %w", o.verb, usage())
	}
	fs := flag.NewFlagSet(o.verb, flag.ContinueOnError)
	fs.SetOutput(out)
	fs.IntVar(&o.windowID, "window-id", 0, "kitty window the screen came from")
	fs.IntVar(&o.width, "width", 0, "screen width in cells (default: the terminal's)")
	fs.BoolVar(&o.full, "full", false, "scan: probe kitty and /proc as the picker does")
	fs.BoolVar(&o.withIndex, "index", false, "scan: also search git indexes for unresolved paths")
	fs.BoolVar(&o.asJSON, "json", false, "scan: print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return options{}, err
	}
	var scanOnly string
	fs.Visit(func(f *flag.Flag) {
		if o.verb != "scan" && slices.Contains([]string{"full", "index", "json"}, f.Name) {
			scanOnly = f.Name
		}
	})
	switch {
	case scanOnly != "":
		return options{}, fmt.Errorf("-%s only applies to scan: %w", scanOnly, usage())
	case fs.NArg() > 0:
		return options{}, fmt.Errorf("unexpected argument %q: %w", fs.Arg(0), usage())
	case o.windowID < 0 || o.width < 0:
		return options{}, fmt.Errorf("-window-id and -width must not be negative")
	}
	return o, nil
}

func run(args []string) error {
	o, err := parseArgs(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read screen: %w", err)
	}
	if o.width == 0 {
		if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil {
			o.width = w
		}
	}

	switch o.verb {
	case "hints", "list":
		mode := tui.Hints
		if o.verb == "list" {
			mode = tui.List
		}
		return pick(string(text), mode, o.width, o.windowID)
	}
	return scan(string(text), o.width, o.windowID, o.full, o.withIndex, o.asJSON)
}

func usage() error {
	return fmt.Errorf("usage: screen-pick hints|list|scan [flags] < screen-text")
}

func pick(text string, mode tui.Mode, width, windowID int) error {
	s := newSession(text, width, windowID)
	defer s.cancel()
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
	Start  int      `json:"start"`
	End    int      `json:"end"`
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
				merger.ApplyMatch(&ms[res.ID], res)
			}
		}
	}
	elapsed := time.Since(start)

	rows := make([]scanRow, len(ms))
	for i, m := range ms {
		rows[i] = scanRow{Kind: m.Kind.String(), Text: m.Text, Start: m.Start, End: m.End, Line: m.Line, Col: m.Col, Path: m.Path, Exists: m.Exists}
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
