package geometry

import (
	"image"
	"testing"
)

func newTestNRGBA(w, h int, fill func(x, y int) [4]uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
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
		for y := 0; y < 3; y++ {
			for x := 0; x < 3; x++ {
				sOff := src.PixOffset(x, y)
				dOff := dst.PixOffset(x, y)
				gotPix := dst.Pix[dOff : dOff+4]
				wantPix := src.Pix[sOff : sOff+4]
				for i := 0; i < 4; i++ {
					if gotPix[i] != wantPix[i] {
						t.Errorf("pixel (%d,%d)[%d] = %d, want %d", x, y, i, gotPix[i], wantPix[i])
					}
				}
			}
		}

		for _, p := range [][2]int{{3, 0}, {0, 3}, {3, 3}} {
			off := dst.PixOffset(p[0], p[1])
			for i := 0; i < 4; i++ {
				if dst.Pix[off+i] != 0 {
					t.Errorf("border pixel (%d,%d)[%d] = %d, want 0 (transparent)", p[0], p[1], i, dst.Pix[off+i])
				}
			}
		}
	})

	t.Run("translation shifts content and leaves the rest transparent", func(t *testing.T) {
		t.Parallel()
		src := newTestNRGBA(2, 2, func(x, y int) [4]uint8 {
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
		for i := 0; i < 4; i++ {
			if dst.Pix[off+i] != want[i] {
				t.Errorf("shifted pixel (2,1)[%d] = %d, want %d", i, dst.Pix[off+i], want[i])
			}
		}

		// A destination pixel far outside the translated source footprint stays
		// fully transparent.
		off = dst.PixOffset(4, 4)
		for i := 0; i < 4; i++ {
			if dst.Pix[off+i] != 0 {
				t.Errorf("out-of-footprint pixel (4,4)[%d] = %d, want 0", i, dst.Pix[off+i])
			}
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
