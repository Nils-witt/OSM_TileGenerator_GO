package main

import (
	"fmt"
	"image"
	"log"
	"os"
	"sort"
)

// osmTileOffset is the margin, in whole tiles, padded around the marker bounding
// box at the working zoom level. Mirrors create_marker_overlay.py's hardcoded
// osm_tile_offset = 20.
const osmTileOffset = 20

// tileSize is the pixel size of one OSM slippy-map tile.
const tileSize = 256

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

	coordinates   [4][2]float64 // lat, lng
	picturePoints [4][2]float64 // x, y in source image pixel space

	xMin, xMax, yMin, yMax int
	mapImgOffsets          [4][2]int
}

func NewGenerateTiles(tilesPath, overlayImgPath string, markers [4]Marker, zoom int, tmpDir string) *GenerateTiles {
	return &GenerateTiles{
		TilesPath:      tilesPath,
		OverlayImgPath: overlayImgPath,
		Zoom:           zoom,
		TmpDir:         tmpDir,
		Markers:        markers,
	}
}

// createReferencePoints extracts parallel lat/lng and pixel-coordinate lists
// from the 4 markers.
func (g *GenerateTiles) createReferencePoints() {
	for i, m := range g.Markers {
		g.coordinates[i] = [2]float64{m.MapLat, m.MapLng}
		g.picturePoints[i] = [2]float64{m.OverlayX, m.OverlayY}
		log.Printf("[GenerateTiles] loaded coordinate %v", g.coordinates[i])
		log.Printf("[GenerateTiles] loaded picture point %v", g.picturePoints[i])
	}
}

// prepareCoordinatesForWarp computes the OSM tile bounding box (with the
// osmTileOffset margin) around the 4 markers at the working zoom, and the
// pixel offsets of each marker within the eventual warped canvas.
func (g *GenerateTiles) prepareCoordinatesForWarp() {
	var markerOsmTiles [4][2]float64
	for i, coord := range g.coordinates {
		x, y := deg2num(coord[0], coord[1], g.Zoom)
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

	log.Printf("[GenerateTiles] X-Tile-Bounds[%d]: max: %d min: %d, count: %d", g.Zoom, g.xMax, g.xMin, g.xMax-g.xMin+1)
	log.Printf("[GenerateTiles] Y-Tile-Bounds[%d]: max: %d min: %d, count: %d", g.Zoom, g.yMax, g.yMin, g.yMax-g.yMin+1)

	for i, val := range markerOsmTiles {
		log.Printf("[GenerateTiles] [Marker %d][%d] Overlay [%v, %v] Map [%v, %v]",
			i+1, g.Zoom, g.picturePoints[i][0], g.picturePoints[i][1],
			(val[0]-float64(g.xMin))*tileSize, (val[1]-float64(g.yMin))*tileSize)
	}

	for i, val := range markerOsmTiles {
		g.mapImgOffsets[i] = [2]int{
			int((val[0] - float64(g.xMin)) * tileSize),
			int((val[1] - float64(g.yMin)) * tileSize),
		}
	}
	log.Printf("[GenerateTiles] Map Image Offsets: %v", g.mapImgOffsets)
}

// calculateTargetSize returns the pixel dimensions of the canvas the source
// image will be warped into.
func (g *GenerateTiles) calculateTargetSize() (int, int) {
	return (g.xMax - g.xMin) * tileSize, (g.yMax - g.yMin) * tileSize
}

func (g *GenerateTiles) overlayPath() string {
	return fmt.Sprintf("%s/overlay_%d.png", g.TmpDir, g.Zoom)
}

func (g *GenerateTiles) overlayCropPath() string {
	return fmt.Sprintf("%s/overlay_%d_crop.png", g.TmpDir, g.Zoom)
}

// warpImage computes the homography from picturePoints -> mapImgOffsets and
// warps the source image into the target canvas, writing tmp/overlay_<zoom>.png.
func (g *GenerateTiles) warpImage() error {
	path := g.overlayPath()
	if fileExists(path) {
		log.Printf("[GenerateTiles] Skipping Warp, already exists")
		return nil
	}

	var srcPts, dstPts [4][2]float64
	for i := 0; i < 4; i++ {
		srcPts[i] = g.picturePoints[i]
		dstPts[i] = [2]float64{float64(g.mapImgOffsets[i][0]), float64(g.mapImgOffsets[i][1])}
	}

	h := solveHomography(srcPts, dstPts)

	srcImg, err := loadImageNRGBA(g.OverlayImgPath)
	if err != nil {
		return fmt.Errorf("warpImage: loading overlay image: %w", err)
	}

	targetW, targetH := g.calculateTargetSize()
	outImg := warpPerspective(srcImg, h, targetW, targetH)

	if err := savePNG(path, outImg); err != nil {
		return fmt.Errorf("warpImage: saving warped image: %w", err)
	}
	return nil
}

// cropImage trims the warped canvas down to just its non-transparent region,
// writing tmp/overlay_<zoom>_crop.png.
func (g *GenerateTiles) cropImage() error {
	log.Printf("[GenerateTiles] Cropping image")
	cropPath := g.overlayCropPath()
	if fileExists(cropPath) {
		log.Printf("[GenerateTiles] Skipping crop, already exists")
		return nil
	}

	img, err := loadImageNRGBA(g.overlayPath())
	if err != nil {
		return fmt.Errorf("cropImage: loading warped image: %w", err)
	}

	lowerY := findYBounds(img)
	rightX := findXBounds(img)
	log.Printf("[GenerateTiles] YL: %d", lowerY)
	log.Printf("[GenerateTiles] XR: %d", rightX)

	cropped := cropRegion(img, 0, 0, rightX, lowerY)
	if err := savePNG(cropPath, cropped); err != nil {
		return fmt.Errorf("cropImage: saving cropped image: %w", err)
	}
	log.Printf("[GenerateTiles] Cropping image done")
	return nil
}

// tileWorker crops one 256x256 tile from img and saves it, unless the tile is
// fully transparent.
func (g *GenerateTiles) tileWorker(img *image.NRGBA, xOffset, yOffset int, savePath string) error {
	tile := cropRegion(img,
		xOffset*tileSize, yOffset*tileSize,
		xOffset*tileSize+tileSize, yOffset*tileSize+tileSize)
	if tileEmpty(tile) {
		return nil
	}
	return savePNG(savePath, tile)
}

// tileXWorker processes all tiles in one column of the cropped image, saving
// them under their global OSM tile coordinates.
func (g *GenerateTiles) tileXWorker(img *image.NRGBA, xOffset int) error {
	log.Printf("[GenerateTiles] Processing X Column %d", xOffset)

	currentXDir := fmt.Sprintf("%s/%d/%d", g.TilesPath, g.Zoom, xOffset+g.xMin)
	if err := os.MkdirAll(currentXDir, 0o755); err != nil {
		return err
	}

	yOffset := 0
	for float64(yOffset) < float64(img.Bounds().Dy())/float64(tileSize) {
		savePath := fmt.Sprintf("%s/%d.png", currentXDir, yOffset+g.yMin)
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
	img, err := loadImageNRGBA(g.overlayCropPath())
	if err != nil {
		return fmt.Errorf("generateTilesStep: loading cropped image: %w", err)
	}

	srcWidth := img.Bounds().Dx()
	log.Printf("[GenerateTiles] Src Image[%d]: %d, %d", g.Zoom, srcWidth, img.Bounds().Dy())

	xOffset := 0
	for float64(xOffset) < float64(srcWidth)/float64(tileSize) {
		if err := g.tileXWorker(img, xOffset); err != nil {
			return err
		}
		xOffset++
	}
	log.Printf("[GenerateTiles] Done alle")
	return nil
}

// Run executes the complete tile generation pipeline: reference points,
// warp, crop, and tile generation.
func (g *GenerateTiles) Run() error {
	log.Printf("[GenerateTiles] Starting")
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
	log.Printf("[GenerateTiles] Done")
	return nil
}
