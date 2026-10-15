package geometry

import (
	"image"
	"math"
)

// Warper renders a source image through a forward homography (mapping src
// pixel coords -> dest pixel coords) into arbitrary rectangular regions of the
// destination canvas. For each destination pixel it applies h^-1 to find the
// source sample point and bilinearly interpolates (matching OpenCV's default
// INTER_LINEAR). Destination pixels whose source sample would require reading
// outside the source image bounds are left untouched, matching
// cv2.warpPerspective's BORDER_TRANSPARENT.
//
// A Warper never mutates its source and is safe for concurrent use.
type Warper struct {
	src  sampler
	h    [3][3]float64
	hInv [3][3]float64
}

// NewWarper returns a Warper for src and the forward homography h. It panics
// if h is singular.
func NewWarper(src *image.NRGBA, h [3][3]float64) *Warper {
	return &Warper{src: newSampler(src), h: h, hInv: invert3x3(h)}
}

// WarpPerspective rasterizes src into a canvas of size targetW x targetH using the
// forward homography h. See Warper for the sampling semantics.
func WarpPerspective(src *image.NRGBA, h [3][3]float64, targetW, targetH int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, targetW, targetH))
	NewWarper(src, h).Render(dst, 0, 0)

	return dst
}

// Render fills dst with the part of the warped canvas whose top-left corner is
// at canvas coordinate (originX, originY). dst should be fully transparent on
// entry, since pixels outside the source footprint are left untouched. It
// reports whether any non-zero byte was written, so callers can skip saving
// empty tiles without rescanning dst.
func (w *Warper) Render(dst *image.NRGBA, originX, originY int) bool {
	hInv := w.hInv
	b := dst.Bounds()
	nonEmpty := false

	for dy := range b.Dy() {
		px := float64(originY + dy)
		row := dst.Pix[dst.PixOffset(b.Min.X, b.Min.Y+dy):]

		for dx := range b.Dx() {
			x := float64(originX + dx)

			denom := hInv[2][0]*x + hInv[2][1]*px + hInv[2][2]
			if denom == 0 {
				continue
			}

			sx := (hInv[0][0]*x + hInv[0][1]*px + hInv[0][2]) / denom
			sy := (hInv[1][0]*x + hInv[1][1]*px + hInv[1][2]) / denom

			r, g, bl, a, ok := w.src.sample(sx, sy)
			if !ok {
				continue
			}

			p := row[dx*4 : dx*4+4 : dx*4+4]
			p[0], p[1], p[2], p[3] = r, g, bl, a
			nonEmpty = nonEmpty || r|g|bl|a != 0
		}
	}

	return nonEmpty
}

// maxFootprintCoord bounds projected corner coordinates so converting them to
// int cannot overflow; anything larger is treated as unbounded.
const maxFootprintCoord = 1 << 30

// Footprint returns the destination-space rectangle outside of which Render
// never writes a pixel. ok is false when no finite bound exists because the
// projection of the source crosses the line at infinity; callers must then
// treat the whole canvas as potentially covered.
func (w *Warper) Footprint() (r image.Rectangle, ok bool) {
	// Render only writes where the sample point lies in [0, srcW-1) x [0, srcH-1).
	maxX, maxY := float64(w.src.w-1), float64(w.src.h-1)
	if maxX <= 0 || maxY <= 0 {
		return image.Rectangle{}, true
	}

	// The homography's denominator is affine in (x, y), so if it has the same
	// strict sign at all four corners it keeps that sign over the whole
	// (convex) source rectangle. The source then maps to a convex quad whose
	// bounding box is that of its corner images.
	minDX, minDY := math.Inf(1), math.Inf(1)
	maxDX, maxDY := math.Inf(-1), math.Inf(-1)
	sign := 0.0

	for _, c := range [4][2]float64{{0, 0}, {maxX, 0}, {0, maxY}, {maxX, maxY}} {
		denom := w.h[2][0]*c[0] + w.h[2][1]*c[1] + w.h[2][2]
		if denom == 0 || (sign != 0 && math.Signbit(denom) != math.Signbit(sign)) {
			return image.Rectangle{}, false
		}

		sign = denom

		u := (w.h[0][0]*c[0] + w.h[0][1]*c[1] + w.h[0][2]) / denom
		v := (w.h[1][0]*c[0] + w.h[1][1]*c[1] + w.h[1][2]) / denom
		// Also rejects NaN, which fails every comparison.
		if !(math.Abs(u) < maxFootprintCoord && math.Abs(v) < maxFootprintCoord) {
			return image.Rectangle{}, false
		}

		minDX, maxDX = min(minDX, u), max(maxDX, u)
		minDY, maxDY = min(minDY, v), max(maxDY, v)
	}

	// Pad by a pixel on each side to absorb rounding between the forward and
	// inverse mappings.
	return image.Rect(
		int(math.Floor(minDX))-1, int(math.Floor(minDY))-1,
		int(math.Ceil(maxDX))+2, int(math.Ceil(maxDY))+2,
	), true
}

// sampler caches the source image fields the per-pixel bilinear fetch needs,
// so the hot loop does not recompute bounds for every pixel.
type sampler struct {
	pix    []uint8
	stride int
	w, h   int
}

func newSampler(src *image.NRGBA) sampler {
	b := src.Bounds()

	return sampler{pix: src.Pix, stride: src.Stride, w: b.Dx(), h: b.Dy()}
}

// bilinearSample samples src at fractional coordinate (sx, sy). It returns ok=false
// (leave destination untouched/transparent) if any of the 4 neighboring pixels needed
// for interpolation lies outside the source image bounds.
func bilinearSample(src *image.NRGBA, sx, sy float64) (r, g, b, a uint8, ok bool) {
	s := newSampler(src)

	return s.sample(sx, sy)
}

func (s *sampler) sample(sx, sy float64) (r, g, b, a uint8, ok bool) {
	x0 := int(math.Floor(sx))
	y0 := int(math.Floor(sy))

	if x0 < 0 || y0 < 0 || x0+1 >= s.w || y0+1 >= s.h {
		return 0, 0, 0, 0, false
	}

	fx := sx - float64(x0)
	fy := sy - float64(y0)

	// The two pixels of the neighborhood's top row, then its bottom row, each
	// as 8 contiguous bytes.
	topOff := y0*s.stride + x0*4
	botOff := topOff + s.stride
	top := s.pix[topOff : topOff+8 : topOff+8]
	bot := s.pix[botOff : botOff+8 : botOff+8]

	return interp(top[0], top[4], bot[0], bot[4], fx, fy),
		interp(top[1], top[5], bot[1], bot[5], fx, fy),
		interp(top[2], top[6], bot[2], bot[6], fx, fy),
		interp(top[3], top[7], bot[3], bot[7], fx, fy),
		true
}

// interp bilinearly blends one channel of a 2x2 neighborhood. Keep the
// expression shape unchanged: reordering it alters float rounding and shifts
// output pixels by one level.
func interp(v00, v10, v01, v11 uint8, fx, fy float64) uint8 {
	top := float64(v00)*(1-fx) + float64(v10)*fx
	bot := float64(v01)*(1-fx) + float64(v11)*fx

	return uint8(math.Round(top*(1-fy) + bot*fy))
}
