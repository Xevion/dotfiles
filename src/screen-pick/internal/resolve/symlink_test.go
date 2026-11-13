package resolve

import (
	"testing"

	"screenpick/internal/fakefs"
)

func TestResolveFollowsSymlinkForIsDir(t *testing.T) {
	fs := fakefs.New(
		"/home/u/proj/src/main.rs",
		"/home/u/proj/srclink -> src",
		"/home/u/proj/abslink -> /home/u/proj/src",
		"/home/u/proj/chain -> srclink",
		"/home/u/proj/filelink -> src/main.rs",
		"/home/u/proj/broken -> nowhere",
		"/home/u/proj/loopa -> loopb",
		"/home/u/proj/loopb -> loopa",
		"/home/u/up/dirlink -> ../proj/src",
	)
	r := New(Env{
		Bases:    []Base{{Dir: "/home/u/proj", Source: SourceProcess}},
		Home:     "/home/u",
		Getenv:   func(string) string { return "" },
		Lstat:    fs.Lstat,
		Readlink: fs.Readlink,
		ReadDir:  fs.ReadDir,
	})
	cases := []struct {
		token string
		isDir bool
	}{
		{"srclink", true},
		{"abslink", true},
		{"chain", true},
		{"/home/u/up/dirlink", true},
		{"filelink", false},
		{"broken", false},
		{"loopa", false},
		{"src", true},
		{"src/main.rs", false},
	}
	for _, tc := range cases {
		t.Run(tc.token, func(t *testing.T) {
			res := r.Resolve(tc.token, nil)
			if !res.Exists {
				t.Fatalf("%s does not exist: %+v", tc.token, res)
			}
			if res.IsDir != tc.isDir {
				t.Errorf("IsDir = %v, want %v", res.IsDir, tc.isDir)
			}
		})
	}
}
