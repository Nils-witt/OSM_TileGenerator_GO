package slippy

import (
	"math"
	"testing"
)

func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func TestDeg2Num(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		latDeg, lonDeg float64
		zoom           int
		wantX, wantY   float64
	}{
		{
			name:   "origin at zoom 0",
			latDeg: 0, lonDeg: 0, zoom: 0,
			wantX: 0.5, wantY: 0.5,
		},
		{
			name:   "top-left corner at zoom 0",
			latDeg: 85.0511287798, lonDeg: -180, zoom: 0,
			wantX: 0, wantY: 0,
		},
		{
			name:   "bottom-right corner at zoom 0",
			latDeg: -85.0511287798, lonDeg: 180, zoom: 0,
			wantX: 1, wantY: 1,
		},
		{
			name:   "origin at zoom 4 lands at tile grid center",
			latDeg: 0, lonDeg: 0, zoom: 4,
			wantX: 8, wantY: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotX, gotY := Deg2Num(tt.latDeg, tt.lonDeg, tt.zoom)
			if !approxEqual(gotX, tt.wantX, 1e-6) || !approxEqual(gotY, tt.wantY, 1e-6) {
				t.Errorf("Deg2Num(%v, %v, %d) = (%v, %v), want (%v, %v)",
					tt.latDeg, tt.lonDeg, tt.zoom, gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestNum2Deg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		xTile, yTile     float64
		zoom             int
		wantLat, wantLon float64
	}{
		{
			name:  "center tile at zoom 0",
			xTile: 0.5, yTile: 0.5, zoom: 0,
			wantLat: 0, wantLon: 0,
		},
		{
			name:  "origin tile at zoom 0",
			xTile: 0, yTile: 0, zoom: 0,
			wantLat: 85.0511287798, wantLon: -180,
		},
		{
			name:  "far corner at zoom 0",
			xTile: 1, yTile: 1, zoom: 0,
			wantLat: -85.0511287798, wantLon: 180,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotLat, gotLon := Num2Deg(tt.xTile, tt.yTile, tt.zoom)
			if !approxEqual(gotLat, tt.wantLat, 1e-6) || !approxEqual(gotLon, tt.wantLon, 1e-6) {
				t.Errorf("Num2Deg(%v, %v, %d) = (%v, %v), want (%v, %v)",
					tt.xTile, tt.yTile, tt.zoom, gotLat, gotLon, tt.wantLat, tt.wantLon)
			}
		})
	}
}

func TestDeg2NumNum2DegRoundTrip(t *testing.T) {
	t.Parallel()

	points := []struct {
		lat, lon float64
	}{
		{0, 0},
		{51.5074, -0.1278},   // London
		{40.7128, -74.0060},  // New York
		{-33.8688, 151.2093}, // Sydney
		{64.1466, -21.9426},  // Reykjavik
	}

	for _, p := range points {
		for zoom := 0; zoom <= 18; zoom += 3 {
			x, y := Deg2Num(p.lat, p.lon, zoom)

			gotLat, gotLon := Num2Deg(x, y, zoom)
			if !approxEqual(gotLat, p.lat, 1e-6) || !approxEqual(gotLon, p.lon, 1e-6) {
				t.Errorf("round trip at zoom %d for (%v, %v): got (%v, %v)",
					zoom, p.lat, p.lon, gotLat, gotLon)
			}
		}
	}
}
