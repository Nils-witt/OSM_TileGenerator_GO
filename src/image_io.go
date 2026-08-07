package main

import (
	"image"
	"image/draw"
	"image/png"
	"os"
)

// loadImageNRGBA decodes a PNG file into an *image.NRGBA regardless of its
// original concrete pixel format.
func loadImageNRGBA(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// cropRegion extracts the region [x0,y0)-[x1,y1) from img, top-left anchored at
// (0,0) in the result. Mirrors PIL's Image.crop: parts of the requested region
// that fall outside img's bounds are filled with zero bytes (fully transparent),
// rather than erroring or clamping.
func cropRegion(img *image.NRGBA, x0, y0, x1, y1 int) *image.NRGBA {
	w := x1 - x0
	h := y1 - y0
	out := image.NewNRGBA(image.Rect(0, 0, w, h))

	srcB := img.Bounds()
	for y := 0; y < h; y++ {
		sy := y0 + y
		if sy < srcB.Min.Y || sy >= srcB.Max.Y {
			continue
		}
		xStart := 0
		if x0 < srcB.Min.X {
			xStart = srcB.Min.X - x0
		}
		xEnd := w
		if x0+w > srcB.Max.X {
			xEnd = srcB.Max.X - x0
		}
		if xEnd <= xStart {
			continue
		}
		srcOff := img.PixOffset(x0+xStart, sy)
		dstOff := out.PixOffset(xStart, y)
		n := (xEnd - xStart) * 4
		copy(out.Pix[dstOff:dstOff+n], img.Pix[srcOff:srcOff+n])
	}
	return out
}
