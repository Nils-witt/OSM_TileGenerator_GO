// Package workspace describes the on-disk layout of a tile generation
// project directory, so the rest of the program never builds these paths by
// hand.
package workspace

import (
	"os"
	"path/filepath"
)

// Dir is the root of a project directory, laid out as:
//
//	config.json  project configuration
//	image.png    source overlay image
//	tiles/       generated {z}/{x}/{y}.png tile pyramid
//	tmp/         intermediate warp and crop images
//	tiles.zip    zipped tile pyramid
type Dir string

// ConfigPath returns the path of config.json.
func (d Dir) ConfigPath() string { return filepath.Join(string(d), "config.json") }

// ImagePath returns the path of the source overlay image.
func (d Dir) ImagePath() string { return filepath.Join(string(d), "image.png") }

// TilesDir returns the root of the generated tile pyramid.
func (d Dir) TilesDir() string { return filepath.Join(string(d), "tiles") }

// TmpDir returns the directory for intermediate images.
func (d Dir) TmpDir() string { return filepath.Join(string(d), "tmp") }

// ZipPath returns the path of the zipped tile pyramid.
func (d Dir) ZipPath() string { return filepath.Join(string(d), "tiles.zip") }

// Create makes the project, tiles and tmp directories. It is idempotent.
func (d Dir) Create() error {
	for _, dir := range []string{string(d), d.TilesDir(), d.TmpDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
