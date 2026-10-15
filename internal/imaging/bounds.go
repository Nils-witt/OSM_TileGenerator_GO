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
