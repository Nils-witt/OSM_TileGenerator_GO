package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTilesDir creates a tiles directory holding a single 0/0/0.png tile with
// the given body.
func newTilesDir(t *testing.T, body string) string {
	t.Helper()

	tilesDir := t.TempDir()

	xDir := filepath.Join(tilesDir, "0", "0")
	if err := os.MkdirAll(xDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(xDir, "0.png"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return tilesDir
}

func TestHandler(t *testing.T) {
	t.Parallel()

	want := "fake tile bytes"
	server := httptest.NewServer(Handler(newTilesDir(t, want)))
	// Cleanup, not defer: the parallel subtests run after this function returns.
	t.Cleanup(server.Close)

	t.Run("serves tiles with cors and no-cache headers", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/0/0/0.png", nil)
		if err != nil {
			t.Fatalf("NewRequestWithContext() error = %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}

		defer func() { _ = resp.Body.Close() }()

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
	})

	t.Run("answers preflight requests with no content", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, server.URL+"/0/0/0.png", nil)
		if err != nil {
			t.Fatalf("NewRequestWithContext() error = %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}

		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
		}

		if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "GET, HEAD, OPTIONS" {
			t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, "GET, HEAD, OPTIONS")
		}
	})
}

func TestListenAndServe(t *testing.T) {
	t.Parallel()

	t.Run("fails when the tiles directory is missing", func(t *testing.T) {
		t.Parallel()

		if err := ListenAndServe(slog.New(slog.DiscardHandler), filepath.Join(t.TempDir(), "tiles"), 0); err == nil {
			t.Fatal("ListenAndServe() error = nil, want error for missing tiles directory")
		}
	})

	t.Run("fails when the tiles path is a file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "tiles")
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		if err := ListenAndServe(slog.New(slog.DiscardHandler), path, 0); err == nil {
			t.Fatal("ListenAndServe() error = nil, want error for non-directory tiles path")
		}
	})
}
