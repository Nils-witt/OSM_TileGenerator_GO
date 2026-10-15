package tilestore

import (
	"image"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/imaging"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/slippy"
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

		const (
			childZoom      = 10
			parentZoom     = childZoom - 1
			childX, childY = 500, 300
		)

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

		const (
			childZoom  = 12
			parentZoom = childZoom - 1
		)

		// A 2x2 block of adjacent child tiles maps to a single parent tile.
		const baseX, baseY = 1000, 1000

		colors := [4][4]uint8{
			{255, 0, 0, 255},
			{0, 255, 0, 255},
			{0, 0, 255, 255},
			{255, 255, 0, 255},
		}
		i := 0

		for dx := range 2 {
			for dy := range 2 {
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

	t.Run("child lands in its own quadrant of the parent", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()

		// Odd x, even y: the top-right quadrant of parent (10, 20).
		const childZoom, childX, childY = 8, 21, 40

		if err := os.MkdirAll(TileDirPath(base, childZoom, childX), 0o755); err != nil {
			t.Fatalf("mkdir child dir: %v", err)
		}

		if err := imaging.SavePNG(TilePath(base, childZoom, childX, childY), solidTile(255, 0, 0, 255)); err != nil {
			t.Fatalf("saving child tile: %v", err)
		}

		if err := Resize(base, childZoom-1); err != nil {
			t.Fatalf("Resize() error = %v", err)
		}

		parentImg, err := imaging.LoadNRGBA(TilePath(base, childZoom-1, 10, 20))
		if err != nil {
			t.Fatalf("loading parent tile: %v", err)
		}

		half := imaging.TileSize / 2
		for _, tc := range []struct {
			name      string
			x, y      int
			wantAlpha uint8
		}{
			{name: "top-right", x: half + half/2, y: half / 2, wantAlpha: 255},
			{name: "top-left", x: half / 2, y: half / 2, wantAlpha: 0},
			{name: "bottom-right", x: half + half/2, y: half + half/2, wantAlpha: 0},
		} {
			if got := parentImg.NRGBAAt(tc.x, tc.y).A; got != tc.wantAlpha {
				t.Errorf("%s quadrant alpha = %d, want %d", tc.name, got, tc.wantAlpha)
			}
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

func TestGroupByParent(t *testing.T) {
	t.Parallel()

	got := groupByParent([]Coord{{X: 4, Y: 6}, {X: 5, Y: 7}, {X: 5, Y: 6}, {X: 7, Y: 0}})

	want := map[Coord][]Coord{
		{X: 2, Y: 3}: {{X: 4, Y: 6}, {X: 5, Y: 7}, {X: 5, Y: 6}},
		{X: 3, Y: 0}: {{X: 7, Y: 0}},
	}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("groupByParent() = %v, want %v", got, want)
	}
}
