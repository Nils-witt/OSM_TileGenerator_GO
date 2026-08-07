package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"os"
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

func imageDimensions(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func run() error {
	configDir := flag.String("config-dir", "config", "Directory containing configuration files")
	maxZoom := flag.Int("max-zoom", 18, "Maximum zoom level")
	minZoom := flag.Int("min-zoom", 14, "Minimum zoom level")
	flag.Parse()

	fmt.Printf("Hello from Go Worker! Using config directory: %s\n", *configDir)

	cfg, err := loadConfig(*configDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	fmt.Printf("Config loaded: %+v\n", cfg)

	if err := createDirectories(*configDir); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	imgWidth, imgHeight, err := imageDimensions(*configDir + "/image.png")
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

	var markersScaled [4]Marker
	for i := 0; i < 4; i++ {
		c := cfg.Coordinates[i]
		markersScaled[i] = Marker{
			OverlayX: c.Img.X * scaleWidth,
			OverlayY: c.Img.Y * scaleHeight,
			MapLat:   c.Map.Latitude,
			MapLng:   c.Map.Longitude,
		}
	}

	generator := NewGenerateTiles(
		*configDir+"/tiles",
		*configDir+"/image.png",
		markersScaled,
		*maxZoom,
		*configDir+"/tmp",
	)
	if err := generator.Run(); err != nil {
		return fmt.Errorf("generating tiles: %w", err)
	}

	for z := *maxZoom - 1; z >= *minZoom; z-- {
		fmt.Printf("Resizing to %d\n", z)
		if err := Resize(*configDir+"/tiles", z); err != nil {
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
