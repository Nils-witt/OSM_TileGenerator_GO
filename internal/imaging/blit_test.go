package imaging

import (
	"image"
	"testing"
)

func solidNRGBA(w, h int, r, g, b, a uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, b, a
	}

	return img
}

func pixelAt(img *image.NRGBA, x, y int) [4]uint8 {
	off := img.PixOffset(x, y)
	return [4]uint8{img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3]}
}

func TestBlit(t *testing.T) {
	t.Parallel()

	t.Run("fully in bounds overwrites including alpha", func(t *testing.T) {
		t.Parallel()

		dst := solidNRGBA(4, 4, 10, 10, 10, 255)
		src := solidNRGBA(2, 2, 200, 100, 50, 0)

		Blit(dst, src, 1, 1)

		for y := 1; y < 3; y++ {
			for x := 1; x < 3; x++ {
				got := pixelAt(dst, x, y)

				want := [4]uint8{200, 100, 50, 0}
				if got != want {
					t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
				}
			}
		}
		// Untouched region keeps the original dst pixels.
		if got := pixelAt(dst, 0, 0); got != [4]uint8{10, 10, 10, 255} {
			t.Errorf("pixel (0,0) = %v, want unchanged {10,10,10,255}", got)
		}
	})

	t.Run("clips negative offset", func(t *testing.T) {
		t.Parallel()

		dst := solidNRGBA(4, 4, 0, 0, 0, 0)
		src := solidNRGBA(3, 3, 255, 255, 255, 255)

		Blit(dst, src, -1, -1)

		// Only the bottom-right 2x2 of src should land, at dst (0,0)-(1,1).
		for y := range 2 {
			for x := range 2 {
				if got := pixelAt(dst, x, y); got != [4]uint8{255, 255, 255, 255} {
					t.Errorf("pixel (%d,%d) = %v, want opaque white", x, y, got)
				}
			}
		}

		if got := pixelAt(dst, 2, 2); got != [4]uint8{0, 0, 0, 0} {
			t.Errorf("pixel (2,2) = %v, want untouched transparent", got)
		}
	})

	t.Run("clips beyond far edge", func(t *testing.T) {
		t.Parallel()

		dst := solidNRGBA(4, 4, 0, 0, 0, 0)
		src := solidNRGBA(3, 3, 255, 0, 0, 255)

		Blit(dst, src, 3, 3)

		if got := pixelAt(dst, 3, 3); got != [4]uint8{255, 0, 0, 255} {
			t.Errorf("pixel (3,3) = %v, want opaque red", got)
		}
	})

	t.Run("entirely outside bounds is a no-op", func(t *testing.T) {
		t.Parallel()

		dst := solidNRGBA(4, 4, 0, 0, 0, 0)
		src := solidNRGBA(2, 2, 255, 255, 255, 255)

		Blit(dst, src, 10, 10)

		for y := range 4 {
			for x := range 4 {
				if got := pixelAt(dst, x, y); got != [4]uint8{0, 0, 0, 0} {
					t.Errorf("pixel (%d,%d) = %v, want untouched transparent", x, y, got)
				}
			}
		}
	})
}

func TestScale(t *testing.T) {
	t.Parallel()

	t.Run("resizes to requested dimensions", func(t *testing.T) {
		t.Parallel()

		src := solidNRGBA(8, 8, 50, 60, 70, 255)

		got := Scale(src, 4, 2)

		if got.Bounds().Dx() != 4 || got.Bounds().Dy() != 2 {
			t.Fatalf("Scale size = %v, want 4x2", got.Bounds())
		}
	})

	t.Run("uniform color image stays uniform after scaling", func(t *testing.T) {
		t.Parallel()

		src := solidNRGBA(8, 8, 50, 60, 70, 255)

		got := Scale(src, 3, 3)

		for y := range 3 {
			for x := range 3 {
				p := pixelAt(got, x, y)

				want := [4]uint8{50, 60, 70, 255}
				for i := range p {
					diff := int(p[i]) - int(want[i])
					if diff < -1 || diff > 1 {
						t.Errorf("pixel (%d,%d)[%d] = %d, want ~%d", x, y, i, p[i], want[i])
					}
				}
			}
		}
	})
}
