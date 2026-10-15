package tilestore

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZip(t *testing.T) {
	t.Parallel()

	tilesDir := t.TempDir()

	want := map[string]string{
		"14/8192/5461.png": "tile-a",
		"14/8193/5461.png": "tile-b",
		"13/4096/2730.png": "tile-c",
		"README.txt":       "not a tile",
	}
	for rel, body := range want {
		path := filepath.Join(tilesDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}

		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	destPath := filepath.Join(t.TempDir(), "tiles.zip")
	if err := Zip(tilesDir, destPath); err != nil {
		t.Fatalf("Zip() error = %v", err)
	}

	r, err := zip.OpenReader(destPath)
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	defer func() { _ = r.Close() }()

	got := map[string]string{}

	for _, f := range r.File {
		// Already-compressed PNGs are stored; anything else is deflated.
		wantMethod := zip.Deflate
		if strings.HasSuffix(f.Name, ".png") {
			wantMethod = zip.Store
		}

		if f.Method != wantMethod {
			t.Errorf("entry %s method = %d, want %d", f.Name, f.Method, wantMethod)
		}

		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}

		data, err := io.ReadAll(rc)
		if cerr := rc.Close(); err == nil {
			err = cerr
		}

		if err != nil {
			t.Fatalf("read entry %s: %v", f.Name, err)
		}

		got[f.Name] = string(data)
	}

	if len(got) != len(want) {
		t.Fatalf("archive has %d entries, want %d: %v", len(got), len(want), got)
	}

	for name, body := range want {
		if got[name] != body {
			t.Errorf("entry %s = %q, want %q", name, got[name], body)
		}
	}
}

func TestZipMissingSource(t *testing.T) {
	t.Parallel()

	destPath := filepath.Join(t.TempDir(), "tiles.zip")
	if err := Zip(filepath.Join(t.TempDir(), "missing"), destPath); err == nil {
		t.Fatal("Zip() error = nil, want error for missing source directory")
	}
}

func TestZipRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()

	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tilesDir := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(tilesDir, "leak.png")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "tiles.zip")
	if err := Zip(tilesDir, destPath); err == nil {
		t.Fatal("Zip() error = nil, want error for symlink escaping the tiles directory")
	}
}
