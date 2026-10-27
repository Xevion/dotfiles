package probe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Trimmed from real `kitten @ ls --match id:N` output.
const kittyLSOutput = `[{"id": 1, "tabs": [{"id": 3, "windows": [
  {"id": 1969, "cwd": "/elsewhere", "foreground_processes": []},
  {"id": 1970, "cwd": "/home/u/old", "foreground_processes": [
    {"pid": 3725386, "cwd": "/home/u/work", "cmdline": ["claude"]},
    {"pid": 3725574, "cwd": "/home/u/work", "cmdline": ["uv"]}
  ]}
]}]}]`

func TestParseKittyLS(t *testing.T) {
	w, err := parseKittyLS([]byte(kittyLSOutput), 1970)
	if err != nil {
		t.Fatal(err)
	}
	if w.Cwd != "/home/u/old" || len(w.Foreground) != 2 || w.Foreground[0].PID != 3725386 {
		t.Errorf("unexpected window: %+v", w)
	}
	if _, err := parseKittyLS([]byte(kittyLSOutput), 42); err == nil {
		t.Error("want an error for a missing window")
	}
	if _, err := parseKittyLS([]byte("not json"), 1970); err == nil {
		t.Error("want an error for bad json")
	}
}

func TestDescendants(t *testing.T) {
	tree := map[int][]int{1: {2, 3}, 2: {4}, 4: {5}, 3: {2}, 9: {10}}
	if diff := cmp.Diff([]int{1, 2, 3, 4, 5}, descendants(tree, []int{1})); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]int{9, 4, 10, 5}, descendants(tree, []int{9, 4})); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestWineVars(t *testing.T) {
	got := wineVars([]string{"PATH=/bin", "WINEPREFIX=/opt/pfx", "STEAM_COMPAT_DATA_PATH=/s/compatdata/1", "WINEPREFIXX=no"})
	want := map[string]string{"WINEPREFIX": "/opt/pfx", "STEAM_COMPAT_DATA_PATH": "/s/compatdata/1"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestSteamLibraries(t *testing.T) {
	vdf := `"libraryfolders"
{
	"0"
	{
		"path"		"/home/u/.local/share/Steam"
		"apps" { "100" "1" }
	}
	"1"
	{
		"path"		"/mnt/data/Steam Library"
	}
	"2"
	{
		"path"		"D:\\Library"
	}
}`
	want := []string{"/home/u/.local/share/Steam", "/mnt/data/Steam Library", `D:\Library`}
	if diff := cmp.Diff(want, steamLibraries([]byte(vdf))); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestPrefixesOnDisk(t *testing.T) {
	home := t.TempDir()
	lib2 := filepath.Join(t.TempDir(), "Lib")
	mk := func(p string) {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mk(filepath.Join(home, ".wine/dosdevices"))
	mk(filepath.Join(home, ".local/share/Steam/steamapps/compatdata/100/pfx/dosdevices"))
	mk(filepath.Join(home, ".local/share/Steam/steamapps/compatdata/200/pfx")) // no dosdevices yet
	mk(filepath.Join(lib2, "steamapps/compatdata/300/pfx/dosdevices"))
	vdf := `"libraryfolders" { "0" { "path" "` + filepath.Join(home, ".local/share/Steam") + `" } "1" { "path" "` + lib2 + `" } }`
	if err := os.WriteFile(filepath.Join(home, ".local/share/Steam/steamapps/libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(home, ".wine"),
		filepath.Join(home, ".local/share/Steam/steamapps/compatdata/100/pfx"),
		filepath.Join(lib2, "steamapps/compatdata/300/pfx"),
	}
	if diff := cmp.Diff(want, PrefixesOnDisk(home)); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}
