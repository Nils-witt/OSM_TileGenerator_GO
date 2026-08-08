package main

import (
	"flag"
	"fmt"
	_ "image/png"
	"log"
	"os"

	"OSM_TileGenerator_GO/internal/config"
	"OSM_TileGenerator_GO/internal/generator"
	"OSM_TileGenerator_GO/internal/imaging"
	"OSM_TileGenerator_GO/internal/tilestore"
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

func run() error {
	configDir := flag.String("config-dir", "config", "Directory containing configuration files")
	maxZoom := flag.Int("max-zoom", 18, "Maximum zoom level")
	minZoom := flag.Int("min-zoom", 14, "Minimum zoom level")
	flag.Parse()

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

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
