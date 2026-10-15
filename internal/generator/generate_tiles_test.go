package generator

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/geometry"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/imaging"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/slippy"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/tilestore"
)

func testMarkers() [4]Marker {
	return [4]Marker{
		{OverlayX: 10, OverlayY: 20, MapLat: 51.50, MapLng: -0.14},
		{OverlayX: 90, OverlayY: 20, MapLat: 51.51, MapLng: -0.10},
		{OverlayX: 90, OverlayY: 80, MapLat: 51.48, MapLng: -0.10},
		{OverlayX: 10, OverlayY: 80, MapLat: 51.48, MapLng: -0.14},
	}
}

func solidNRGBA(w, h int, r, g, b, a uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, b, a
	}

	return img
}

func TestGenerateTiles_createReferencePoints(t *testing.T) {
	t.Parallel()

	markers := testMarkers()
	g := &GenerateTiles{Markers: markers}
	g.createReferencePoints()

	for i, m := range markers {
		wantCoord := [2]float64{m.MapLat, m.MapLng}
		wantPoint := [2]float64{m.OverlayX, m.OverlayY}

		if g.coordinates[i] != wantCoord {
			t.Errorf("coordinates[%d] = %v, want %v", i, g.coordinates[i], wantCoord)
		}

		if g.picturePoints[i] != wantPoint {
			t.Errorf("picturePoints[%d] = %v, want %v", i, g.picturePoints[i], wantPoint)
		}
	}
}

func TestGenerateTiles_prepareCoordinatesForWarp(t *testing.T) {
	t.Parallel()

	const zoom = 15

	markers := testMarkers()
	g := &GenerateTiles{Markers: markers, Zoom: zoom}
	g.createReferencePoints()
	g.prepareCoordinatesForWarp()

	var osmTiles [4][2]float64

	xTiles := make([]int, 4)
	yTiles := make([]int, 4)

	for i, m := range markers {
		x, y := slippy.Deg2Num(m.MapLat, m.MapLng, zoom)
		osmTiles[i] = [2]float64{x, y}
		xTiles[i] = int(x)
		yTiles[i] = int(y)
	}

	sort.Ints(xTiles)
	sort.Ints(yTiles)

	wantXMin := xTiles[0] - osmTileOffset
	wantXMax := xTiles[3] + osmTileOffset
	wantYMin := yTiles[0] - osmTileOffset
	wantYMax := yTiles[3] + osmTileOffset

	if g.xMin != wantXMin || g.xMax != wantXMax || g.yMin != wantYMin || g.yMax != wantYMax {
		t.Errorf("bounds = (xMin=%d, xMax=%d, yMin=%d, yMax=%d), want (xMin=%d, xMax=%d, yMin=%d, yMax=%d)",
			g.xMin, g.xMax, g.yMin, g.yMax, wantXMin, wantXMax, wantYMin, wantYMax)
	}

	for i, tile := range osmTiles {
		want := [2]int{
			int((tile[0] - float64(wantXMin)) * imaging.TileSize),
			int((tile[1] - float64(wantYMin)) * imaging.TileSize),
		}
		if g.mapImgOffsets[i] != want {
			t.Errorf("mapImgOffsets[%d] = %v, want %v", i, g.mapImgOffsets[i], want)
		}
	}
}

func TestGenerateTiles_calculateTargetSize(t *testing.T) {
	t.Parallel()

	g := &GenerateTiles{Markers: testMarkers(), Zoom: 15}
	g.createReferencePoints()
	g.prepareCoordinatesForWarp()

	wantW := (g.xMax - g.xMin) * imaging.TileSize
	wantH := (g.yMax - g.yMin) * imaging.TileSize

	gotW, gotH := g.calculateTargetSize()
	if gotW != wantW || gotH != wantH {
		t.Errorf("calculateTargetSize() = (%d, %d), want (%d, %d)", gotW, gotH, wantW, wantH)
	}
}

func TestGenerateTiles_homography(t *testing.T) {
	t.Parallel()

	g := squareGenerator(t.TempDir(), 1, 50, 200)
	h := g.homography()

	for i, p := range g.picturePoints {
		x, y := p[0], p[1]
		denom := h[2][0]*x + h[2][1]*y + h[2][2]
		u := (h[0][0]*x + h[0][1]*y + h[0][2]) / denom
		v := (h[1][0]*x + h[1][1]*y + h[1][2]) / denom

		want := g.mapImgOffsets[i]
		if math.Abs(u-float64(want[0])) > 1e-9 || math.Abs(v-float64(want[1])) > 1e-9 {
			t.Errorf("homography maps picture point %d %v to (%g, %g), want %v", i, p, u, v, want)
		}
	}
}

// squareGenerator returns a generator on a tiles x tiles canvas whose 4x4
// source image maps onto the canvas square spanning [from, to] on both axes.
func squareGenerator(tilesPath string, tiles, from, to int) *GenerateTiles {
	g := &GenerateTiles{TilesPath: tilesPath, Zoom: 9, xMin: 100, xMax: 100 + tiles, yMin: 200, yMax: 200 + tiles}
	g.picturePoints = [4][2]float64{{0, 0}, {3, 0}, {3, 3}, {0, 3}}
	g.mapImgOffsets = [4][2]int{{from, from}, {to, from}, {to, to}, {from, to}}

	return g
}

func squareWarper(g *GenerateTiles) *geometry.Warper {
	return geometry.NewWarper(solidNRGBA(4, 4, 200, 50, 50, 255), g.homography())
}

func TestGenerateTiles_tileRange(t *testing.T) {
	t.Parallel()

	ts := imaging.TileSize

	tests := []struct {
		name     string
		tiles    int
		from, to int
		want     image.Rectangle
	}{
		{name: "overlay inside one tile", tiles: 4, from: ts + 40, to: ts + 200, want: image.Rect(1, 1, 2, 2)},
		{name: "overlay straddling tiles", tiles: 4, from: ts - 40, to: 2*ts + 40, want: image.Rect(0, 0, 3, 3)},
		{name: "overlay clipped to the canvas", tiles: 2, from: ts + 40, to: 5 * ts, want: image.Rect(1, 1, 2, 2)},
		{name: "overlay outside the canvas", tiles: 2, from: 3 * ts, to: 4 * ts, want: image.Rectangle{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := squareGenerator(t.TempDir(), tt.tiles, tt.from, tt.to)
			if got := g.tileRange(squareWarper(g)); got != tt.want {
				t.Errorf("tileRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateTiles_renderTiles(t *testing.T) {
	t.Parallel()

	t.Run("writes only non-empty tiles at their global coordinates", func(t *testing.T) {
		t.Parallel()

		ts := imaging.TileSize
		tilesPath := t.TempDir()
		// 2x2 canvas; the overlay lies entirely in the bottom-right tile.
		g := squareGenerator(tilesPath, 2, ts+40, ts+200)

		n, err := g.renderTiles(squareWarper(g))
		if err != nil {
			t.Fatalf("renderTiles() error = %v", err)
		}

		if n != 1 {
			t.Errorf("renderTiles() wrote %d tiles, want 1", n)
		}

		if p := tilestore.TilePath(tilesPath, 9, 101, 201); !imaging.FileExists(p) {
			t.Errorf("expected tile at %s to exist", p)
		}

		// Column 100 holds only transparent tiles, so it must not be created.
		if p := tilestore.TileDirPath(tilesPath, 9, 100); imaging.FileExists(p) {
			t.Errorf("expected no column directory at %s", p)
		}
	})

	t.Run("reports a failure to save", func(t *testing.T) {
		t.Parallel()

		// A regular file where the tiles directory should be makes every
		// MkdirAll beneath it fail.
		tilesPath := filepath.Join(t.TempDir(), "tiles")
		if err := os.WriteFile(tilesPath, nil, 0o644); err != nil {
			t.Fatalf("writing blocking file: %v", err)
		}

		g := squareGenerator(tilesPath, 2, 40, 2*imaging.TileSize-40)

		if _, err := g.renderTiles(squareWarper(g)); err == nil {
			t.Error("renderTiles() error = nil, want error when tiles cannot be saved")
		}
	})
}

func TestGenerateTiles_Run(t *testing.T) {
	t.Parallel()

	newSource := func(t *testing.T, size int) string {
		t.Helper()

		path := filepath.Join(t.TempDir(), "overlay.png")
		if err := imaging.SavePNG(path, solidNRGBA(size, size, 10, 200, 30, 255)); err != nil {
			t.Fatalf("saving overlay image: %v", err)
		}

		return path
	}

	t.Run("writes the warped overlay's tiles", func(t *testing.T) {
		t.Parallel()

		tilesPath := t.TempDir()
		if err := NewGenerateTiles(tilesPath, newSource(t, 100), testMarkers(), 15).Run(); err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		tiles, err := tilestore.Walk(filepath.Join(tilesPath, "15"))
		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		if len(tiles) == 0 {
			t.Error("Run() wrote no tiles, want the overlay's footprint covered")
		}
	})

	t.Run("creates the zoom directory even when every tile is empty", func(t *testing.T) {
		t.Parallel()

		// A 1x1 source has no 2x2 neighborhood to sample, so nothing renders.
		tilesPath := t.TempDir()
		if err := NewGenerateTiles(tilesPath, newSource(t, 1), testMarkers(), 15).Run(); err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		tiles, err := tilestore.Walk(filepath.Join(tilesPath, "15"))
		if err != nil {
			t.Fatalf("Walk() error = %v, want an empty but existing zoom directory", err)
		}

		if len(tiles) != 0 {
			t.Errorf("Run() wrote %d tiles, want 0", len(tiles))
		}
	})

	t.Run("fails when the overlay image is missing", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.png")
		if err := NewGenerateTiles(t.TempDir(), missing, testMarkers(), 15).Run(); err == nil {
			t.Error("Run() error = nil, want error for a missing overlay image")
		}
	})
}
