package tilestore

import (
	"image"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"OSM_TileGenerator_GO/internal/imaging"
	"OSM_TileGenerator_GO/internal/slippy"
)

func solidTile(r, g, b, a uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, imaging.TileSize, imaging.TileSize))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, b, a
	}
	return img
}

func TestResize(t *testing.T) {
	t.Parallel()

	t.Run("builds parent tile from a single opaque child", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		const childZoom = 10
		const parentZoom = childZoom - 1
		const childX, childY = 500, 300

		childDir := TileDirPath(base, childZoom, childX)
		if err := os.MkdirAll(childDir, 0o755); err != nil {
			t.Fatalf("mkdir child dir: %v", err)
		}
		child := solidTile(255, 0, 0, 255)
		if err := imaging.SavePNG(TilePath(base, childZoom, childX, childY), child); err != nil {
			t.Fatalf("saving child tile: %v", err)
		}

		if err := Resize(base, parentZoom); err != nil {
			t.Fatalf("Resize() error = %v", err)
		}

		// Reproduce the expected parent coordinate the same way Resize does,
		// using slippy math directly (the package under test's own dependency,
		// not a reimplementation of Resize's assembly logic).
		lat, lon := slippy.Num2Deg(childX, childY, childZoom)
		px, py := slippy.Deg2Num(lat, lon, parentZoom)
		parentX, parentY := int(px), int(py)

		parentPath := TilePath(base, parentZoom, parentX, parentY)
		if !imaging.FileExists(parentPath) {
			t.Fatalf("Resize() did not create expected parent tile at %s", parentPath)
		}

		parentImg, err := imaging.LoadNRGBA(parentPath)
		if err != nil {
			t.Fatalf("loading parent tile: %v", err)
		}
		if parentImg.Bounds().Dx() != imaging.TileSize || parentImg.Bounds().Dy() != imaging.TileSize {
			t.Errorf("parent tile size = %v, want %dx%d", parentImg.Bounds(), imaging.TileSize, imaging.TileSize)
		}
		if imaging.Empty(parentImg) {
			t.Error("parent tile is fully empty, want content blitted in from the opaque child")
		}
	})

	t.Run("four children sharing a parent all land in one output tile", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		const childZoom = 12
		const parentZoom = childZoom - 1

		// A 2x2 block of adjacent child tiles maps to a single parent tile.
		const baseX, baseY = 1000, 1000
		colors := [4][4]uint8{
			{255, 0, 0, 255},
			{0, 255, 0, 255},
			{0, 0, 255, 255},
			{255, 255, 0, 255},
		}
		i := 0
		for dx := 0; dx < 2; dx++ {
			for dy := 0; dy < 2; dy++ {
				x, y := baseX+dx, baseY+dy
				dir := TileDirPath(base, childZoom, x)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("mkdir child dir: %v", err)
				}
				c := colors[i]
				if err := imaging.SavePNG(TilePath(base, childZoom, x, y), solidTile(c[0], c[1], c[2], c[3])); err != nil {
					t.Fatalf("saving child tile %d/%d: %v", x, y, err)
				}
				i++
			}
		}

		if err := Resize(base, parentZoom); err != nil {
			t.Fatalf("Resize() error = %v", err)
		}

		tiles, err := Walk(filepath.Join(base, strconv.Itoa(parentZoom)))
		if err != nil {
			t.Fatalf("Walk() parent zoom dir: %v", err)
		}
		if len(tiles) != 1 {
			t.Fatalf("Walk() parent tiles = %v, want exactly 1 merged tile", tiles)
		}

		parentImg, err := imaging.LoadNRGBA(TilePath(base, parentZoom, tiles[0].X, tiles[0].Y))
		if err != nil {
			t.Fatalf("loading merged parent tile: %v", err)
		}
		if imaging.Empty(parentImg) {
			t.Error("merged parent tile is fully empty, want content from 4 opaque children")
		}
	})

	t.Run("missing source zoom directory returns error", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		if err := Resize(base, 5); err == nil {
			t.Fatal("Resize() error = nil, want error when source zoom dir does not exist")
		}
	})
}
