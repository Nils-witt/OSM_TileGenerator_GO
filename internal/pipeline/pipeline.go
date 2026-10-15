// Package pipeline runs the complete tile generation flow for a project
// directory: warp the overlay image into tiles at the maximum zoom, build each
// lower zoom level from the one above it, then zip the resulting pyramid.
package pipeline

import (
	"fmt"
	"log/slog"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/config"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/generator"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/imaging"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/tilestore"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/workspace"
)

// markerCount is the number of image/map coordinate pairs the homography needs.
const markerCount = 4

// Options configures a pipeline run.
type Options struct {
	// Dir is the project directory holding config.json and image.png. Tiles,
	// intermediate images and the zip archive are written beneath it.
	Dir workspace.Dir
	// MinZoom is the lowest zoom level produced.
	MinZoom int
	// MaxZoom is the zoom level the overlay is warped at; every level from
	// MaxZoom-1 down to MinZoom is derived from it.
	MaxZoom int
	// Logger receives progress and diagnostic output; nil uses slog.Default().
	Logger *slog.Logger
}

// Run generates the tile pyramid and tiles.zip for opts.Dir. A config with
// fewer than four coordinate pairs is logged as a warning and skipped without
// error.
func Run(opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("loading project", "dir", string(opts.Dir))

	cfg, err := config.LoadConfig(string(opts.Dir))
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Debug("loaded config",
		"id", cfg.ID,
		"name", cfg.Name,
		"img_scale_width", cfg.ImgScale.Width,
		"img_scale_height", cfg.ImgScale.Height,
		"coordinates", len(cfg.Coordinates))

	if err := opts.Dir.Create(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	imgWidth, imgHeight, err := imaging.Dimensions(opts.Dir.ImagePath())
	if err != nil {
		return fmt.Errorf("reading image.png dimensions: %w", err)
	}

	if len(cfg.Coordinates) < markerCount {
		logger.Warn("not enough coordinates, skipping tile generation",
			"have", len(cfg.Coordinates),
			"need", markerCount)

		return nil
	}

	markers := scaleMarkers(cfg.Coordinates, cfg.ImgScale, imgWidth, imgHeight)

	gen := generator.NewGenerateTiles(
		opts.Dir.TilesDir(),
		opts.Dir.ImagePath(),
		markers,
		opts.MaxZoom,
		opts.Dir.TmpDir(),
	)

	gen.Logger = logger
	if err := gen.Run(); err != nil {
		return fmt.Errorf("generating tiles: %w", err)
	}

	for z := opts.MaxZoom - 1; z >= opts.MinZoom; z-- {
		logger.Info("building zoom level", "zoom", z)

		if err := tilestore.Resize(opts.Dir.TilesDir(), z); err != nil {
			return fmt.Errorf("resizing zoom %d: %w", z, err)
		}
	}

	logger.Info("creating zip archive", "path", opts.Dir.ZipPath())

	if err := tilestore.Zip(opts.Dir.TilesDir(), opts.Dir.ZipPath()); err != nil {
		return fmt.Errorf("zipping tiles: %w", err)
	}

	logger.Info("tile generation complete",
		"tiles_dir", opts.Dir.TilesDir(),
		"zip", opts.Dir.ZipPath())

	return nil
}

// scaleMarkers converts the first four coordinate pairs from the displayed
// image size the markers were placed on (scale) to the source image's real
// pixel size. coords must hold at least markerCount entries.
func scaleMarkers(
	coords []config.CoordinatePair,
	scale config.ImgScale,
	imgWidth, imgHeight int,
) [markerCount]generator.Marker {
	scaleWidth := float64(imgWidth) / scale.Width
	scaleHeight := float64(imgHeight) / scale.Height

	var markers [markerCount]generator.Marker
	for i := range markers {
		c := coords[i]
		markers[i] = generator.Marker{
			OverlayX: c.Img.X * scaleWidth,
			OverlayY: c.Img.Y * scaleHeight,
			MapLat:   c.Map.Latitude,
			MapLng:   c.Map.Longitude,
		}
	}

	return markers
}
