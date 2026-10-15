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

	newTilesSet := map[int]map[int]bool{}

	for _, tile := range tiles {
		latDeg, lonDeg := slippy.Num2Deg(float64(tile.X), float64(tile.Y), targetZoom+1)
		newX, newY := slippy.Deg2Num(latDeg, lonDeg, targetZoom)

		parentX := int(newX)
		parentY := int(newY)

		currentTilePath := TilePath(tilePath, targetZoom+1, tile.X, tile.Y)
		parentXDir := TileDirPath(tilePath, targetZoom, parentX)
		resizedTilePath := TilePath(tilePath, targetZoom, parentX, parentY)

		boundsX := int((newX - float64(parentX)) * 512)
		boundsY := int((newY - float64(parentY)) * 512)

		if newTilesSet[parentX] == nil {
			newTilesSet[parentX] = map[int]bool{}
		}
		newTilesSet[parentX][parentY] = true

		childImg, err := imaging.LoadNRGBA(currentTilePath)
		if err != nil {
			return fmt.Errorf("Resize: loading child tile %s: %w", currentTilePath, err)
		}

		if err := os.MkdirAll(parentXDir, 0o755); err != nil {
			return err
		}

		var parentImg *image.NRGBA
		if !imaging.FileExists(resizedTilePath) {
			parentImg = image.NewNRGBA(image.Rect(0, 0, 512, 512))
		} else {
			parentImg, err = imaging.LoadNRGBA(resizedTilePath)
			if err != nil {
				return fmt.Errorf("Resize: loading existing parent tile %s: %w", resizedTilePath, err)
			}
		}

		imaging.Blit(parentImg, childImg, boundsX, boundsY)

		if err := imaging.SavePNG(resizedTilePath, parentImg); err != nil {
			return fmt.Errorf("Resize: saving parent tile %s: %w", resizedTilePath, err)
		}
	}

	for parentX, ys := range newTilesSet {
		for parentY := range ys {
			path := TilePath(tilePath, targetZoom, parentX, parentY)
			img, err := imaging.LoadNRGBA(path)
			if err != nil {
				return fmt.Errorf("Resize: loading assembled tile %s: %w", path, err)
			}
			resized := imaging.Scale(img, imaging.TileSize, imaging.TileSize)
			if err := imaging.SavePNG(path, resized); err != nil {
				return fmt.Errorf("Resize: saving downsampled tile %s: %w", path, err)
			}
		}
	}

	return nil
}
