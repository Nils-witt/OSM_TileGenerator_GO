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
// structure. Close errors are reported because the zip central directory is
// only written when the writer is closed; ignoring them would leave a
// truncated archive behind.
func Zip(tilePath, destPath string) (err error) {
	zipFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("Zip: creating %s: %w", destPath, err)
	}
	defer func() {
		if cerr := zipFile.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("Zip: closing %s: %w", destPath, cerr)
		}
	}()

	zw := zip.NewWriter(zipFile)
	defer func() {
		if cerr := zw.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("Zip: finalizing %s: %w", destPath, cerr)
		}
	}()

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
