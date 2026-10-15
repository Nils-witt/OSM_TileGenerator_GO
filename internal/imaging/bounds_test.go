package imaging

import (
	"image"
	"testing"
)

func TestEmpty(t *testing.T) {
	t.Parallel()

	t.Run("all zero bytes", func(t *testing.T) {
		t.Parallel()

		img := image.NewNRGBA(image.Rect(0, 0, 3, 3))
		if !Empty(img) {
			t.Error("Empty() = false, want true for all-zero image")
		}
	})

	t.Run("single non-zero byte", func(t *testing.T) {
		t.Parallel()

		img := image.NewNRGBA(image.Rect(0, 0, 3, 3))

		img.Pix[len(img.Pix)-1] = 1 // last byte, e.g. alpha of the last pixel
		if Empty(img) {
			t.Error("Empty() = true, want false when one byte is non-zero")
		}
	})
}
