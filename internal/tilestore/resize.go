package tilestore

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/imaging"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/slippy"
)

// Resize builds zoom level targetZoom entirely from the tiles present at
// targetZoom+1. Mirrors tile_calc.py::resize. Must be called in descending
// zoom order since each level depends on its immediate parent already
// existing on disk.
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

	parents := map[Coord]struct{}{}

	for _, tile := range tiles {
		parent, err := mergeIntoParent(tilePath, targetZoom, tile)
		if err != nil {
			return err
		}

		parents[parent] = struct{}{}
	}

	for parent := range parents {
		path := TilePath(tilePath, targetZoom, parent.X, parent.Y)

		img, err := imaging.LoadNRGBA(path)
		if err != nil {
			return fmt.Errorf("Resize: loading assembled tile %s: %w", path, err)
		}

		resized := imaging.Scale(img, imaging.TileSize, imaging.TileSize)
		if err := imaging.SavePNG(path, resized); err != nil {
			return fmt.Errorf("Resize: saving downsampled tile %s: %w", path, err)
		}
	}

	return nil
}

// mergeIntoParent pastes the zoom targetZoom+1 tile child into its quadrant
// of the double-size (512x512) parent tile at targetZoom, creating the parent
// if needed. It returns the parent's coordinate; the parent is downsampled to
// TileSize once all of its children are merged.
func mergeIntoParent(tilePath string, targetZoom int, child Coord) (Coord, error) {
	latDeg, lonDeg := slippy.Num2Deg(float64(child.X), float64(child.Y), targetZoom+1)
	newX, newY := slippy.Deg2Num(latDeg, lonDeg, targetZoom)
	parent := Coord{X: int(newX), Y: int(newY)}

	childPath := TilePath(tilePath, targetZoom+1, child.X, child.Y)
	parentXDir := TileDirPath(tilePath, targetZoom, parent.X)
	parentPath := TilePath(tilePath, targetZoom, parent.X, parent.Y)

	boundsX := int((newX - float64(parent.X)) * 512)
	boundsY := int((newY - float64(parent.Y)) * 512)

	childImg, err := imaging.LoadNRGBA(childPath)
	if err != nil {
		return parent, fmt.Errorf("Resize: loading child tile %s: %w", childPath, err)
	}

	if err := os.MkdirAll(parentXDir, 0o755); err != nil {
		return parent, err
	}

	var parentImg *image.NRGBA
	if !imaging.FileExists(parentPath) {
		parentImg = image.NewNRGBA(image.Rect(0, 0, 512, 512))
	} else {
		parentImg, err = imaging.LoadNRGBA(parentPath)
		if err != nil {
			return parent, fmt.Errorf("Resize: loading existing parent tile %s: %w", parentPath, err)
		}
	}

	imaging.Blit(parentImg, childImg, boundsX, boundsY)

	if err := imaging.SavePNG(parentPath, parentImg); err != nil {
		return parent, fmt.Errorf("Resize: saving parent tile %s: %w", parentPath, err)
	}

	return parent, nil
}
