// Package tilestore manages the on-disk {z}/{x}/{y}.png tile pyramid layout:
// path building, directory walking, and building lower zoom levels from
// higher ones.
package tilestore

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Coord is a tile's (x, y) position within a single zoom level.
type Coord struct {
	X, Y int
}

// TilePath returns the on-disk path of the tile at (z, x, y) under base.
func TilePath(base string, z, x, y int) string {
	return filepath.Join(base, strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".png")
}

// TileDirPath returns the on-disk path of the x-directory for zoom z under base.
func TileDirPath(base string, z, x int) string {
	return filepath.Join(base, strconv.Itoa(z), strconv.Itoa(x))
}

// Walk lists every tile coordinate present under a single zoom-level
// directory laid out as {x}/{y}.png.
func Walk(dir string) ([]Coord, error) {
	xDirs, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var tiles []Coord

	for _, xDir := range xDirs {
		if !xDir.IsDir() {
			continue
		}

		xVal, err := strconv.Atoi(xDir.Name())
		if err != nil {
			continue
		}

		yEntries, err := os.ReadDir(filepath.Join(dir, xDir.Name()))
		if err != nil {
			return nil, err
		}

		for _, yEntry := range yEntries {
			name := yEntry.Name()
			if !strings.HasSuffix(name, ".png") {
				continue
			}

			yVal, err := strconv.Atoi(strings.TrimSuffix(name, ".png"))
			if err != nil {
				continue
			}

			tiles = append(tiles, Coord{X: xVal, Y: yVal})
		}
	}

	return tiles, nil
}
