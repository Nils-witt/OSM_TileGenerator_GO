package main

import (
	"image"
	"math"
)

// warpPerspective rasterizes src into a canvas of size targetW x targetH using the
// forward homography h (mapping src pixel coords -> dest pixel coords). For each
// destination pixel it applies h^-1 to find the source sample point and bilinearly
// interpolates (matching OpenCV's default INTER_LINEAR). Destination pixels whose
// source sample would require reading outside the source image bounds are left
// fully transparent, matching cv2.warpPerspective's BORDER_TRANSPARENT.
func warpPerspective(src *image.NRGBA, h [3][3]float64, targetW, targetH int) *image.NRGBA {
	hInv := invert3x3(h)
	dst := image.NewNRGBA(image.Rect(0, 0, targetW, targetH))

	for dy := 0; dy < targetH; dy++ {
		px := float64(dy)
		for dx := 0; dx < targetW; dx++ {
			x := float64(dx)

			denom := hInv[2][0]*x + hInv[2][1]*px + hInv[2][2]
			if denom == 0 {
				continue
			}
			sx := (hInv[0][0]*x + hInv[0][1]*px + hInv[0][2]) / denom
			sy := (hInv[1][0]*x + hInv[1][1]*px + hInv[1][2]) / denom

			r, g, b, a, ok := bilinearSample(src, sx, sy)
			if !ok {
				continue
			}

			off := dst.PixOffset(dx, dy)
			dst.Pix[off] = r
			dst.Pix[off+1] = g
			dst.Pix[off+2] = b
			dst.Pix[off+3] = a
		}
	}

	return dst
}

// bilinearSample samples src at fractional coordinate (sx, sy). It returns ok=false
// (leave destination untouched/transparent) if any of the 4 neighboring pixels needed
// for interpolation lies outside the source image bounds.
func bilinearSample(src *image.NRGBA, sx, sy float64) (r, g, b, a uint8, ok bool) {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	x0 := int(math.Floor(sx))
	y0 := int(math.Floor(sy))
	x1 := x0 + 1
	y1 := y0 + 1

	if x0 < 0 || y0 < 0 || x1 >= w || y1 >= h {
		return 0, 0, 0, 0, false
	}

	fx := sx - float64(x0)
	fy := sy - float64(y0)

	get := func(x, y int) (float64, float64, float64, float64) {
		off := src.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
		return float64(src.Pix[off]), float64(src.Pix[off+1]), float64(src.Pix[off+2]), float64(src.Pix[off+3])
	}

	r00, g00, b00, a00 := get(x0, y0)
	r10, g10, b10, a10 := get(x1, y0)
	r01, g01, b01, a01 := get(x0, y1)
	r11, g11, b11, a11 := get(x1, y1)

	lerp := func(v00, v10, v01, v11 float64) float64 {
		top := v00*(1-fx) + v10*fx
		bot := v01*(1-fx) + v11*fx
		return top*(1-fy) + bot*fy
	}

	rf := lerp(r00, r10, r01, r11)
	gf := lerp(g00, g10, g01, g11)
	bf := lerp(b00, b10, b01, b11)
	af := lerp(a00, a10, a01, a11)

	return uint8(math.Round(rf)), uint8(math.Round(gf)), uint8(math.Round(bf)), uint8(math.Round(af)), true
}
