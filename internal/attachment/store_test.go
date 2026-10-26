package attachment

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pngBytes 是一个最小的 PNG 文件头（足够让文件头嗅探通过）。
func pngBytes() []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x00}, 32)...)
}

func TestSaveWritesFileAndReturnsMarkdown(t *testing.T) {
	dataDir := t.TempDir()
	store := NewStore(dataDir)

	res, err := store.Save(bytes.NewReader(pngBytes()))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if !strings.HasPrefix(res.Path, DirName+"/") || !strings.HasSuffix(res.Path, ".png") {
		t.Errorf("unexpected path %q", res.Path)
	}
	if res.Mime != "image/png" {
		t.Errorf("mime = %q, want image/png", res.Mime)
	}
	wantMD := "![](../" + res.Path + ")"
	if res.Markdown != wantMD {
		t.Errorf("markdown = %q, want %q", res.Markdown, wantMD)
	}
	wantURL := "/api/" + res.Path
	if res.URL != wantURL {
		t.Errorf("url = %q, want %q", res.URL, wantURL)
	}

	abs := filepath.Join(dataDir, filepath.FromSlash(res.Path))
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if !bytes.Equal(got, pngBytes()) {
		t.Error("stored content differs from upload")
	}
}

func TestSaveIsContentAddressed(t *testing.T) {
	store := NewStore(t.TempDir())

	first, err := store.Save(bytes.NewReader(pngBytes()))
	if err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	second, err := store.Save(bytes.NewReader(pngBytes()))
	if err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	if first.Path != second.Path {
		t.Errorf("same content stored twice: %q vs %q", first.Path, second.Path)
	}
}

func TestSaveDetectsFormats(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", pngBytes(), ".png"},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, ".jpg"},
		{"gif", []byte("GIF89a........"), ".gif"},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), ".webp"},
		{"bmp", []byte("BM\x00\x00\x00\x00"), ".bmp"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := NewStore(t.TempDir()).Save(bytes.NewReader(tc.data))
			if err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			if !strings.HasSuffix(res.Path, tc.want) {
				t.Errorf("path = %q, want suffix %q", res.Path, tc.want)
			}
		})
	}
}

func TestSaveRejectsUnsupportedType(t *testing.T) {
	_, err := NewStore(t.TempDir()).Save(strings.NewReader("not an image"))
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("err = %v, want ErrUnsupportedType", err)
	}
}

func TestSaveRejectsTooLarge(t *testing.T) {
	big := bytes.Repeat([]byte{0x00}, int(MaxSize)+1)
	_, err := NewStore(t.TempDir()).Save(bytes.NewReader(big))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestSaveRejectsEmpty(t *testing.T) {
	if _, err := NewStore(t.TempDir()).Save(bytes.NewReader(nil)); err == nil {
		t.Fatal("expected error for empty upload")
	}
}

func TestAbsPath(t *testing.T) {
	dataDir := t.TempDir()
	store := NewStore(dataDir)

	res, err := store.Save(bytes.NewReader(pngBytes()))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	relPath := strings.TrimPrefix(res.Path, DirName+"/")
	parts := strings.SplitN(relPath, "/", 2)

	abs, err := store.AbsPath(parts[0], parts[1])
	if err != nil {
		t.Fatalf("AbsPath() error = %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Errorf("stat %q: %v", abs, err)
	}

	bad := []struct{ month, name string }{
		{"2026-10", "../.config.json"},
		{"../2026-10", parts[1]},
		{"2026-13", parts[1]},
		{"2026-1", parts[1]},
		{"2026-10", "not-a-hash.png"},
		{"2026-10", ""},
	}
	for _, b := range bad {
		if _, err := store.AbsPath(b.month, b.name); err == nil {
			t.Errorf("AbsPath(%q, %q) should fail", b.month, b.name)
		}
	}
}

func TestContentType(t *testing.T) {
	cases := map[string]string{
		"a.png":  "image/png",
		"a.jpg":  "image/jpeg",
		"a.gif":  "image/gif",
		"a.webp": "image/webp",
		"a.bmp":  "image/bmp",
		"a.txt":  "application/octet-stream",
	}
	for name, want := range cases {
		if got := ContentType(name); got != want {
			t.Errorf("ContentType(%q) = %q, want %q", name, got, want)
		}
	}
}
