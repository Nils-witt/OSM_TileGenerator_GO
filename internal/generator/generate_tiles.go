// Package generator warps a source overlay image onto the OSM tile grid via
// a 4-point homography and slices the result into 256x256 tile PNGs.
package generator

import (
	"fmt"
	"image"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

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
// homography, crops the warped canvas to its content, and slices it into
// 256x256 tile PNGs.
type GenerateTiles struct {
	TilesPath      string
	OverlayImgPath string
	Zoom           int
	TmpDir         string
	Markers        [4]Marker
	// Logger receives progress and diagnostic output; nil uses slog.Default().
	Logger *slog.Logger

	coordinates   [4][2]float64 // lat, lng
	picturePoints [4][2]float64 // x, y in source image pixel space

	xMin, xMax, yMin, yMax int
	mapImgOffsets          [4][2]int
}

// NewGenerateTiles returns a generator that warps the image at overlayImgPath
// onto the tile grid at zoom and writes tiles beneath tilesPath, using tmpDir
// for intermediate images. Set Logger on the result to capture progress.
func NewGenerateTiles(tilesPath, overlayImgPath string, markers [4]Marker, zoom int, tmpDir string) *GenerateTiles {
	return &GenerateTiles{
		TilesPath:      tilesPath,
		OverlayImgPath: overlayImgPath,
		Zoom:           zoom,
		TmpDir:         tmpDir,
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
// image will be warped into.
func (g *GenerateTiles) calculateTargetSize() (int, int) {
	return (g.xMax - g.xMin) * imaging.TileSize, (g.yMax - g.yMin) * imaging.TileSize
}

func (g *GenerateTiles) overlayPath() string {
	return filepath.Join(g.TmpDir, fmt.Sprintf("overlay_%d.png", g.Zoom))
}

func (g *GenerateTiles) overlayCropPath() string {
	return filepath.Join(g.TmpDir, fmt.Sprintf("overlay_%d_crop.png", g.Zoom))
}

// warpImage computes the homography from picturePoints -> mapImgOffsets and
// warps the source image into the target canvas, writing tmp/overlay_<zoom>.png.
func (g *GenerateTiles) warpImage() error {
	path := g.overlayPath()
	if imaging.FileExists(path) {
		g.logger().Info("skipping warp, output already exists", "path", path)
		return nil
	}

	var srcPts, dstPts [4][2]float64
	for i := range 4 {
		srcPts[i] = g.picturePoints[i]
		dstPts[i] = [2]float64{float64(g.mapImgOffsets[i][0]), float64(g.mapImgOffsets[i][1])}
	}

	h := geometry.SolveHomography(srcPts, dstPts)

	srcImg, err := imaging.LoadNRGBA(g.OverlayImgPath)
	if err != nil {
		return fmt.Errorf("warpImage: loading overlay image: %w", err)
	}

	targetW, targetH := g.calculateTargetSize()
	outImg := geometry.WarpPerspective(srcImg, h, targetW, targetH)

	if err := imaging.SavePNG(path, outImg); err != nil {
		return fmt.Errorf("warpImage: saving warped image: %w", err)
	}

	return nil
}

// cropImage trims the warped canvas down to just its non-transparent region,
// writing tmp/overlay_<zoom>_crop.png.
func (g *GenerateTiles) cropImage() error {
	cropPath := g.overlayCropPath()
	if imaging.FileExists(cropPath) {
		g.logger().Info("skipping crop, output already exists", "path", cropPath)
		return nil
	}

	img, err := imaging.LoadNRGBA(g.overlayPath())
	if err != nil {
		return fmt.Errorf("cropImage: loading warped image: %w", err)
	}

	lowerY := imaging.FindYBounds(img)
	rightX := imaging.FindXBounds(img)
	g.logger().Debug("cropping warped image", "right_x", rightX, "lower_y", lowerY)

	cropped := imaging.CropRegion(img, 0, 0, rightX, lowerY)
	if err := imaging.SavePNG(cropPath, cropped); err != nil {
		return fmt.Errorf("cropImage: saving cropped image: %w", err)
	}

	return nil
}

// tileWorker crops one 256x256 tile from img and saves it, unless the tile is
// fully transparent.
func (g *GenerateTiles) tileWorker(img *image.NRGBA, xOffset, yOffset int, savePath string) error {
	tile := imaging.CropRegion(img,
		xOffset*imaging.TileSize, yOffset*imaging.TileSize,
		xOffset*imaging.TileSize+imaging.TileSize, yOffset*imaging.TileSize+imaging.TileSize)
	if imaging.Empty(tile) {
		return nil
	}

	return imaging.SavePNG(savePath, tile)
}

// tileXWorker processes all tiles in one column of the cropped image, saving
// them under their global OSM tile coordinates.
func (g *GenerateTiles) tileXWorker(img *image.NRGBA, xOffset int) error {
	g.logger().Debug("slicing tile column", "x", xOffset+g.xMin)

	currentXDir := tilestore.TileDirPath(g.TilesPath, g.Zoom, xOffset+g.xMin)
	if err := os.MkdirAll(currentXDir, 0o755); err != nil {
		return err
	}

	yOffset := 0
	for float64(yOffset) < float64(img.Bounds().Dy())/float64(imaging.TileSize) {
		savePath := tilestore.TilePath(g.TilesPath, g.Zoom, xOffset+g.xMin, yOffset+g.yMin)
		if err := g.tileWorker(img, xOffset, yOffset, savePath); err != nil {
			return err
		}

		yOffset++
	}

	entries, err := os.ReadDir(currentXDir)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return os.Remove(currentXDir)
	}

	return nil
}

// generateTilesStep tiles the entire cropped overlay image column by column.
func (g *GenerateTiles) generateTilesStep() error {
	img, err := imaging.LoadNRGBA(g.overlayCropPath())
	if err != nil {
		return fmt.Errorf("generateTilesStep: loading cropped image: %w", err)
	}

	srcWidth := img.Bounds().Dx()
	g.logger().Debug("slicing cropped image",
		"zoom", g.Zoom,
		"width", srcWidth,
		"height", img.Bounds().Dy())

	xOffset := 0
	for float64(xOffset) < float64(srcWidth)/float64(imaging.TileSize) {
		if err := g.tileXWorker(img, xOffset); err != nil {
			return err
		}

		xOffset++
	}

	return nil
}

// Run executes the complete tile generation pipeline: reference points,
// warp, crop, and tile generation.
func (g *GenerateTiles) Run() error {
	g.logger().Info("generating tiles", "zoom", g.Zoom)
	g.createReferencePoints()
	g.prepareCoordinatesForWarp()

	if err := g.warpImage(); err != nil {
		return err
	}

	if err := g.cropImage(); err != nil {
		return err
	}

	if err := g.generateTilesStep(); err != nil {
		return err
	}

	g.logger().Info("generated tiles", "zoom", g.Zoom)

	return nil
}
