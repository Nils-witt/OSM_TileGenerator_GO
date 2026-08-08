// Package geometry provides the 4-point homography solver and perspective
// warping used to project the source overlay image onto the OSM tile grid.
package geometry

import (
	"fmt"
	"math"
)

// SolveHomography computes the 3x3 projective homography matrix H mapping each
// src[i] point to the corresponding dst[i] point, for exactly 4 point
// correspondences. This is the direct linear transform (DLT) with h33 fixed to 1,
// equivalent to cv2.findHomography's exact solution for 4 non-degenerate points
// (no RANSAC/outlier rejection needed since the system is exactly determined).
func SolveHomography(src, dst [4][2]float64) [3][3]float64 {
	var a [8][8]float64
	var b [8]float64

	for i := 0; i < 4; i++ {
		x, y := src[i][0], src[i][1]
		u, v := dst[i][0], dst[i][1]

		r0 := 2 * i
		r1 := 2*i + 1

		a[r0] = [8]float64{x, y, 1, 0, 0, 0, -x * u, -y * u}
		b[r0] = u

		a[r1] = [8]float64{0, 0, 0, x, y, 1, -x * v, -y * v}
		b[r1] = v
	}

	h := gaussianSolve8(a, b)

	return [3][3]float64{
		{h[0], h[1], h[2]},
		{h[3], h[4], h[5]},
		{h[6], h[7], 1},
	}
}

// gaussianSolve8 solves the linear system A*x = b for an 8x8 system using
// Gaussian elimination with partial pivoting.
func gaussianSolve8(a [8][8]float64, b [8]float64) [8]float64 {
	const n = 8

	for col := 0; col < n; col++ {
		pivot := col
		maxVal := math.Abs(a[col][col])
		for row := col + 1; row < n; row++ {
			if v := math.Abs(a[row][col]); v > maxVal {
				pivot = row
				maxVal = v
			}
		}
		if maxVal == 0 {
			panic(fmt.Sprintf("homography: singular matrix at column %d", col))
		}
		if pivot != col {
			a[col], a[pivot] = a[pivot], a[col]
			b[col], b[pivot] = b[pivot], b[col]
		}

		for row := col + 1; row < n; row++ {
			factor := a[row][col] / a[col][col]
			if factor == 0 {
				continue
			}
			for k := col; k < n; k++ {
				a[row][k] -= factor * a[col][k]
			}
			b[row] -= factor * b[col]
		}
	}

	var x [8]float64
	for row := n - 1; row >= 0; row-- {
		sum := b[row]
		for k := row + 1; k < n; k++ {
			sum -= a[row][k] * x[k]
		}
		x[row] = sum / a[row][row]
	}
	return x
}

// invert3x3 computes the inverse of a 3x3 matrix via the adjugate/determinant formula.
func invert3x3(m [3][3]float64) [3][3]float64 {
	det := m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])

	if det == 0 {
		panic("homography: cannot invert singular matrix")
	}
	invDet := 1.0 / det

	var inv [3][3]float64
	inv[0][0] = (m[1][1]*m[2][2] - m[1][2]*m[2][1]) * invDet
	inv[0][1] = (m[0][2]*m[2][1] - m[0][1]*m[2][2]) * invDet
	inv[0][2] = (m[0][1]*m[1][2] - m[0][2]*m[1][1]) * invDet
	inv[1][0] = (m[1][2]*m[2][0] - m[1][0]*m[2][2]) * invDet
	inv[1][1] = (m[0][0]*m[2][2] - m[0][2]*m[2][0]) * invDet
	inv[1][2] = (m[0][2]*m[1][0] - m[0][0]*m[1][2]) * invDet
	inv[2][0] = (m[1][0]*m[2][1] - m[1][1]*m[2][0]) * invDet
	inv[2][1] = (m[0][1]*m[2][0] - m[0][0]*m[2][1]) * invDet
	inv[2][2] = (m[0][0]*m[1][1] - m[0][1]*m[1][0]) * invDet

	return inv
}
