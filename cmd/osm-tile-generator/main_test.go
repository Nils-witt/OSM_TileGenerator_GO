package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDirectories(t *testing.T) {
	t.Parallel()

	t.Run("creates config, tiles, and tmp directories", func(t *testing.T) {
		t.Parallel()
		configDir := filepath.Join(t.TempDir(), "project")

		if err := createDirectories(configDir); err != nil {
			t.Fatalf("createDirectories() error = %v", err)
		}

		for _, dir := range []string{configDir, filepath.Join(configDir, "tiles"), filepath.Join(configDir, "tmp")} {
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
		configDir := filepath.Join(t.TempDir(), "project")

		if err := createDirectories(configDir); err != nil {
			t.Fatalf("first createDirectories() error = %v", err)
		}
		if err := createDirectories(configDir); err != nil {
			t.Fatalf("second createDirectories() error = %v", err)
		}
	})
}
