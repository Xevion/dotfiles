package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"screenpick/internal/detect"
	"screenpick/internal/resolve"
)

// benchScreen is rows lines of the mix a terminal full of build output holds:
// paths with locations, URLs, hosts, hashes and filler.
func benchScreen(rows int) string {
	var b strings.Builder
	for i := range rows {
		switch i % 7 {
		case 0:
			fmt.Fprintf(&b, "internal/tui/model.go:%d:%d: error: something went wrong near here\n", i, i%80)
		case 1:
			fmt.Fprintf(&b, "  --> /home/u/proj/internal/detect/scan.go:%d\n", i)
		case 2:
			fmt.Fprintf(&b, "fetch https://github.com/u/repo/blob/master/src/f%d.go 200 OK commit %08xa\n", i, i*2654435761)
		case 3:
			fmt.Fprintf(&b, "~/.config/kitty/kitty.conf listening on 127.0.0.1:%d\n", 1000+i)
		case 4:
			fmt.Fprintf(&b, "./cmd/main.go  ../other/lib%d.rs  /etc/hosts  missing/nope/x%d.go\n", i, i)
		case 5:
			fmt.Fprintf(&b, "ok  \tscreenpick/internal/pkg%d\t0.%03ds\n", i%9, i%1000)
		default:
			b.WriteString("plain output line with nothing at all of interest on it, just filler words\n")
		}
	}
	return b.String()
}

// BenchmarkFirstFrame is everything between reading the screen and handing
// the first frame to the renderer, with the disk out of the picture: a
// textual scan, the model, one View.
func BenchmarkFirstFrame(b *testing.B) {
	for _, rows := range []int{45, 400, 3000} {
		screen := benchScreen(rows)
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				r := resolve.New(resolve.NoDisk("/home/u"))
				ms := detect.ScanWidth(screen, r, 200)
				m := New(Config{Mode: Hints, Screen: screen, Matches: ms, Backend: &fakeBackend{}, Home: "/home/u", Quick: nil})
				m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
				if m.View().Content == "" {
					b.Fatal("empty frame")
				}
			}
		})
	}
}
