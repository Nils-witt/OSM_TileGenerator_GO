package config

import (
	"encoding/json"
	"os"
)

// ImgCoordinate is a pixel coordinate within the source image.
type ImgCoordinate struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// MapCoordinate is a WGS84 lat/lng coordinate.
type MapCoordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// CoordinatePair correlates a pixel position in the source image with a map position.
type CoordinatePair struct {
	Img ImgCoordinate `json:"img"`
	Map MapCoordinate `json:"map"`
}

// ImgScale is the CSS/displayed pixel size of the image element the webclient used
// when the user placed markers, needed to rescale marker pixel coords to the source
// image's true pixel dimensions.
type ImgScale struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Project mirrors webclient/src/data/entities/Project.ts's IProject shape as written
// into config.json. Only Coordinates and ImgScale are consumed by the tile generator,
// matching what main.py itself reads.
type Project struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	MinZoom     int              `json:"min_zoom"`
	MaxZoom     int              `json:"max_zoom"`
	ImgScale    ImgScale         `json:"img_scale"`
	Coordinates []CoordinatePair `json:"coordinates"`
}

// LoadConfig reads and parses config.json from configDir.
func LoadConfig(configDir string) (*Project, error) {
	data, err := os.ReadFile(configDir + "/config.json")
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
