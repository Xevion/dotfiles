package tui

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"screenpick/internal/detect"
	"screenpick/internal/fakefs"
	"screenpick/internal/lookup"
	"screenpick/internal/resolve"
)

var fuzzScreens = []string{
	screen,
	"",
	"\n\n\n",
	"日本語/ファイル.txt:3 and 日本/x.rs\n/etc/hosts 🙂 https://例え.jp/パス\r\nlast",
	"é/café.go:1:2 áb́.rs /etc/hosts\n\u200b/etc/hosts\u200d",
	strings.Repeat("x", 300) + " /etc/hosts " + strings.Repeat("y/", 150) + "z.go\nshort",
	strings.Repeat("/etc/hosts src/main.rs https://x.y/a 4ecb91bf\n", 120),
	"a/b c/d e/f g/h i/j k/l m/n o/p q/r s/t u/v w/x y/z",
	"see /etc/ho\rsts and src/ma\rin.rs:12\r\nhttps://x.y/a\r\n\r\nlast\r",
	"/etc/hosts\nsrc/main.rs:12\n\n\n",
}

func fuzzModel(t testing.TB, sc string, w, h int, mode Mode) (*Model, *fakeBackend) {
	disk := fakefs.New("/etc/hosts", "/home/u/proj/src/main.rs", "/home/u/proj/b/src/main.rs", "/home/u/proj/日本語/ファイル.txt", "/home/u/proj/café.go")
	r := resolve.New(resolve.Env{
		Bases: []resolve.Base{{Dir: "/home/u/proj", Source: resolve.SourceProcess}},
		Home:  "/home/u", Lstat: disk.Lstat, Readlink: disk.Readlink, ReadDir: disk.ReadDir,
	})
	ms := detect.ScanWidth(sc, r, 80)
	b := &fakeBackend{disk: disk, refined: ms}
	b.deep = []lookup.Result{{ID: 0, Path: "/mnt/x/etc/hosts", Source: resolve.SourceSearch}, {ID: 1, Path: "/home/u/proj/b/src/main.rs", Source: resolve.SourceSearch}}
	m := New(Config{Mode: mode, Screen: sc, Matches: ms, Backend: b, Home: "/home/u"})
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m, b
}

var fuzzKeys = []tea.KeyPressMsg{
	{Code: tea.KeyEscape}, {Code: tea.KeyEnter}, {Code: tea.KeyTab}, {Code: tea.KeyBackspace},
	{Code: tea.KeyDown}, {Code: tea.KeyUp}, {Code: tea.KeyLeft}, {Code: tea.KeyRight},
	{Code: 'p', Mod: tea.ModCtrl}, {Code: 'n', Mod: tea.ModCtrl}, {Code: tea.KeyTab, Mod: tea.ModShift},
	{Code: 'q', Mod: tea.ModCtrl}, {Code: 'c', Mod: tea.ModCtrl},
	{Code: tea.KeyPgUp}, {Code: tea.KeyPgDown}, {Code: tea.KeyHome}, {Code: tea.KeyEnd},
	{Code: 'u', Mod: tea.ModCtrl}, {Code: 'd', Mod: tea.ModCtrl},
}

const fuzzText = "asdfghjklwertyuiopzxcvbnm/qyoftrRSAé日́ 🙂ab"

// play interprets ops as a session of keys, resizes and messages against a
// model, checking after each that nothing panics and the frame fits.
func play(t testing.TB, screenIdx int, ops []byte) {
	if len(ops) < 3 {
		return
	}
	sc := fuzzScreens[screenIdx%len(fuzzScreens)]
	w, h := 1+int(ops[0])%140, 1+int(ops[1])%40
	mode := Mode(ops[2] % 2)
	m, b := fuzzModel(t, sc, w, h, mode)
	runes := []rune(fuzzText)
	for i := 3; i < len(ops); i++ {
		op := ops[i]
		var msg tea.Msg
		switch {
		case op < 120:
			r := runes[int(op)%len(runes)]
			msg = tea.KeyPressMsg{Code: r, Text: string(r)}
		case op < 190:
			msg = fuzzKeys[int(op)%len(fuzzKeys)]
		case op < 225 && i+2 < len(ops):
			w, h = 1+int(ops[i+1])%160, 1+int(ops[i+2])%45
			i += 2
			msg = tea.WindowSizeMsg{Width: w, Height: h}
		case op < 235:
			msg = refinedMsg{b.refined}
		case op < 245:
			msg = quickMsg{b.refined}
		case op < 250:
			closed := make(chan lookup.Result)
			close(closed)
			id := int(op) % max(len(m.matches), 1)
			msg = resultMsg{r: lookup.Result{ID: id, Path: fmt.Sprintf("/mnt/c%d/src/main.rs", op), Source: resolve.SourceIndex, Rank: int(op) % 3}, ch: closed}
		default:
			msg = tea.KeyPressMsg{Code: 'v', Text: strings.Repeat("v", 200)}
		}
		_, cmd := m.Update(msg)
		drain(m, cmd)
		if v := m.View().Content; len(strings.Split(v, "\n")) > h {
			t.Fatalf("screen %d %dx%d after op %d (%v): %d rows, mode %v\n%s", screenIdx, w, h, i, msg, len(strings.Split(v, "\n")), m.mode, v)
		}
		checkFitsTB(t, m, w, h, fmt.Sprintf("screen %d op %d %v", screenIdx, i, msg))
		if t.Failed() {
			return
		}
	}
}

func TestRandomSessions(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	for n := range 160 {
		ops := make([]byte, 3+rng.IntN(50))
		for i := range ops {
			ops[i] = byte(rng.IntN(256))
		}
		t.Run(fmt.Sprint(n), func(t *testing.T) { play(t, n, ops) })
	}
}

func FuzzSession(f *testing.F) {
	f.Add(3, []byte{80, 20, 0, 3, 4, 5})
	f.Add(0, []byte{10, 3, 1, 200, 201, 5, 130, 140})
	f.Add(3, []byte{5, 4, 0, 130, 130, 135, 200, 3, 3})
	f.Fuzz(func(t *testing.T, screen int, ops []byte) {
		if screen < 0 {
			screen = -screen
		}
		play(t, screen, ops)
	})
}
