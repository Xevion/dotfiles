// Package filetype tells what a file is from its name and first bytes, so the
// picker offers an editor only for text and the right tools for the rest.
package filetype

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

type Class uint8

const (
	Unknown Class = iota
	Text
	Image
	Document
	Archive
	Media
	// Executable is a native program or shared object.
	Executable
	// WindowsExe is a PE image, runnable under Wine.
	WindowsExe
	// Binary is any other non-text data.
	Binary
)

// Type is a file's class and media type.
type Type struct {
	Class Class
	Mime  string
}

// HeadSize is how much of a file Sniff looks at.
const HeadSize = 8192

// Head reads the start of the file at p.
func Head(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, HeadSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf[:n], nil
}

// Sniff classifies a file by its first bytes, using the name only where the
// content is ambiguous.
func Sniff(name string, head []byte) Type {
	switch {
	case bytes.HasPrefix(head, []byte("\x7fELF")):
		return Type{Executable, "application/x-executable"}
	case bytes.HasPrefix(head, []byte("MZ")):
		return Type{WindowsExe, "application/vnd.microsoft.portable-executable"}
	case len(head) == 0:
		return Type{Text, "text/plain"}
	}
	mt, _, _ := strings.Cut(http.DetectContentType(head), ";")
	switch {
	case strings.HasPrefix(mt, "text/"):
		if byExt := mimeByName(name); byExt != "" && !strings.HasPrefix(byExt, "text/") && ByName(name) != Text {
			return Type{ByName(name), byExt}
		}
		return Type{Text, mt}
	case strings.HasPrefix(mt, "image/"):
		return Type{Image, mt}
	case strings.HasPrefix(mt, "audio/"), strings.HasPrefix(mt, "video/"):
		return Type{Media, mt}
	case mt == "application/pdf", mt == "application/postscript":
		return Type{Document, mt}
	case mt == "application/zip", mt == "application/x-gzip", mt == "application/x-rar-compressed", mt == "application/x-7z-compressed", mt == "application/vnd.rar":
		return Type{Archive, mt}
	}
	if bytes.IndexByte(head, 0) < 0 && utf8.Valid(trimPartialRune(head)) {
		return Type{Text, "text/plain"}
	}
	if c := ByName(name); c != Unknown && c != Text {
		return Type{c, or(mimeByName(name), mt)}
	}
	return Type{Binary, or(mimeByName(name), mt)}
}

// ByName classifies by extension alone, for when reading is not worth it.
func ByName(name string) Class {
	return extClass[strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))]
}

var extClass = map[string]Class{}

func init() {
	for class, exts := range map[Class]string{
		Image:      "png jpg jpeg gif webp bmp ico svg tif tiff avif heic psd",
		Document:   "pdf epub djvu ps",
		Archive:    "zip tar gz tgz xz txz zst bz2 7z rar jar deb rpm apk iso",
		Media:      "mp3 mp4 mkv webm mov avi wav flac ogg opus m4a",
		WindowsExe: "exe dll msi sys scr",
		Executable: "so o a ko elf appimage",
		Binary:     "bin dat db sqlite pak pck wasm class pyc",
		Text:       "txt md log json jsonc yaml yml toml ini conf cfg xml html css js ts go rs py rb sh fish lua c h cpp hpp java kt cs",
	} {
		for e := range strings.FieldsSeq(exts) {
			extClass[e] = class
		}
	}
}

func mimeByName(name string) string {
	mt, _, _ := strings.Cut(mime.TypeByExtension(path.Ext(name)), ";")
	return mt
}

// trimPartialRune drops a multibyte rune cut off by the read limit.
func trimPartialRune(b []byte) []byte {
	for i := 0; i < utf8.UTFMax && i < len(b); i++ {
		if utf8.Valid(b[:len(b)-i]) {
			return b[:len(b)-i]
		}
	}
	return b
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
