package imaging

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNRGBASavePNGRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.png")

	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))

	for y := range 2 {
		for x := range 3 {
			off := src.PixOffset(x, y)
			src.Pix[off] = uint8(x * 40)
			src.Pix[off+1] = uint8(y * 40)
			src.Pix[off+2] = 10
			src.Pix[off+3] = 255
		}
	}

	if err := SavePNG(path, src); err != nil {
		t.Fatalf("SavePNG() error = %v", err)
	}

	got, err := LoadNRGBA(path)
	if err != nil {
		t.Fatalf("LoadNRGBA() error = %v", err)
	}

	if got.Bounds().Dx() != 3 || got.Bounds().Dy() != 2 {
		t.Fatalf("LoadNRGBA() size = %v, want 3x2", got.Bounds())
	}

	for y := range 2 {
		for x := range 3 {
			wantOff := src.PixOffset(x, y)
			gotOff := got.PixOffset(x, y)
			want := src.Pix[wantOff : wantOff+4]

			gotPix := got.Pix[gotOff : gotOff+4]
			for i := range 4 {
				if gotPix[i] != want[i] {
					t.Errorf("pixel (%d,%d)[%d] = %d, want %d", x, y, i, gotPix[i], want[i])
				}
			}
		}
	}
}

func TestLoadNRGBAConvertsNonNRGBASource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "paletted.png")

	pal := color.Palette{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 0, 0, 255},
	}
	src := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	src.SetColorIndex(0, 0, 0)
	src.SetColorIndex(1, 0, 1)
	src.SetColorIndex(0, 1, 1)
	src.SetColorIndex(1, 1, 0)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating fixture file: %v", err)
	}

	if err := png.Encode(f, src); err != nil {
		_ = f.Close()

		t.Fatalf("encoding paletted PNG: %v", err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("closing fixture file: %v", err)
	}

	got, err := LoadNRGBA(path)
	if err != nil {
		t.Fatalf("LoadNRGBA() error = %v", err)
	}

	if r, g, b, a := got.NRGBAAt(1, 0).R, got.NRGBAAt(1, 0).G, got.NRGBAAt(1, 0).B, got.NRGBAAt(1, 0).A; r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("pixel (1,0) = (%d,%d,%d,%d), want (255,0,0,255)", r, g, b, a)
	}

	if r, g, b, a := got.NRGBAAt(0, 0).R, got.NRGBAAt(0, 0).G, got.NRGBAAt(0, 0).B, got.NRGBAAt(0, 0).A; r != 0 || g != 0 || b != 0 || a != 255 {
		t.Errorf("pixel (0,0) = (%d,%d,%d,%d), want (0,0,0,255)", r, g, b, a)
	}
}

func TestLoadNRGBAMissingFile(t *testing.T) {
	t.Parallel()

	_, err := LoadNRGBA(filepath.Join(t.TempDir(), "missing.png"))
	if err == nil {
		t.Fatal("LoadNRGBA() error = nil, want error for missing file")
	}
}

func TestFileExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	path := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	if !FileExists(path) {
		t.Error("FileExists() = false, want true for a file that exists")
	}

	if FileExists(filepath.Join(dir, "missing.txt")) {
		t.Error("FileExists() = true, want false for a file that does not exist")
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()

	t.Run("valid png", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "sized.png")

		img := image.NewNRGBA(image.Rect(0, 0, 7, 5))
		if err := SavePNG(path, img); err != nil {
			t.Fatalf("SavePNG() error = %v", err)
		}

		w, h, err := Dimensions(path)
		if err != nil {
			t.Fatalf("Dimensions() error = %v", err)
		}

		if w != 7 || h != 5 {
			t.Errorf("Dimensions() = (%d, %d), want (7, 5)", w, h)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		_, _, err := Dimensions(filepath.Join(t.TempDir(), "missing.png"))
		if err == nil {
			t.Fatal("Dimensions() error = nil, want error for missing file")
		}
	})

	t.Run("not a png", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		path := filepath.Join(dir, "not-a-png.png")
		if err := os.WriteFile(path, []byte("not a png"), 0o644); err != nil {
			t.Fatalf("writing fixture file: %v", err)
		}

		_, _, err := Dimensions(path)
		if err == nil {
			t.Fatal("Dimensions() error = nil, want error for non-PNG content")
		}
	})
}
