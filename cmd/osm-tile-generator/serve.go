package main

import (
	"flag"
	"io"
	"log/slog"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/server"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/version"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/workspace"
)

const (
	defaultPort = 8080
	maxPort     = 65535
)

func runServe(args []string, stderr io.Writer) error {
	fs := newFlagSet("serve", "Serve <config-dir>/tiles over HTTP with CORS enabled.", stderr)
	configDir := fs.String("config-dir", "config", "Directory whose tiles/ subdirectory is served")
	port := fs.Int("port", defaultPort, "TCP port to listen on")

	var logs logFlags
	logs.register(fs)

	if err := parseFlags(fs, args); err != nil {
		return err
	}

	if err := validatePort(fs, "-port", *port); err != nil {
		return err
	}

	return serve(*configDir, *port, logs.newLogger(stderr))
}

func validatePort(fs *flag.FlagSet, flagName string, port int) error {
	if port < 1 || port > maxPort {
		return usageErrorf(fs, "%s must be between 1 and %d, got %d", flagName, maxPort, port)
	}

	return nil
}

func serve(configDir string, port int, logger *slog.Logger) error {
	logger.Info("starting", "command", "serve", "version", version.String())
	return server.ListenAndServe(logger, workspace.Dir(configDir).TilesDir(), port)
}
