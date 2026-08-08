package imaging

import (
	"image"

	xdraw "golang.org/x/image/draw"
)

// blit copies src's pixel bytes directly into dst at dst-space offset (x0, y0),
// clipping to dst's bounds. It fully overwrites dst including alpha (no alpha
// compositing), mirroring PIL's Image.paste(src, (x, y)) without a mask.
func blit(dst, src *image.NRGBA, x0, y0 int) {
	srcB := src.Bounds()
	dstB := dst.Bounds()

	for y := 0; y < srcB.Dy(); y++ {
		dy := y0 + y
		if dy < dstB.Min.Y || dy >= dstB.Max.Y {
			continue
		}
		rowW := srcB.Dx()
		dxStart := 0
		if x0 < dstB.Min.X {
			dxStart = dstB.Min.X - x0
		}
		dxEnd := rowW
		if x0+rowW > dstB.Max.X {
			dxEnd = dstB.Max.X - x0
		}
		if dxEnd <= dxStart {
			continue
		}
		srcOff := src.PixOffset(srcB.Min.X+dxStart, srcB.Min.Y+y)
		dstOff := dst.PixOffset(x0+dxStart, dy)
		n := (dxEnd - dxStart) * 4
		copy(dst.Pix[dstOff:dstOff+n], src.Pix[srcOff:srcOff+n])
	}
}

// Blit copies src's pixels into dst at dst-space offset (x0, y0), clipping to
// dst's bounds and fully overwriting dst including alpha.
func Blit(dst, src *image.NRGBA, x0, y0 int) {
	blit(dst, src, x0, y0)
}

// CropRegion extracts the region [x0,y0)-[x1,y1) from img, top-left anchored at
// (0,0) in the result. Mirrors PIL's Image.crop: parts of the requested region
// that fall outside img's bounds are filled with zero bytes (fully transparent),
// rather than erroring or clamping.
func CropRegion(img *image.NRGBA, x0, y0, x1, y1 int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	blit(out, img, -x0, -y0)
	return out
}

// Scale resizes img to w x h using a Catmull-Rom (bicubic-family) filter,
// standing in for PIL's default bicubic Image.resize().
func Scale(img *image.NRGBA, w, h int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return dst
}
