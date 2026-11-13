package filetype

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSniff(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	cases := []struct {
		name string
		head []byte
		want Class
	}{
		{"main.rs", []byte("fn main() {}\n"), Text},
		{"empty.txt", nil, Text},
		{"no-extension", []byte("#!/bin/sh\necho hi\n"), Text},
		{"utf8 cut by the read limit", []byte("caf\xc3"), Text},
		{"tool", []byte("\x7fELF\x02\x01\x01\x00"), Executable},
		{"app.exe", []byte("MZ\x90\x00\x03\x00"), WindowsExe},
		{"misnamed.txt", []byte("MZ\x90\x00"), WindowsExe},
		{"shot.png", png, Image},
		{"doc.pdf", []byte("%PDF-1.7\n"), Document},
		{"a.zip", []byte("PK\x03\x04\x14\x00"), Archive},
		{"save.dat", []byte("\x00\x01\x02\x03garbage"), Binary},
		{"notes.txt", []byte("MZ Corp meeting notes\nagenda\n"), Text},
		{"readings.dat", []byte("0.1 0.2 0.3\n0.4 0.5 0.6\n"), Text},
		{"table.bin", []byte("id,name\n1,a\n"), Text},
		{"icon.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), Image},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sniff(tc.name, tc.head); got.Class != tc.want {
				t.Errorf("Sniff(%q) = %+v, want class %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestHead(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big")
	if err := os.WriteFile(p, make([]byte, HeadSize*2), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := Head(p); err != nil || len(b) != HeadSize {
		t.Errorf("Head read %d bytes, err %v", len(b), err)
	}
	if _, err := Head(filepath.Join(dir, "gone")); err == nil {
		t.Error("a missing file should error")
	}
}

// Opening a FIFO for reading blocks until a writer shows up, which would hang
// the menu of a path that merely names one.
func TestHeadSkipsFifo(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skip("no fifo support:", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Head(p)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a fifo has no head to read")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Head blocked on a fifo")
	}
}

func TestHeadSkipsDirectory(t *testing.T) {
	if _, err := Head(t.TempDir()); err == nil {
		t.Error("a directory has no head to read")
	}
}
