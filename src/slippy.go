package main

import "math"

// deg2num converts latitude/longitude to fractional OSM tile numbers at a given zoom level.
// Credits: https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames#Go
func deg2num(latDeg, lonDeg float64, zoom int) (float64, float64) {
	latRad := latDeg * math.Pi / 180.0
	n := math.Exp2(float64(zoom))
	xTile := (lonDeg + 180.0) / 360.0 * n
	yTile := (1.0 - math.Asinh(math.Tan(latRad))/math.Pi) / 2.0 * n
	return xTile, yTile
}

// num2deg converts OSM tile numbers to latitude/longitude at a given zoom level.
// Credits: https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames#Go
func num2deg(xTile, yTile float64, zoom int) (float64, float64) {
	n := math.Exp2(float64(zoom))
	lonDeg := xTile/n*360.0 - 180.0
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*yTile/n)))
	latDeg := latRad * 180.0 / math.Pi
	return latDeg, lonDeg
}
