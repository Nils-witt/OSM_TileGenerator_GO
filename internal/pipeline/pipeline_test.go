package pipeline

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/config"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/generator"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/imaging"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/workspace"
)

// discardLogger keeps the pipeline's progress output out of test logs.
var discardLogger = slog.New(slog.DiscardHandler)

// testCoordinates places four markers on the corners of a 100x100 display
// image covering roughly 700m x 1.1km in London. At zoom 16 that warps to
// several tiles in each direction; an overlay narrower than one tile can fall
// between the columns imaging.FindXBounds samples and produce no tiles.
func testCoordinates() []config.CoordinatePair {
	return []config.CoordinatePair{
		{Img: config.ImgCoordinate{X: 0, Y: 0}, Map: config.MapCoordinate{Latitude: 51.510, Longitude: -0.130}},
		{Img: config.ImgCoordinate{X: 100, Y: 0}, Map: config.MapCoordinate{Latitude: 51.510, Longitude: -0.120}},
		{Img: config.ImgCoordinate{X: 100, Y: 100}, Map: config.MapCoordinate{Latitude: 51.500, Longitude: -0.120}},
		{Img: config.ImgCoordinate{X: 0, Y: 100}, Map: config.MapCoordinate{Latitude: 51.500, Longitude: -0.130}},
	}
}

// newProject writes config.json and a fully opaque image.png into a fresh
// project directory.
func newProject(t *testing.T, coords []config.CoordinatePair, imgSize int) workspace.Dir {
	t.Helper()

	dir := workspace.Dir(t.TempDir())
	cfg := config.Project{
		ImgScale:    config.ImgScale{Width: 100, Height: 100},
		Coordinates: coords,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(string(dir), "config.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, imgSize, imgSize))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	if err := imaging.SavePNG(dir.ImagePath(), img); err != nil {
		t.Fatalf("SavePNG() error = %v", err)
	}
	return dir
}

func countPNGs(t *testing.T, root string) int {
	t.Helper()

	n := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".png") {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return n
}

func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("generates every zoom level and a zip archive", func(t *testing.T) {
		t.Parallel()
		if testing.Short() {
			// The generator always warps into a canvas of at least 41x41 tiles
			// (~440MB of RGBA), which takes about a minute under -race.
			t.Skip("skipping full pipeline run in -short mode")
		}
		dir := newProject(t, testCoordinates(), 200)

		if err := Run(Options{Dir: dir, MinZoom: 14, MaxZoom: 16, Logger: discardLogger}); err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		total := 0
		for _, z := range []string{"14", "15", "16"} {
			n := countPNGs(t, filepath.Join(dir.TilesDir(), z))
			if n == 0 {
				t.Errorf("zoom %s has no tiles", z)
			}
			total += n
		}

		r, err := zip.OpenReader(dir.ZipPath())
		if err != nil {
			t.Fatalf("OpenReader() error = %v", err)
		}
		defer func() { _ = r.Close() }()
		if len(r.File) != total {
			t.Errorf("zip has %d entries, want %d (one per tile)", len(r.File), total)
		}
	})

	t.Run("skips generation when fewer than four coordinates", func(t *testing.T) {
		t.Parallel()
		dir := newProject(t, testCoordinates()[:3], 200)
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))

		if err := Run(Options{Dir: dir, MinZoom: 14, MaxZoom: 16, Logger: logger}); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if want := "level=WARN msg=\"not enough coordinates, skipping tile generation\" have=3 need=4"; !strings.Contains(logs.String(), want) {
			t.Errorf("logs = %q, want them to contain %q", logs.String(), want)
		}
		if _, err := os.Stat(dir.ZipPath()); !os.IsNotExist(err) {
			t.Errorf("Stat(tiles.zip) error = %v, want not-exist", err)
		}
	})

	t.Run("fails without config.json", func(t *testing.T) {
		t.Parallel()
		dir := workspace.Dir(t.TempDir())

		if err := Run(Options{Dir: dir, MinZoom: 14, MaxZoom: 16, Logger: discardLogger}); err == nil {
			t.Fatal("Run() error = nil, want error for missing config.json")
		}
	})
}

func TestScaleMarkers(t *testing.T) {
	t.Parallel()

	coords := testCoordinates()
	// Markers were placed on a 100x100 display of a 400x200 source image.
	got := scaleMarkers(coords, config.ImgScale{Width: 100, Height: 100}, 400, 200)

	want := [markerCount]generator.Marker{
		{OverlayX: 0, OverlayY: 0, MapLat: 51.510, MapLng: -0.130},
		{OverlayX: 400, OverlayY: 0, MapLat: 51.510, MapLng: -0.120},
		{OverlayX: 400, OverlayY: 200, MapLat: 51.500, MapLng: -0.120},
		{OverlayX: 0, OverlayY: 200, MapLat: 51.500, MapLng: -0.130},
	}
	if got != want {
		t.Errorf("scaleMarkers() = %+v, want %+v", got, want)
	}
}
