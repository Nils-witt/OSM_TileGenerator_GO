package geometry

import (
	"bytes"
	"image"
	"math"
	"testing"
)

func newTestNRGBA(w, h int, fill func(x, y int) [4]uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := fill(x, y)
			off := img.PixOffset(x, y)
			img.Pix[off] = c[0]
			img.Pix[off+1] = c[1]
			img.Pix[off+2] = c[2]
			img.Pix[off+3] = c[3]
		}
	}

	return img
}

func TestWarpPerspective(t *testing.T) {
	t.Parallel()

	t.Run("identity homography reproduces interior pixels", func(t *testing.T) {
		t.Parallel()

		src := newTestNRGBA(4, 4, func(x, y int) [4]uint8 {
			return [4]uint8{uint8(x * 50), uint8(y * 50), 100, 255}
		})
		identity := [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}

		dst := WarpPerspective(src, identity, 4, 4)

		// Bilinear sampling needs the neighbor one pixel to the right/below in
		// bounds, so only the interior (excluding the last row/column) can be
		// reproduced exactly; the last row/column is expected to be transparent.
		for y := range 3 {
			for x := range 3 {
				sOff := src.PixOffset(x, y)
				dOff := dst.PixOffset(x, y)
				gotPix := dst.Pix[dOff : dOff+4]

				wantPix := src.Pix[sOff : sOff+4]
				for i := range 4 {
					if gotPix[i] != wantPix[i] {
						t.Errorf("pixel (%d,%d)[%d] = %d, want %d", x, y, i, gotPix[i], wantPix[i])
					}
				}
			}
		}

		for _, p := range [][2]int{{3, 0}, {0, 3}, {3, 3}} {
			off := dst.PixOffset(p[0], p[1])
			for i := range 4 {
				if dst.Pix[off+i] != 0 {
					t.Errorf("border pixel (%d,%d)[%d] = %d, want 0 (transparent)", p[0], p[1], i, dst.Pix[off+i])
				}
			}
		}
	})

	t.Run("translation shifts content and leaves the rest transparent", func(t *testing.T) {
		t.Parallel()

		src := newTestNRGBA(2, 2, func(_, _ int) [4]uint8 {
			return [4]uint8{200, 100, 50, 255}
		})
		// Forward mapping dst = src + (2, 1): h maps (x,y,1) -> (x+2, y+1, 1).
		translate := [3][3]float64{{1, 0, 2}, {0, 1, 1}, {0, 0, 1}}

		dst := WarpPerspective(src, translate, 5, 5)

		// Destination pixel (2,1) samples source (0,0), which is a full 2x2
		// neighborhood away from src's own bounds edge, so it should be
		// transparent (needs src (1,1) which is in bounds -> actually in bounds).
		off := dst.PixOffset(2, 1)

		want := [4]uint8{200, 100, 50, 255}
		for i := range 4 {
			if dst.Pix[off+i] != want[i] {
				t.Errorf("shifted pixel (2,1)[%d] = %d, want %d", i, dst.Pix[off+i], want[i])
			}
		}

		// A destination pixel far outside the translated source footprint stays
		// fully transparent.
		off = dst.PixOffset(4, 4)
		for i := range 4 {
			if dst.Pix[off+i] != 0 {
				t.Errorf("out-of-footprint pixel (4,4)[%d] = %d, want 0", i, dst.Pix[off+i])
			}
		}
	})
}

// referenceWarp is the original whole-canvas WarpPerspective loop, kept as an
// oracle that the tiled Warper.Render must reproduce byte for byte.
func referenceWarp(src *image.NRGBA, h [3][3]float64, targetW, targetH int) *image.NRGBA {
	hInv := invert3x3(h)
	dst := image.NewNRGBA(image.Rect(0, 0, targetW, targetH))

	for dy := range targetH {
		for dx := range targetW {
			x, y := float64(dx), float64(dy)

			denom := hInv[2][0]*x + hInv[2][1]*y + hInv[2][2]
			if denom == 0 {
				continue
			}

			sx := (hInv[0][0]*x + hInv[0][1]*y + hInv[0][2]) / denom
			sy := (hInv[1][0]*x + hInv[1][1]*y + hInv[1][2]) / denom

			r, g, b, a, ok := bilinearSample(src, sx, sy)
			if !ok {
				continue
			}

			off := dst.PixOffset(dx, dy)
			dst.Pix[off], dst.Pix[off+1], dst.Pix[off+2], dst.Pix[off+3] = r, g, b, a
		}
	}

	return dst
}

// gradientNRGBA returns a w x h image whose pixels all differ, with partial
// alpha, so interpolation and rounding errors show up in comparisons.
func gradientNRGBA(w, h int) *image.NRGBA {
	return newTestNRGBA(w, h, func(x, y int) [4]uint8 {
		return [4]uint8{uint8(x * 7), uint8(y * 11), uint8(x ^ y), uint8(128 + (x+y)%128)}
	})
}

// testHomography maps a 64x48 source onto a skewed, perspective-distorted quad
// inside a 300x250 canvas.
func testHomography() [3][3]float64 {
	return SolveHomography(
		[4][2]float64{{0, 0}, {63, 0}, {63, 47}, {0, 47}},
		[4][2]float64{{40.3, 30.7}, {250.1, 55.2}, {230.8, 210.4}, {20.5, 190.9}},
	)
}

func TestWarper_Render(t *testing.T) {
	t.Parallel()

	const (
		canvasW, canvasH = 300, 250
		tile             = 64
	)

	src := gradientNRGBA(64, 48)
	h := testHomography()
	want := referenceWarp(src, h, canvasW, canvasH)
	w := NewWarper(src, h)

	t.Run("tiles reassemble into the reference canvas", func(t *testing.T) {
		t.Parallel()

		got := image.NewNRGBA(image.Rect(0, 0, canvasW, canvasH))

		for ty := 0; ty < canvasH; ty += tile {
			for tx := 0; tx < canvasW; tx += tile {
				buf := image.NewNRGBA(image.Rect(0, 0, min(tile, canvasW-tx), min(tile, canvasH-ty)))
				w.Render(buf, tx, ty)

				for y := range buf.Bounds().Dy() {
					copy(got.Pix[got.PixOffset(tx, ty+y):], buf.Pix[buf.PixOffset(0, y):buf.PixOffset(buf.Bounds().Dx(), y)])
				}
			}
		}

		if !bytes.Equal(got.Pix, want.Pix) {
			t.Error("tiled Render output differs from the reference whole-canvas warp")
		}
	})

	t.Run("reports whether the tile has content", func(t *testing.T) {
		t.Parallel()

		inside := image.NewNRGBA(image.Rect(0, 0, tile, tile))
		if !w.Render(inside, 100, 100) {
			t.Error("Render() over the warped quad = false, want true")
		}

		outside := image.NewNRGBA(image.Rect(0, 0, tile, tile))
		if w.Render(outside, 0, canvasH) {
			t.Error("Render() below the warped quad = true, want false")
		}

		if !bytes.Equal(outside.Pix, make([]uint8, len(outside.Pix))) {
			t.Error("Render() wrote pixels into a tile outside the warped quad")
		}
	})
}

func TestWarper_Footprint(t *testing.T) {
	t.Parallel()

	t.Run("covers every pixel Render writes", func(t *testing.T) {
		t.Parallel()

		const canvasW, canvasH = 300, 250

		h := testHomography()
		canvas := referenceWarp(gradientNRGBA(64, 48), h, canvasW, canvasH)

		fp, ok := NewWarper(gradientNRGBA(64, 48), h).Footprint()
		if !ok {
			t.Fatal("Footprint() ok = false, want true for a well-behaved homography")
		}

		if fp.Dx() > canvasW || fp.Dy() > canvasH {
			t.Errorf("Footprint() = %v, want it tighter than the %dx%d canvas", fp, canvasW, canvasH)
		}

		for y := range canvasH {
			for x := range canvasW {
				off := canvas.PixOffset(x, y)
				if canvas.Pix[off+3] != 0 && !image.Pt(x, y).In(fp) {
					t.Fatalf("pixel (%d,%d) is written but outside Footprint() = %v", x, y, fp)
				}
			}
		}
	})

	t.Run("source too small to sample is empty", func(t *testing.T) {
		t.Parallel()

		identity := [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}

		fp, ok := NewWarper(gradientNRGBA(1, 5), identity).Footprint()
		if !ok || !fp.Empty() {
			t.Errorf("Footprint() = %v, %v, want an empty rectangle, true", fp, ok)
		}
	})

	t.Run("projection crossing the horizon is unbounded", func(t *testing.T) {
		t.Parallel()

		// The denominator x - 10 changes sign across the 20px-wide source.
		h := [3][3]float64{{1, 0, 0}, {0, 1, 0}, {1, 0, -10}}

		if _, ok := NewWarper(gradientNRGBA(20, 20), h).Footprint(); ok {
			t.Error("Footprint() ok = true, want false when the denominator changes sign")
		}
	})

	t.Run("huge projected corners are unbounded", func(t *testing.T) {
		t.Parallel()

		h := [3][3]float64{{math.MaxFloat32, 0, 0}, {0, 1, 0}, {0, 0, 1}}

		if _, ok := NewWarper(gradientNRGBA(20, 20), h).Footprint(); ok {
			t.Error("Footprint() ok = true, want false when corners overflow int range")
		}
	})
}

func TestBilinearSample(t *testing.T) {
	t.Parallel()

	// 2x2 image with distinct corner colors, all fully opaque.
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	setPixel := func(x, y int, r, g, b, a uint8) {
		off := src.PixOffset(x, y)
		src.Pix[off], src.Pix[off+1], src.Pix[off+2], src.Pix[off+3] = r, g, b, a
	}
	setPixel(0, 0, 0, 0, 0, 255)
	setPixel(1, 0, 100, 0, 0, 255)
	setPixel(0, 1, 0, 100, 0, 255)
	setPixel(1, 1, 100, 100, 0, 255)

	t.Run("exact corner sample", func(t *testing.T) {
		t.Parallel()

		r, g, b, a, ok := bilinearSample(src, 0, 0)
		if !ok {
			t.Fatal("bilinearSample(0,0) ok = false, want true")
		}

		if r != 0 || g != 0 || b != 0 || a != 255 {
			t.Errorf("bilinearSample(0,0) = (%d,%d,%d,%d), want (0,0,0,255)", r, g, b, a)
		}
	})

	t.Run("center sample averages all four corners", func(t *testing.T) {
		t.Parallel()

		r, g, b, a, ok := bilinearSample(src, 0.5, 0.5)
		if !ok {
			t.Fatal("bilinearSample(0.5,0.5) ok = false, want true")
		}

		if r != 50 || g != 50 || b != 0 || a != 255 {
			t.Errorf("bilinearSample(0.5,0.5) = (%d,%d,%d,%d), want (50,50,0,255)", r, g, b, a)
		}
	})

	t.Run("out of bounds below zero", func(t *testing.T) {
		t.Parallel()

		_, _, _, _, ok := bilinearSample(src, -0.5, 0)
		if ok {
			t.Error("bilinearSample(-0.5,0) ok = true, want false")
		}
	})

	t.Run("out of bounds requires neighbor past far edge", func(t *testing.T) {
		t.Parallel()

		_, _, _, _, ok := bilinearSample(src, 1, 1) // needs (2,2), outside 2x2 image
		if ok {
			t.Error("bilinearSample(1,1) ok = true, want false")
		}
	})
}

func BenchmarkWarper_Render(b *testing.B) {
	src := gradientNRGBA(2000, 1600)
	h := SolveHomography(
		[4][2]float64{{0, 0}, {1999, 0}, {1999, 1599}, {0, 1599}},
		[4][2]float64{{10, 20}, {1000, 60}, {980, 900}, {0, 870}},
	)
	w := NewWarper(src, h)
	tile := image.NewNRGBA(image.Rect(0, 0, 256, 256))

	for b.Loop() {
		clear(tile.Pix)
		w.Render(tile, 256, 256)
	}
}
