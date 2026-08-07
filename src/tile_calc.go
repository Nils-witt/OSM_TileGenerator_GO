package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"os"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Resize builds zoom level targetZoom entirely from the tiles present at
// targetZoom+1. Mirrors tile_calc.py::resize. Must be called in descending
// zoom order since each level depends on its immediate parent already
// existing on disk.
func Resize(tilePath string, targetZoom int) error {
	sourceDir := fmt.Sprintf("%s/%d", tilePath, targetZoom+1)
	xDirs, err := os.ReadDir(sourceDir)
	if err != nil {
		return err
	}

	type tileCoord struct{ x, y int }
	var tiles []tileCoord

	for _, xDir := range xDirs {
		if !xDir.IsDir() {
			continue
		}
		xVal, err := strconv.Atoi(xDir.Name())
		if err != nil {
			continue
		}
		yEntries, err := os.ReadDir(fmt.Sprintf("%s/%s", sourceDir, xDir.Name()))
		if err != nil {
			return err
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
			tiles = append(tiles, tileCoord{xVal, yVal})
		}
	}

	targetDir := fmt.Sprintf("%s/%d", tilePath, targetZoom)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	newTilesSet := map[int]map[int]bool{}

	for _, tile := range tiles {
		latDeg, lonDeg := num2deg(float64(tile.x), float64(tile.y), targetZoom+1)
		newX, newY := deg2num(latDeg, lonDeg, targetZoom)

		parentX := int(newX)
		parentY := int(newY)

		currentTilePath := fmt.Sprintf("%s/%d/%d.png", sourceDir, tile.x, tile.y)
		parentXDir := fmt.Sprintf("%s/%d", targetDir, parentX)
		resizedTilePath := fmt.Sprintf("%s/%d.png", parentXDir, parentY)

		boundsX := int((newX - float64(parentX)) * 512)
		boundsY := int((newY - float64(parentY)) * 512)

		if newTilesSet[parentX] == nil {
			newTilesSet[parentX] = map[int]bool{}
		}
		newTilesSet[parentX][parentY] = true

		childImg, err := loadImageNRGBA(currentTilePath)
		if err != nil {
			return fmt.Errorf("Resize: loading child tile %s: %w", currentTilePath, err)
		}

		if err := os.MkdirAll(parentXDir, 0o755); err != nil {
			return err
		}

		var parentImg *image.NRGBA
		if !fileExists(resizedTilePath) {
			parentImg = image.NewNRGBA(image.Rect(0, 0, 512, 512))
		} else {
			parentImg, err = loadImageNRGBA(resizedTilePath)
			if err != nil {
				return fmt.Errorf("Resize: loading existing parent tile %s: %w", resizedTilePath, err)
			}
		}

		pasteDirect(parentImg, childImg, boundsX, boundsY)

		if err := savePNG(resizedTilePath, parentImg); err != nil {
			return fmt.Errorf("Resize: saving parent tile %s: %w", resizedTilePath, err)
		}
	}

	for parentX, ys := range newTilesSet {
		for parentY := range ys {
			path := fmt.Sprintf("%s/%d/%d.png", targetDir, parentX, parentY)
			img, err := loadImageNRGBA(path)
			if err != nil {
				return fmt.Errorf("Resize: loading assembled tile %s: %w", path, err)
			}
			resized := resizeNRGBA(img, tileSize, tileSize)
			if err := savePNG(path, resized); err != nil {
				return fmt.Errorf("Resize: saving downsampled tile %s: %w", path, err)
			}
		}
	}

	return nil
}

// pasteDirect copies src's pixel bytes directly into dst at (x0, y0), fully
// overwriting whatever was there (including alpha) rather than alpha
// compositing. Mirrors PIL's Image.paste(src, (x, y)) without a mask.
func pasteDirect(dst, src *image.NRGBA, x0, y0 int) {
	srcB := src.Bounds()
	dstB := dst.Bounds()

	for y := 0; y < srcB.Dy(); y++ {
		dy := y0 + y
		if dy < dstB.Min.Y || dy >= dstB.Max.Y {
			continue
		}
		rowW := srcB.Dx()
		dxStart := 0
		if x0 < dstB.Min.X {
			dxStart = dstB.Min.X - x0
		}
		dxEnd := rowW
		if x0+rowW > dstB.Max.X {
			dxEnd = dstB.Max.X - x0
		}
		if dxEnd <= dxStart {
			continue
		}
		srcOff := src.PixOffset(srcB.Min.X+dxStart, srcB.Min.Y+y)
		dstOff := dst.PixOffset(x0+dxStart, dy)
		n := (dxEnd - dxStart) * 4
		copy(dst.Pix[dstOff:dstOff+n], src.Pix[srcOff:srcOff+n])
	}
}

// resizeNRGBA scales img to w x h using a Catmull-Rom (bicubic-family) filter,
// standing in for PIL's default bicubic Image.resize().
func resizeNRGBA(img *image.NRGBA, w, h int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return dst
}

// CreateCanvas stitches together all tiles at targetZoom into one large
// alpha-composited canvas.png. Mirrors tile_calc.py::create_canvas. Unused by
// the main CLI flow, ported for parity with the Python module which also
// leaves it unused but present.
func CreateCanvas(dataDir string, targetZoom int) error {
	zoomDir := fmt.Sprintf("%s/%d", dataDir, targetZoom)
	xEntries, err := os.ReadDir(zoomDir)
	if err != nil {
		return err
	}

	var xMin, xMax, yMin, yMax int
	haveX, haveY := false, false

	type tileCoord struct{ x, y int }
	var tiles []tileCoord

	for _, xEntry := range xEntries {
		if !xEntry.IsDir() {
			continue
		}
		xVal, err := strconv.Atoi(xEntry.Name())
		if err != nil {
			continue
		}
		if !haveX || xVal < xMin {
			xMin = xVal
		}
		if !haveX || xVal > xMax {
			xMax = xVal
		}
		haveX = true

		yEntries, err := os.ReadDir(fmt.Sprintf("%s/%s", zoomDir, xEntry.Name()))
		if err != nil {
			return err
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
			if !haveY || yVal < yMin {
				yMin = yVal
			}
			if !haveY || yVal > yMax {
				yMax = yVal
			}
			haveY = true
			tiles = append(tiles, tileCoord{xVal, yVal})
		}
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, (1+xMax-xMin)*tileSize, (1+yMax-yMin)*tileSize))
	for i := 0; i < len(canvas.Pix); i += 4 {
		canvas.Pix[i] = 255
	}

	for _, t := range tiles {
		xOffset := (t.x - xMin) * tileSize
		yOffset := (t.y - yMin) * tileSize
		tilePath := fmt.Sprintf("%s/%d/%d.png", zoomDir, t.x, t.y)
		tileImg, err := loadImageNRGBA(tilePath)
		if err != nil {
			return fmt.Errorf("CreateCanvas: loading tile %s: %w", tilePath, err)
		}
		rect := image.Rect(xOffset, yOffset, xOffset+tileImg.Bounds().Dx(), yOffset+tileImg.Bounds().Dy())
		draw.Draw(canvas, rect, tileImg, tileImg.Bounds().Min, draw.Over)
	}

	return savePNG(fmt.Sprintf("%s/canvas.png", zoomDir), canvas)
}

// CreateIndexJSON walks every zoom/x directory under dataDir and writes an
// index.json listing the tile filenames present at each zoom/x. Mirrors
// tile_calc.py::create_index_json. Unused by the main CLI flow, ported for
// parity with the Python module.
func CreateIndexJSON(dataDir string) error {
	globalIndex := map[string]map[string][]string{}

	zoomEntries, err := os.ReadDir(dataDir)
	if err != nil {
		return err
	}
	for _, zoomEntry := range zoomEntries {
		if !zoomEntry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(zoomEntry.Name()); err != nil {
			continue
		}
		zoomDir := fmt.Sprintf("%s/%s", dataDir, zoomEntry.Name())
		zoomIndex := map[string][]string{}

		xEntries, err := os.ReadDir(zoomDir)
		if err != nil {
			return err
		}
		for _, xEntry := range xEntries {
			if !xEntry.IsDir() {
				continue
			}
			if _, err := strconv.Atoi(xEntry.Name()); err != nil {
				continue
			}
			yEntries, err := os.ReadDir(fmt.Sprintf("%s/%s", zoomDir, xEntry.Name()))
			if err != nil {
				return err
			}
			xArray := make([]string, 0, len(yEntries))
			for _, yEntry := range yEntries {
				xArray = append(xArray, yEntry.Name())
			}
			zoomIndex[xEntry.Name()] = xArray
		}
		globalIndex[zoomEntry.Name()] = zoomIndex
	}

	data, err := json.MarshalIndent(globalIndex, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("%s/index.json", dataDir), data, 0o644)
}
