package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirPaths(t *testing.T) {
	t.Parallel()

	d := Dir(filepath.FromSlash("/proj"))
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ConfigPath", d.ConfigPath(), "/proj/config.json"},
		{"ImagePath", d.ImagePath(), "/proj/image.png"},
		{"TilesDir", d.TilesDir(), "/proj/tiles"},
		{"TmpDir", d.TmpDir(), "/proj/tmp"},
		{"ZipPath", d.ZipPath(), "/proj/tiles.zip"},
	}
	for _, tt := range tests {
		if want := filepath.FromSlash(tt.want); tt.got != want {
			t.Errorf("%s() = %q, want %q", tt.name, tt.got, want)
		}
	}
}

func TestDirCreate(t *testing.T) {
	t.Parallel()

	t.Run("creates project, tiles, and tmp directories", func(t *testing.T) {
		t.Parallel()
		d := Dir(filepath.Join(t.TempDir(), "project"))

		if err := d.Create(); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		for _, dir := range []string{string(d), d.TilesDir(), d.TmpDir()} {
			info, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("stat %s: %v", dir, err)
			}
			if !info.IsDir() {
				t.Errorf("%s exists but is not a directory", dir)
			}
		}
	})

	t.Run("is idempotent when directories already exist", func(t *testing.T) {
		t.Parallel()
		d := Dir(filepath.Join(t.TempDir(), "project"))

		if err := d.Create(); err != nil {
			t.Fatalf("first Create() error = %v", err)
		}
		if err := d.Create(); err != nil {
			t.Fatalf("second Create() error = %v", err)
		}
	})
}
