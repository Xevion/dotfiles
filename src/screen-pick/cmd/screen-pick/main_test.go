package main

import (
	"errors"
	"flag"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want options
		err  bool
	}{
		{"hints with window", []string{"hints", "-window-id", "42"}, options{verb: "hints", windowID: 42}, false},
		{"scan flags", []string{"scan", "-full", "-index", "-json", "-width", "80"}, options{verb: "scan", full: true, withIndex: true, asJSON: true, width: 80}, false},
		{"list", []string{"list"}, options{verb: "list"}, false},
		{"no verb", nil, options{}, true},
		{"unknown verb", []string{"bogus"}, options{}, true},
		{"flag before verb", []string{"-window-id", "5", "hints"}, options{}, true},
		{"stray argument", []string{"hints", "extra"}, options{}, true},
		{"unsubstituted placeholder", []string{"hints", "-window-id", "@active-kitty-window-id"}, options{}, true},
		{"negative width", []string{"scan", "-width", "-5"}, options{}, true},
		{"negative window", []string{"hints", "-window-id", "-1"}, options{}, true},
		{"scan flag on hints", []string{"hints", "-json", "-full", "-index"}, options{}, true},
		{"unknown flag", []string{"hints", "-nope"}, options{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseArgs(tc.args, io.Discard)
			if (err != nil) != tc.err {
				t.Fatalf("err = %v, want error: %v", err, tc.err)
			}
			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(options{}), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseArgsHelpIsNotAFailure(t *testing.T) {
	if _, err := parseArgs([]string{"hints", "-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("err = %v, want flag.ErrHelp", err)
	}
}
