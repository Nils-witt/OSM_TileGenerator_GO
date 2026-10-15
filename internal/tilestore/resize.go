package tilestore

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/imaging"
)

// Resize builds zoom level targetZoom entirely from the tiles present at
// targetZoom+1. Mirrors tile_calc.py::resize. Must be called in descending
// zoom order since each level depends on its immediate parent already
// existing on disk.
//
// Each parent tile is assembled from its (up to four) children in memory and
// encoded once, with parents built concurrently. The Python original instead
// re-read and re-wrote the half-built parent PNG once per child.
func Resize(tilePath string, targetZoom int) error {
	sourceDir := filepath.Join(tilePath, strconv.Itoa(targetZoom+1))

	tiles, err := Walk(sourceDir)
	if err != nil {
		return err
	}

	targetDir := filepath.Join(tilePath, strconv.Itoa(targetZoom))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	families := groupByParent(tiles)

	columns := map[int]struct{}{}
	for parent := range families {
		columns[parent.X] = struct{}{}
	}

	for x := range columns {
		if err := os.MkdirAll(TileDirPath(tilePath, targetZoom, x), 0o755); err != nil {
			return err
		}
	}

	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(runtime.GOMAXPROCS(0))

	for parent, children := range families {
		if ctx.Err() != nil {
			break
		}

		eg.Go(func() error {
			return buildParent(tilePath, targetZoom, parent, children)
		})
	}

	return eg.Wait()
}

// groupByParent buckets zoom z+1 tiles by the zoom z tile containing them.
// Each tile splits into exactly four children at the next zoom level, so the
// parent of (x, y) is (x/2, y/2); tile numbers are never negative.
func groupByParent(tiles []Coord) map[Coord][]Coord {
	families := make(map[Coord][]Coord, len(tiles)/2)
	for _, child := range tiles {
		parent := Coord{X: child.X / 2, Y: child.Y / 2}
		families[parent] = append(families[parent], child)
	}

	return families
}

// buildParent pastes each zoom targetZoom+1 child into its quadrant of a
// double-size (512x512) canvas, downsamples it to TileSize and saves it as
// the parent tile at targetZoom. Quadrants without a child stay transparent.
func buildParent(tilePath string, targetZoom int, parent Coord, children []Coord) error {
	canvas := image.NewNRGBA(image.Rect(0, 0, 2*imaging.TileSize, 2*imaging.TileSize))

	for _, child := range children {
		childPath := TilePath(tilePath, targetZoom+1, child.X, child.Y)

		childImg, err := imaging.LoadNRGBA(childPath)
		if err != nil {
			return fmt.Errorf("Resize: loading child tile %s: %w", childPath, err)
		}

		imaging.Blit(canvas, childImg, (child.X%2)*imaging.TileSize, (child.Y%2)*imaging.TileSize)
	}

	path := TilePath(tilePath, targetZoom, parent.X, parent.Y)

	resized := imaging.Scale(canvas, imaging.TileSize, imaging.TileSize)
	if err := imaging.SavePNG(path, resized); err != nil {
		return fmt.Errorf("Resize: saving downsampled tile %s: %w", path, err)
	}

	return nil
}
