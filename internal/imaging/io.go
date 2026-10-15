// Package imaging provides generic NRGBA image I/O and pixel-buffer
// operations used throughout the tile generation pipeline.
package imaging

import (
	"image"
	"image/draw"
	"image/png"
	"os"
)

// TileSize is the pixel size of one OSM slippy-map tile.
const TileSize = 256

// LoadNRGBA decodes a PNG file into an *image.NRGBA regardless of its
// original concrete pixel format.
func LoadNRGBA(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only: a close error cannot lose data

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}

	return toNRGBA(img), nil
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}

	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)

	return out
}

// SavePNG encodes img as a PNG file at path. It reports errors from closing
// the file too, since a failed close can leave a truncated PNG on disk.
func SavePNG(path string, img image.Image) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return png.Encode(f, img)
}

// FileExists reports whether a file exists at path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Dimensions returns the pixel width and height of the image at path without
// decoding its full pixel data.
func Dimensions(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = f.Close() }() // read-only: a close error cannot lose data

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}

	return cfg.Width, cfg.Height, nil
}
