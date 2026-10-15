package generator

import (
	"image"
	"os"
	"path/filepath"
	"sort"
	"testing"

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

func TestGenerateTiles_overlayPaths(t *testing.T) {
	t.Parallel()

	g := &GenerateTiles{TmpDir: "/tmp/proj", Zoom: 15}

	if got, want := g.overlayPath(), filepath.FromSlash("/tmp/proj/overlay_15.png"); got != want {
		t.Errorf("overlayPath() = %q, want %q", got, want)
	}
	if got, want := g.overlayCropPath(), filepath.FromSlash("/tmp/proj/overlay_15_crop.png"); got != want {
		t.Errorf("overlayCropPath() = %q, want %q", got, want)
	}
}

func TestGenerateTiles_warpImage(t *testing.T) {
	t.Parallel()

	t.Run("skips work when output already exists", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		g := &GenerateTiles{TmpDir: tmp, Zoom: 3}

		sentinel := []byte("not a real png, but warpImage should never touch it")
		if err := os.WriteFile(g.overlayPath(), sentinel, 0o644); err != nil {
			t.Fatalf("writing sentinel file: %v", err)
		}

		if err := g.warpImage(); err != nil {
			t.Fatalf("warpImage() error = %v", err)
		}

		got, err := os.ReadFile(g.overlayPath())
		if err != nil {
			t.Fatalf("reading overlay file: %v", err)
		}
		if string(got) != string(sentinel) {
			t.Error("warpImage() overwrote an existing output file instead of skipping")
		}
	})

	t.Run("warps source image into the target canvas", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		srcPath := filepath.Join(tmp, "overlay-src.png")
		if err := imaging.SavePNG(srcPath, solidNRGBA(4, 4, 200, 50, 50, 255)); err != nil {
			t.Fatalf("saving source image: %v", err)
		}

		g := &GenerateTiles{TmpDir: tmp, Zoom: 3, OverlayImgPath: srcPath}
		g.picturePoints = [4][2]float64{{0, 0}, {3, 0}, {3, 3}, {0, 3}}
		g.mapImgOffsets = [4][2]int{{50, 50}, {200, 50}, {200, 200}, {50, 200}}
		g.xMin, g.xMax = 0, 1 // 1 tile wide -> 256px target canvas
		g.yMin, g.yMax = 0, 1

		if err := g.warpImage(); err != nil {
			t.Fatalf("warpImage() error = %v", err)
		}

		out, err := imaging.LoadNRGBA(g.overlayPath())
		if err != nil {
			t.Fatalf("loading warped output: %v", err)
		}
		if out.Bounds().Dx() != imaging.TileSize || out.Bounds().Dy() != imaging.TileSize {
			t.Errorf("warped canvas size = %v, want %dx%d", out.Bounds(), imaging.TileSize, imaging.TileSize)
		}
		if imaging.Empty(out) {
			t.Error("warped canvas is fully empty, want the source square painted into it")
		}
	})
}

func TestGenerateTiles_cropImage(t *testing.T) {
	t.Parallel()

	t.Run("skips work when output already exists", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		g := &GenerateTiles{TmpDir: tmp, Zoom: 3}

		sentinel := []byte("not a real png, but cropImage should never touch it")
		if err := os.WriteFile(g.overlayCropPath(), sentinel, 0o644); err != nil {
			t.Fatalf("writing sentinel file: %v", err)
		}

		if err := g.cropImage(); err != nil {
			t.Fatalf("cropImage() error = %v", err)
		}

		got, err := os.ReadFile(g.overlayCropPath())
		if err != nil {
			t.Fatalf("reading crop output file: %v", err)
		}
		if string(got) != string(sentinel) {
			t.Error("cropImage() overwrote an existing output file instead of skipping")
		}
	})

	t.Run("crops to the content bounds reported by imaging.Find{X,Y}Bounds", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		g := &GenerateTiles{TmpDir: tmp, Zoom: 3}

		warped := image.NewNRGBA(image.Rect(0, 0, imaging.TileSize*2, imaging.TileSize*2))
		for y := 50; y < 300; y++ {
			for x := 50; x < 300; x++ {
				off := warped.PixOffset(x, y)
				warped.Pix[off], warped.Pix[off+1], warped.Pix[off+2], warped.Pix[off+3] = 10, 20, 30, 255
			}
		}
		if err := imaging.SavePNG(g.overlayPath(), warped); err != nil {
			t.Fatalf("saving warped fixture: %v", err)
		}

		wantW := imaging.FindXBounds(warped)
		wantH := imaging.FindYBounds(warped)

		if err := g.cropImage(); err != nil {
			t.Fatalf("cropImage() error = %v", err)
		}

		cropped, err := imaging.LoadNRGBA(g.overlayCropPath())
		if err != nil {
			t.Fatalf("loading cropped output: %v", err)
		}
		if cropped.Bounds().Dx() != wantW || cropped.Bounds().Dy() != wantH {
			t.Errorf("cropped size = %v, want %dx%d", cropped.Bounds(), wantW, wantH)
		}
	})
}

func TestGenerateTiles_tileWorker(t *testing.T) {
	t.Parallel()

	t.Run("saves a non-empty tile", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		g := &GenerateTiles{}
		img := solidNRGBA(imaging.TileSize, imaging.TileSize, 5, 5, 5, 255)
		savePath := filepath.Join(tmp, "tile.png")

		if err := g.tileWorker(img, 0, 0, savePath); err != nil {
			t.Fatalf("tileWorker() error = %v", err)
		}
		if !imaging.FileExists(savePath) {
			t.Error("tileWorker() did not save a file for a non-empty tile")
		}
	})

	t.Run("skips saving a fully transparent tile", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		g := &GenerateTiles{}
		img := image.NewNRGBA(image.Rect(0, 0, imaging.TileSize, imaging.TileSize))
		savePath := filepath.Join(tmp, "tile.png")

		if err := g.tileWorker(img, 0, 0, savePath); err != nil {
			t.Fatalf("tileWorker() error = %v", err)
		}
		if imaging.FileExists(savePath) {
			t.Error("tileWorker() saved a file for a fully transparent tile")
		}
	})
}

func TestGenerateTiles_tileXWorker(t *testing.T) {
	t.Parallel()

	t.Run("writes only non-empty tiles at their global coordinates", func(t *testing.T) {
		t.Parallel()
		tilesPath := t.TempDir()
		g := &GenerateTiles{TilesPath: tilesPath, Zoom: 7, xMin: 10, yMin: 20}

		// One column, two vertically-stacked tiles: row 0 has content, row 1 is empty.
		img := image.NewNRGBA(image.Rect(0, 0, imaging.TileSize, imaging.TileSize*2))
		for y := 0; y < imaging.TileSize; y++ {
			for x := 0; x < imaging.TileSize; x++ {
				off := img.PixOffset(x, y)
				img.Pix[off], img.Pix[off+3] = 200, 255
			}
		}

		if err := g.tileXWorker(img, 0); err != nil {
			t.Fatalf("tileXWorker() error = %v", err)
		}

		nonEmptyPath := tilestore.TilePath(tilesPath, 7, 10, 20)
		if !imaging.FileExists(nonEmptyPath) {
			t.Errorf("expected tile at %s to exist", nonEmptyPath)
		}
		emptyPath := tilestore.TilePath(tilesPath, 7, 10, 21)
		if imaging.FileExists(emptyPath) {
			t.Errorf("expected no tile at %s (fully transparent)", emptyPath)
		}
	})

	t.Run("removes the x directory when every tile in the column is empty", func(t *testing.T) {
		t.Parallel()
		tilesPath := t.TempDir()
		g := &GenerateTiles{TilesPath: tilesPath, Zoom: 7, xMin: 0, yMin: 0}

		img := image.NewNRGBA(image.Rect(0, 0, imaging.TileSize, imaging.TileSize))

		if err := g.tileXWorker(img, 0); err != nil {
			t.Fatalf("tileXWorker() error = %v", err)
		}

		xDir := tilestore.TileDirPath(tilesPath, 7, 0)
		if imaging.FileExists(xDir) {
			t.Errorf("expected x directory %s to be removed when empty", xDir)
		}
	})
}

func TestGenerateTiles_generateTilesStep(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	tilesPath := t.TempDir()
	g := &GenerateTiles{TmpDir: tmp, TilesPath: tilesPath, Zoom: 9, xMin: 0, yMin: 0}

	// 2x2 grid of tiles; only the bottom-right tile (col 1, row 1) has content.
	cropped := image.NewNRGBA(image.Rect(0, 0, imaging.TileSize*2, imaging.TileSize*2))
	for y := imaging.TileSize; y < imaging.TileSize*2; y++ {
		for x := imaging.TileSize; x < imaging.TileSize*2; x++ {
			off := cropped.PixOffset(x, y)
			cropped.Pix[off+1], cropped.Pix[off+3] = 200, 255
		}
	}
	if err := imaging.SavePNG(g.overlayCropPath(), cropped); err != nil {
		t.Fatalf("saving cropped fixture: %v", err)
	}

	if err := g.generateTilesStep(); err != nil {
		t.Fatalf("generateTilesStep() error = %v", err)
	}

	wantTile := tilestore.TilePath(tilesPath, 9, 1, 1)
	if !imaging.FileExists(wantTile) {
		t.Errorf("expected tile at %s to exist", wantTile)
	}

	for _, coord := range []tilestore.Coord{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 0}} {
		p := tilestore.TilePath(tilesPath, 9, coord.X, coord.Y)
		if imaging.FileExists(p) {
			t.Errorf("expected no tile at %s (fully transparent)", p)
		}
	}
}
