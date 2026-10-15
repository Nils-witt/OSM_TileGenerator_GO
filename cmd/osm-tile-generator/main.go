package main

import (
	"flag"
	"fmt"
	_ "image/png"
	"log"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/pipeline"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/server"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/version"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/workspace"
)

func run() error {
	fmt.Println(version.String())

	configDir := flag.String("config-dir", "config", "Directory containing configuration files")
	maxZoom := flag.Int("max-zoom", 18, "Maximum zoom level")
	minZoom := flag.Int("min-zoom", 14, "Minimum zoom level")
	servePort := flag.Int("serve", 0, "Port to serve <config-dir>/tiles over HTTP instead of generating tiles (e.g. --serve 8080)")
	flag.Parse()

	dir := workspace.Dir(*configDir)
	if *servePort != 0 {
		return server.ListenAndServe(dir.TilesDir(), *servePort)
	}

	return pipeline.Run(pipeline.Options{
		Dir:     dir,
		MinZoom: *minZoom,
		MaxZoom: *maxZoom,
	})
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
