package tilestore

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Zip archives every file under tilePath (the {z}/{x}/{y}.png tile pyramid)
// into a single zip file at destPath, preserving the relative directory
// structure.
func Zip(tilePath, destPath string) error {
	zipFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("Zip: creating %s: %w", destPath, err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	return filepath.Walk(tilePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(tilePath, path)
		if err != nil {
			return fmt.Errorf("Zip: relative path for %s: %w", path, err)
		}

		writer, err := zw.Create(filepath.ToSlash(relPath))
		if err != nil {
			return fmt.Errorf("Zip: creating zip entry %s: %w", relPath, err)
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("Zip: opening %s: %w", path, err)
		}
		defer file.Close()

		if _, err := io.Copy(writer, file); err != nil {
			return fmt.Errorf("Zip: writing %s: %w", relPath, err)
		}

		return nil
	})
}
