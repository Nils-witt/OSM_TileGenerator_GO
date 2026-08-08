package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		const data = `{
			"id": "proj-1",
			"name": "Test Project",
			"description": "a project",
			"min_zoom": 14,
			"max_zoom": 18,
			"img_scale": {"width": 800, "height": 600},
			"coordinates": [
				{"img": {"x": 1, "y": 2}, "map": {"latitude": 51.5, "longitude": -0.1}}
			]
		}`
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(data), 0o644); err != nil {
			t.Fatalf("writing fixture config: %v", err)
		}

		got, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v, want nil", err)
		}

		want := &Project{
			ID:          "proj-1",
			Name:        "Test Project",
			Description: "a project",
			MinZoom:     14,
			MaxZoom:     18,
			ImgScale:    ImgScale{Width: 800, Height: 600},
			Coordinates: []CoordinatePair{
				{
					Img: ImgCoordinate{X: 1, Y: 2},
					Map: MapCoordinate{Latitude: 51.5, Longitude: -0.1},
				},
			},
		}

		if got.ID != want.ID || got.Name != want.Name || got.Description != want.Description ||
			got.MinZoom != want.MinZoom || got.MaxZoom != want.MaxZoom || got.ImgScale != want.ImgScale {
			t.Errorf("LoadConfig() = %+v, want %+v", got, want)
		}
		if len(got.Coordinates) != len(want.Coordinates) || got.Coordinates[0] != want.Coordinates[0] {
			t.Errorf("LoadConfig() Coordinates = %+v, want %+v", got.Coordinates, want.Coordinates)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		_, err := LoadConfig(dir)
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want error for missing config.json")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatalf("writing fixture config: %v", err)
		}

		_, err := LoadConfig(dir)
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want error for malformed JSON")
		}
	})
}
