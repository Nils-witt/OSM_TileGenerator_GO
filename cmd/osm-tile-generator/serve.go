package main

import (
	"flag"
	"io"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/server"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/version"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/workspace"
)

const (
	defaultPort = 8080
	maxPort     = 65535
)

func runServe(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("serve", "Serve <config-dir>/tiles over HTTP with CORS enabled.", stderr)
	configDir := fs.String("config-dir", "config", "Directory whose tiles/ subdirectory is served")
	port := fs.Int("port", defaultPort, "TCP port to listen on")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := validatePort(fs, "-port", *port); err != nil {
		return err
	}

	return serve(*configDir, *port, stdout)
}

func validatePort(fs *flag.FlagSet, flagName string, port int) error {
	if port < 1 || port > maxPort {
		return usageErrorf(fs, "%s must be between 1 and %d, got %d", flagName, maxPort, port)
	}
	return nil
}

func serve(configDir string, port int, stdout io.Writer) error {
	writef(stdout, "%s\n", version.String())
	return server.ListenAndServe(workspace.Dir(configDir).TilesDir(), port)
}
