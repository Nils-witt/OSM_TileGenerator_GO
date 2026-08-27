package main

import (
	"flag"
	"fmt"
	_ "image/png"
	"log"
	"net/http"
	"os"

	"OSM_TileGenerator_GO/internal/config"
	"OSM_TileGenerator_GO/internal/generator"
	"OSM_TileGenerator_GO/internal/imaging"
	"OSM_TileGenerator_GO/internal/tilestore"
	"OSM_TileGenerator_GO/internal/version"
)

func createDirectories(configDir string) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(configDir+"/tiles", 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(configDir+"/tmp", 0o755); err != nil {
		return err
	}
	return nil
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

func tilesHandler(configDir string) http.Handler {
	return tileResponseHeaders(http.FileServer(http.Dir(configDir + "/tiles")))
}

func serveTiles(configDir string, port int) error {
	tilesDir := configDir + "/tiles"
	if info, err := os.Stat(tilesDir); err != nil || !info.IsDir() {
		return fmt.Errorf("tiles directory %q not found (generate tiles first): %w", tilesDir, err)
	}

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Serving %s on http://localhost%s\n", tilesDir, addr)
	return http.ListenAndServe(addr, tilesHandler(configDir))
}

func run() error {
	fmt.Println(version.String())

	configDir := flag.String("config-dir", "config", "Directory containing configuration files")
	maxZoom := flag.Int("max-zoom", 18, "Maximum zoom level")
	minZoom := flag.Int("min-zoom", 14, "Minimum zoom level")
	servePort := flag.Int("serve", 0, "Port to serve <config-dir>/tiles over HTTP instead of generating tiles (e.g. --serve 8080)")
	flag.Parse()

	if *servePort != 0 {
		return serveTiles(*configDir, *servePort)
	}

	fmt.Printf("Using config directory: %s\n", *configDir)

	cfg, err := config.LoadConfig(*configDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	fmt.Printf("Config loaded: %+v\n", cfg)

	if err := createDirectories(*configDir); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	imgWidth, imgHeight, err := imaging.Dimensions(*configDir + "/image.png")
	if err != nil {
		return fmt.Errorf("reading image.png dimensions: %w", err)
	}

	fmt.Println(cfg.Coordinates)
	if len(cfg.Coordinates) < 4 {
		fmt.Println("Not enough coordinates")
		return nil
	}

	scaleWidth := float64(imgWidth) / cfg.ImgScale.Width
	scaleHeight := float64(imgHeight) / cfg.ImgScale.Height

	var markersScaled [4]generator.Marker
	for i := 0; i < 4; i++ {
		c := cfg.Coordinates[i]
		markersScaled[i] = generator.Marker{
			OverlayX: c.Img.X * scaleWidth,
			OverlayY: c.Img.Y * scaleHeight,
			MapLat:   c.Map.Latitude,
			MapLng:   c.Map.Longitude,
		}
	}

	gen := generator.NewGenerateTiles(
		*configDir+"/tiles",
		*configDir+"/image.png",
		markersScaled,
		*maxZoom,
		*configDir+"/tmp",
	)
	if err := gen.Run(); err != nil {
		return fmt.Errorf("generating tiles: %w", err)
	}

	for z := *maxZoom - 1; z >= *minZoom; z-- {
		fmt.Printf("Resizing to %d\n", z)
		if err := tilestore.Resize(*configDir+"/tiles", z); err != nil {
			return fmt.Errorf("resizing zoom %d: %w", z, err)
		}
	}

	zipPath := *configDir + "/tiles.zip"
	fmt.Printf("Creating zip archive: %s\n", zipPath)
	if err := tilestore.Zip(*configDir+"/tiles", zipPath); err != nil {
		return fmt.Errorf("zipping tiles: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
