package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTilesHandler(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	tilesDir := filepath.Join(configDir, "tiles", "0", "0")
	if err := os.MkdirAll(tilesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	want := "fake tile bytes"
	if err := os.WriteFile(filepath.Join(tilesDir, "0.png"), []byte(want), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	server := httptest.NewServer(tilesHandler(configDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/0/0/0.png")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}

	if got := resp.Header.Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache, no-store, must-revalidate")
	}

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestServeTilesMissingDir(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()

	if err := serveTiles(configDir, 8080); err == nil {
		t.Fatal("serveTiles() error = nil, want error for missing tiles directory")
	}
}
