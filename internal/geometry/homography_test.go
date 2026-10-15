package geometry

import (
	"math"
	"testing"
)

func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

// applyHomography maps point (x, y) through h using the standard projective
// transform, mirroring what WarpPerspective does with the forward matrix.
func applyHomography(h [3][3]float64, x, y float64) (float64, float64) {
	denom := h[2][0]*x + h[2][1]*y + h[2][2]
	u := (h[0][0]*x + h[0][1]*y + h[0][2]) / denom
	v := (h[1][0]*x + h[1][1]*y + h[1][2]) / denom

	return u, v
}

func TestSolveHomography(t *testing.T) {
	t.Parallel()

	t.Run("identity mapping", func(t *testing.T) {
		t.Parallel()

		pts := [4][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
		h := SolveHomography(pts, pts)

		for _, p := range pts {
			u, v := applyHomography(h, p[0], p[1])
			if !approxEqual(u, p[0], 1e-6) || !approxEqual(v, p[1], 1e-6) {
				t.Errorf("identity mapping of (%v, %v) = (%v, %v), want (%v, %v)", p[0], p[1], u, v, p[0], p[1])
			}
		}
	})

	t.Run("translation", func(t *testing.T) {
		t.Parallel()

		src := [4][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
		dst := [4][2]float64{{5, 7}, {15, 7}, {15, 17}, {5, 17}}
		h := SolveHomography(src, dst)

		for i, p := range src {
			u, v := applyHomography(h, p[0], p[1])
			if !approxEqual(u, dst[i][0], 1e-6) || !approxEqual(v, dst[i][1], 1e-6) {
				t.Errorf("translation mapping of (%v, %v) = (%v, %v), want (%v, %v)", p[0], p[1], u, v, dst[i][0], dst[i][1])
			}
		}
	})

	t.Run("scale and translate", func(t *testing.T) {
		t.Parallel()

		src := [4][2]float64{{0, 0}, {100, 0}, {100, 50}, {0, 50}}
		dst := [4][2]float64{{20, 20}, {220, 20}, {220, 120}, {20, 120}}
		h := SolveHomography(src, dst)

		for i, p := range src {
			u, v := applyHomography(h, p[0], p[1])
			if !approxEqual(u, dst[i][0], 1e-6) || !approxEqual(v, dst[i][1], 1e-6) {
				t.Errorf("scale/translate mapping of (%v, %v) = (%v, %v), want (%v, %v)", p[0], p[1], u, v, dst[i][0], dst[i][1])
			}
		}
	})

	t.Run("general projective quadrilateral", func(t *testing.T) {
		t.Parallel()

		src := [4][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
		// A non-parallelogram destination forces a genuinely projective (non-affine) solve.
		dst := [4][2]float64{{0, 0}, {20, 5}, {15, 25}, {2, 18}}
		h := SolveHomography(src, dst)

		for i, p := range src {
			u, v := applyHomography(h, p[0], p[1])
			if !approxEqual(u, dst[i][0], 1e-4) || !approxEqual(v, dst[i][1], 1e-4) {
				t.Errorf("projective mapping of (%v, %v) = (%v, %v), want (%v, %v)", p[0], p[1], u, v, dst[i][0], dst[i][1])
			}
		}
	})

	t.Run("h33 fixed to 1", func(t *testing.T) {
		t.Parallel()

		src := [4][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
		dst := [4][2]float64{{5, 7}, {15, 7}, {15, 17}, {5, 17}}

		h := SolveHomography(src, dst)
		if h[2][2] != 1 {
			t.Errorf("h[2][2] = %v, want 1", h[2][2])
		}
	})

	t.Run("collinear source points panics", func(t *testing.T) {
		t.Parallel()

		src := [4][2]float64{{0, 0}, {1, 0}, {2, 0}, {3, 0}} // all on one line: singular system
		dst := [4][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("SolveHomography() with collinear source points did not panic")
			}
		}()

		SolveHomography(src, dst)
	})
}

func TestInvert3x3(t *testing.T) {
	t.Parallel()

	t.Run("identity", func(t *testing.T) {
		t.Parallel()

		id := [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}

		got := invert3x3(id)
		if got != id {
			t.Errorf("invert3x3(identity) = %v, want %v", got, id)
		}
	})

	t.Run("known matrix", func(t *testing.T) {
		t.Parallel()

		m := [3][3]float64{
			{2, 0, 0},
			{0, 4, 0},
			{0, 0, 1},
		}
		want := [3][3]float64{
			{0.5, 0, 0},
			{0, 0.25, 0},
			{0, 0, 1},
		}
		got := invert3x3(m)

		for r := range 3 {
			for c := range 3 {
				if !approxEqual(got[r][c], want[r][c], 1e-9) {
					t.Errorf("invert3x3(%v)[%d][%d] = %v, want %v", m, r, c, got[r][c], want[r][c])
				}
			}
		}
	})

	t.Run("round trip through SolveHomography", func(t *testing.T) {
		t.Parallel()

		src := [4][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
		dst := [4][2]float64{{5, 7}, {23, 4}, {18, 21}, {1, 15}}
		h := SolveHomography(src, dst)
		inv := invert3x3(h)

		for _, p := range dst {
			// Mapping a dst point through h^-1 should land back near the
			// corresponding src point (verified via forward+inverse round trip).
			u, v := applyHomography(inv, p[0], p[1])

			var matched bool

			for _, s := range src {
				if approxEqual(u, s[0], 1e-3) && approxEqual(v, s[1], 1e-3) {
					matched = true
					break
				}
			}

			if !matched {
				t.Errorf("invert3x3 round trip: (%v,%v) -> (%v,%v) matched no src point", p[0], p[1], u, v)
			}
		}
	})

	t.Run("singular matrix panics", func(t *testing.T) {
		t.Parallel()

		m := [3][3]float64{{1, 2, 3}, {2, 4, 6}, {1, 1, 1}} // row 2 = 2 * row 1

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("invert3x3() with singular matrix did not panic")
			}
		}()

		invert3x3(m)
	})
}
