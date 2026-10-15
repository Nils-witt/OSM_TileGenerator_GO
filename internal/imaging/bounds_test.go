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

func TestRowAllZero(t *testing.T) {
	t.Parallel()

	img := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	// Row 1 has a non-zero pixel; rows 0 and 2 are all zero.
	off := img.PixOffset(0, 1)
	img.Pix[off] = 255

	tests := []struct {
		name   string
		y      int
		height int
		want   bool
	}{
		{"row 0 is zero", 0, 3, true},
		{"row 1 is not zero", 1, 3, false},
		{"row 2 is zero", 2, 3, true},
		{"negative wraps to row 1 (-2 mod 3 = 1)", -2, 3, false},
		{"negative wraps to row 0 (-3 mod 3 = 0)", -3, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := rowAllZero(img, tt.y, tt.height); got != tt.want {
				t.Errorf("rowAllZero(img, %d, %d) = %v, want %v", tt.y, tt.height, got, tt.want)
			}
		})
	}
}

func TestColAllZero(t *testing.T) {
	t.Parallel()

	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	// Column 1 has a non-zero pixel; columns 0 and 2 are all zero.
	off := img.PixOffset(1, 0)
	img.Pix[off] = 255

	tests := []struct {
		name  string
		x     int
		width int
		want  bool
	}{
		{"col 0 is zero", 0, 3, true},
		{"col 1 is not zero", 1, 3, false},
		{"col 2 is zero", 2, 3, true},
		{"negative wraps to col 1 (-2 mod 3 = 1)", -2, 3, false},
		{"negative wraps to col 0 (-3 mod 3 = 0)", -3, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := colAllZero(img, tt.x, tt.width); got != tt.want {
				t.Errorf("colAllZero(img, %d, %d) = %v, want %v", tt.x, tt.width, got, tt.want)
			}
		})
	}
}

// FindYBounds/FindXBounds only sample rows/columns at TileSize-aligned steps
// from the far edge (height-1, height-1-TileSize, ...), so test images are
// built as a multiple of TileSize and content is placed exactly on or off
// those sampled lines.
func TestFindYBounds(t *testing.T) {
	t.Parallel()

	const height = TileSize * 3 // sampled rows: 767, 511, 255

	tests := []struct {
		name       string
		contentRow int // -1 means fully empty image
		want       int
	}{
		{"content on last sampled row stops immediately", height - 1, height - 1 + TileSize*2},
		{"content off the sampled rows exhausts the search", 0, -1 + TileSize*2},
		{"fully empty image exhausts the search", -1, -1 + TileSize*2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			img := image.NewNRGBA(image.Rect(0, 0, 1, height))
			if tt.contentRow >= 0 {
				off := img.PixOffset(0, tt.contentRow)
				img.Pix[off] = 255
			}

			if got := FindYBounds(img); got != tt.want {
				t.Errorf("FindYBounds() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFindXBounds(t *testing.T) {
	t.Parallel()

	const width = TileSize * 3 // sampled columns: 767, 511, 255

	tests := []struct {
		name       string
		contentCol int // -1 means fully empty image
		want       int
	}{
		{"content on last sampled column stops immediately", width - 1, width - 1 + TileSize*2},
		{"content off the sampled columns exhausts the search", 0, -1 + TileSize*2},
		{"fully empty image exhausts the search", -1, -1 + TileSize*2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			img := image.NewNRGBA(image.Rect(0, 0, width, 1))
			if tt.contentCol >= 0 {
				off := img.PixOffset(tt.contentCol, 0)
				img.Pix[off] = 255
			}

			if got := FindXBounds(img); got != tt.want {
				t.Errorf("FindXBounds() = %d, want %d", got, tt.want)
			}
		})
	}
}
