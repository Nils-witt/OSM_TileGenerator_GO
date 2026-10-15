package imaging

import "image"

// Empty reports whether every byte (R,G,B,A of every pixel) in img is zero.
// Mirrors utils.py::tile_empty (`not np.asarray(tile).any()`).
func Empty(img *image.NRGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		off := img.PixOffset(b.Min.X, y)

		row := img.Pix[off : off+4*b.Dx()]
		for _, v := range row {
			if v != 0 {
				return false
			}
		}
	}

	return true
}

// rowAllZero reports whether row y (wrapped like a numpy negative index, modulo height)
// is entirely zero bytes.
func rowAllZero(img *image.NRGBA, y, height int) bool {
	wy := ((y % height) + height) % height
	off := img.PixOffset(img.Bounds().Min.X, img.Bounds().Min.Y+wy)

	row := img.Pix[off : off+4*img.Bounds().Dx()]
	for _, v := range row {
		if v != 0 {
			return false
		}
	}

	return true
}

// colAllZero reports whether column x (wrapped like a numpy negative index, modulo width)
// is entirely zero bytes.
func colAllZero(img *image.NRGBA, x, width int) bool {
	wx := ((x % width) + width) % width

	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		off := img.PixOffset(b.Min.X+wx, y)

		px := img.Pix[off : off+4]
		for _, v := range px {
			if v != 0 {
				return false
			}
		}
	}

	return true
}

// FindYBounds finds the lower y-bound of non-transparent pixels in the image.
// Mirrors utils.py::find_y_bounds, including numpy's negative-index wraparound
// semantics for the (rare) case y goes negative before content is found.
func FindYBounds(img *image.NRGBA) int {
	height := img.Bounds().Dy()

	y := height - 1
	for y > 0 && rowAllZero(img, y, height) {
		y -= TileSize
	}

	return y + TileSize*2
}

// FindXBounds finds the right x-bound of non-transparent pixels in the image.
// Mirrors utils.py::find_x_bounds.
func FindXBounds(img *image.NRGBA) int {
	width := img.Bounds().Dx()

	x := width - 1
	for x > 0 && colAllZero(img, x, width) {
		x -= TileSize
	}

	return x + TileSize*2
}
