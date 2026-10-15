// Package generator warps a source overlay image onto the OSM tile grid via
// a 4-point homography, rendering it directly into 256x256 tile PNGs.
package generator

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/geometry"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/imaging"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/slippy"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/tilestore"
)

// osmTileOffset is the margin, in whole tiles, padded around the marker bounding
// box at the working zoom level. Mirrors create_marker_overlay.py's hardcoded
// osm_tile_offset = 20.
const osmTileOffset = 20

// Marker correlates one pixel position in the (already rescaled) source image
// with a map lat/lng, mirroring the "markers_scaled[i]" dict entries built in
// main.py.
type Marker struct {
	OverlayX float64
	OverlayY float64
	MapLat   float64
	MapLng   float64
}

// GenerateTiles mirrors create_marker_overlay.py's GenerateTiles class: it
// warps the source overlay image onto the OSM tile grid via a 4-point
// homography and writes the non-empty 256x256 tiles as PNGs.
//
// Unlike the Python original it never materializes the whole warped canvas,
// which spans at least 2*osmTileOffset tiles per side (>= 400 MB of NRGBA):
// each tile is rendered straight from the source image, in parallel.
type GenerateTiles struct {
	TilesPath      string
	OverlayImgPath string
	Zoom           int
	Markers        [4]Marker
	// Logger receives progress and diagnostic output; nil uses slog.Default().
	Logger *slog.Logger

	coordinates   [4][2]float64 // lat, lng
	picturePoints [4][2]float64 // x, y in source image pixel space

	xMin, xMax, yMin, yMax int
	mapImgOffsets          [4][2]int
}

// NewGenerateTiles returns a generator that warps the image at overlayImgPath
// onto the tile grid at zoom and writes tiles beneath tilesPath. Set Logger on
// the result to capture progress.
func NewGenerateTiles(tilesPath, overlayImgPath string, markers [4]Marker, zoom int) *GenerateTiles {
	return &GenerateTiles{
		TilesPath:      tilesPath,
		OverlayImgPath: overlayImgPath,
		Zoom:           zoom,
		Markers:        markers,
	}
}

// logger returns g.Logger, falling back to slog.Default() so a zero-value
// GenerateTiles still works.
func (g *GenerateTiles) logger() *slog.Logger {
	if g.Logger == nil {
		return slog.Default()
	}

	return g.Logger
}

// createReferencePoints extracts parallel lat/lng and pixel-coordinate lists
// from the 4 markers.
func (g *GenerateTiles) createReferencePoints() {
	for i, m := range g.Markers {
		g.coordinates[i] = [2]float64{m.MapLat, m.MapLng}
		g.picturePoints[i] = [2]float64{m.OverlayX, m.OverlayY}
		g.logger().Debug("loaded marker",
			"marker", i+1,
			"coordinate", g.coordinates[i],
			"picture_point", g.picturePoints[i])
	}
}

// prepareCoordinatesForWarp computes the OSM tile bounding box (with the
// osmTileOffset margin) around the 4 markers at the working zoom, and the
// pixel offsets of each marker within the eventual warped canvas.
func (g *GenerateTiles) prepareCoordinatesForWarp() {
	var markerOsmTiles [4][2]float64

	for i, coord := range g.coordinates {
		x, y := slippy.Deg2Num(coord[0], coord[1], g.Zoom)
		markerOsmTiles[i] = [2]float64{x, y}
	}

	xTiles := make([]int, 4)
	yTiles := make([]int, 4)

	for i, t := range markerOsmTiles {
		xTiles[i] = int(t[0])
		yTiles[i] = int(t[1])
	}

	sort.Ints(xTiles)
	sort.Ints(yTiles)

	g.yMin = yTiles[0] - osmTileOffset
	g.yMax = yTiles[3] + osmTileOffset
	g.xMin = xTiles[0] - osmTileOffset
	g.xMax = xTiles[3] + osmTileOffset

	g.logger().Debug("computed tile bounds",
		"zoom", g.Zoom,
		"x_min", g.xMin, "x_max", g.xMax, "x_count", g.xMax-g.xMin+1,
		"y_min", g.yMin, "y_max", g.yMax, "y_count", g.yMax-g.yMin+1)

	for i, val := range markerOsmTiles {
		g.logger().Debug("marker position",
			"marker", i+1,
			"zoom", g.Zoom,
			"overlay_x", g.picturePoints[i][0],
			"overlay_y", g.picturePoints[i][1],
			"map_x", (val[0]-float64(g.xMin))*imaging.TileSize,
			"map_y", (val[1]-float64(g.yMin))*imaging.TileSize)
	}

	for i, val := range markerOsmTiles {
		g.mapImgOffsets[i] = [2]int{
			int((val[0] - float64(g.xMin)) * imaging.TileSize),
			int((val[1] - float64(g.yMin)) * imaging.TileSize),
		}
	}

	g.logger().Debug("computed map image offsets", "offsets", g.mapImgOffsets)
}

// calculateTargetSize returns the pixel dimensions of the canvas the source
// image is warped into.
func (g *GenerateTiles) calculateTargetSize() (int, int) {
	return (g.xMax - g.xMin) * imaging.TileSize, (g.yMax - g.yMin) * imaging.TileSize
}

// homography returns the forward homography mapping source image pixels
// (picturePoints) to warped canvas pixels (mapImgOffsets).
func (g *GenerateTiles) homography() [3][3]float64 {
	var srcPts, dstPts [4][2]float64
	for i := range 4 {
		srcPts[i] = g.picturePoints[i]
		dstPts[i] = [2]float64{float64(g.mapImgOffsets[i][0]), float64(g.mapImgOffsets[i][1])}
	}

	return geometry.SolveHomography(srcPts, dstPts)
}

// tileRange returns the half-open range of canvas tile offsets that can
// contain warped content: the whole canvas, narrowed to the warp's footprint
// when it has a finite one. Skipping the transparent margin outright is what
// makes rendering cost proportional to the overlay, not to the padded canvas.
func (g *GenerateTiles) tileRange(w *geometry.Warper) image.Rectangle {
	targetW, targetH := g.calculateTargetSize()

	area := image.Rect(0, 0, targetW, targetH)
	if fp, ok := w.Footprint(); ok {
		area = area.Intersect(fp)
	}

	if area.Empty() {
		return image.Rectangle{}
	}

	// area.Min is never negative, so integer division floors.
	return image.Rect(
		area.Min.X/imaging.TileSize,
		area.Min.Y/imaging.TileSize,
		(area.Max.X+imaging.TileSize-1)/imaging.TileSize,
		(area.Max.Y+imaging.TileSize-1)/imaging.TileSize,
	)
}

// tilePool recycles tile render buffers: most rendered tiles near the
// overlay's edges turn out empty and are discarded straight away.
var tilePool = sync.Pool{
	New: func() any { return newTile() },
}

func newTile() *image.NRGBA {
	return image.NewNRGBA(image.Rect(0, 0, imaging.TileSize, imaging.TileSize))
}

// renderTiles renders every tile in g.tileRange concurrently, saving the
// non-empty ones under their global OSM tile coordinates. It returns how many
// tiles were written. The first error stops further tiles from starting.
func (g *GenerateTiles) renderTiles(w *geometry.Warper) (int, error) {
	tiles := g.tileRange(w)
	g.logger().Debug("rendering tiles",
		"zoom", g.Zoom,
		"x_from", tiles.Min.X+g.xMin, "x_to", tiles.Max.X+g.xMin,
		"y_from", tiles.Min.Y+g.yMin, "y_to", tiles.Max.Y+g.yMin)

	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(runtime.GOMAXPROCS(0))

	var written atomic.Int64

schedule:
	for ty := tiles.Min.Y; ty < tiles.Max.Y; ty++ {
		for tx := tiles.Min.X; tx < tiles.Max.X; tx++ {
			if ctx.Err() != nil {
				break schedule
			}

			eg.Go(func() error {
				ok, err := g.renderTile(w, tx, ty)
				if ok {
					written.Add(1)
				}

				return err
			})
		}
	}

	err := eg.Wait()

	return int(written.Load()), err
}

// renderTile renders the canvas tile at offset (tx, ty) and saves it unless
// it is fully transparent. It reports whether a tile was written.
func (g *GenerateTiles) renderTile(w *geometry.Warper, tx, ty int) (bool, error) {
	tile, ok := tilePool.Get().(*image.NRGBA)
	if !ok {
		tile = newTile()
	}

	defer func() {
		// Render expects a transparent buffer.
		clear(tile.Pix)
		tilePool.Put(tile)
	}()

	if !w.Render(tile, tx*imaging.TileSize, ty*imaging.TileSize) {
		return false, nil
	}

	x, y := tx+g.xMin, ty+g.yMin
	if err := os.MkdirAll(tilestore.TileDirPath(g.TilesPath, g.Zoom, x), 0o755); err != nil {
		return false, fmt.Errorf("renderTile: creating column %d: %w", x, err)
	}

	path := tilestore.TilePath(g.TilesPath, g.Zoom, x, y)
	if err := imaging.SavePNG(path, tile); err != nil {
		return false, fmt.Errorf("renderTile: saving tile %s: %w", path, err)
	}

	return true, nil
}

// Run executes the complete tile generation pipeline: reference points, tile
// bounds, then warping the source image tile by tile.
func (g *GenerateTiles) Run() error {
	g.logger().Info("generating tiles", "zoom", g.Zoom)
	g.createReferencePoints()
	g.prepareCoordinatesForWarp()

	// Create the zoom directory even if no tile ends up non-empty, so the
	// next lower zoom level finds (and walks) an empty level, not a missing one.
	zoomDir := filepath.Join(g.TilesPath, strconv.Itoa(g.Zoom))
	if err := os.MkdirAll(zoomDir, 0o755); err != nil {
		return fmt.Errorf("Run: creating zoom directory: %w", err)
	}

	src, err := imaging.LoadNRGBA(g.OverlayImgPath)
	if err != nil {
		return fmt.Errorf("Run: loading overlay image: %w", err)
	}

	n, err := g.renderTiles(geometry.NewWarper(src, g.homography()))
	if err != nil {
		return err
	}

	g.logger().Info("generated tiles", "zoom", g.Zoom, "tiles", n)

	return nil
}
