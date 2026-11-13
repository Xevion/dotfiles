package resolve

import (
	"io/fs"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestMountPoints(t *testing.T) {
	data := []byte(`26 1 259:2 / / rw,relatime shared:1 - ext4 /dev/nvme0n1p2 rw
34 26 0:25 / /mnt/nfs rw,relatime shared:7 - autofs /etc/auto.nfsdb rw
40 26 0:31 / /mnt/with\040space rw - tmpfs tmpfs rw
`)
	want := []string{"/", "/mnt/nfs", "/mnt/with space"}
	if diff := cmp.Diff(want, mountPoints(data)); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// One dead mount must cost one budget, and every later check under it must
// fail at once without reaching the disk.
func TestGuardSkipsASlowMount(t *testing.T) {
	g := NewGuard(20 * time.Millisecond)
	g.parsed, g.mounts = true, []string{"/", "/mnt/nfs"}

	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	env := g.Wrap(Env{Lstat: func(p string) (fs.FileInfo, error) {
		calls.Add(1)
		if p == "/mnt/nfs/media/a" {
			<-release
		}
		return nil, fs.ErrNotExist
	}})

	start := time.Now()
	if _, err := env.Lstat("/mnt/nfs/media/a"); err != ErrSlow {
		t.Fatalf("a call outlasting the budget gives ErrSlow, got %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("gave up after %v, want about the 20ms budget", d)
	}

	before := calls.Load()
	if _, err := env.Lstat("/mnt/nfs/temp/b"); err != ErrSlow {
		t.Errorf("a sibling under the same mount is skipped, got %v", err)
	}
	if calls.Load() != before {
		t.Errorf("the skipped check still reached the disk")
	}
	if _, err := env.Lstat("/home/u/x"); err != fs.ErrNotExist {
		t.Errorf("other mounts are unaffected, got %v", err)
	}
}

// A stall on the root filesystem skips only that path: skipping "/" would
// blind every later lookup.
func TestGuardRootFilesystemSkipsOnlyThePath(t *testing.T) {
	g := NewGuard(10 * time.Millisecond)
	g.parsed, g.mounts = true, []string{"/"}
	release := make(chan struct{})
	defer close(release)
	env := g.Wrap(Env{Lstat: func(p string) (fs.FileInfo, error) {
		if p == "/home/u/stuck" {
			<-release
		}
		return nil, fs.ErrNotExist
	}})
	if _, err := env.Lstat("/home/u/stuck"); err != ErrSlow {
		t.Fatalf("want ErrSlow, got %v", err)
	}
	if _, err := env.Lstat("/home/u/other"); err != fs.ErrNotExist {
		t.Errorf("a neighbour on the same filesystem is still checked, got %v", err)
	}
}

func BenchmarkGuardedLstat(b *testing.B) {
	env := NewGuard(time.Second).Wrap(Env{})
	b.ReportAllocs()
	for b.Loop() {
		env.Lstat("/etc/hosts")
	}
}
