// Package server serves a generated tile pyramid over HTTP.
package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// readHeaderTimeout bounds how long a client may take to send request
// headers, so idle or slow connections cannot pin the server's goroutines.
const readHeaderTimeout = 10 * time.Second

// Handler serves the {z}/{x}/{y}.png files under tilesDir with permissive
// CORS headers and caching disabled, so map clients always see freshly
// generated tiles.
func Handler(tilesDir string) http.Handler {
	return tileResponseHeaders(http.FileServer(http.Dir(tilesDir)))
}

func tileResponseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ListenAndServe serves tilesDir on the given port until the server fails,
// logging startup and server errors to logger. It returns an error without
// listening if tilesDir is missing or is not a directory.
func ListenAndServe(logger *slog.Logger, tilesDir string, port int) error {
	info, err := os.Stat(tilesDir)
	if err != nil {
		return fmt.Errorf("tiles directory %q not found (generate tiles first): %w", tilesDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("tiles path %q is not a directory", tilesDir)
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           Handler(tilesDir),
		ReadHeaderTimeout: readHeaderTimeout,
		// Route net/http's internal errors (e.g. TLS handshakes, panics in
		// handlers) through the same structured logger.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	logger.Info("serving tiles",
		"dir", tilesDir,
		"url", "http://localhost"+srv.Addr)
	return srv.ListenAndServe()
}
