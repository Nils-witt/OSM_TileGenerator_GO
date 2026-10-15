package tilestore

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

// Zip archives every file under tilePath (the {z}/{x}/{y}.png tile pyramid)
// into a single zip file at destPath, preserving the relative directory
// structure. Files are read through an os.Root scoped to tilePath, so a
// symlink pointing outside the pyramid fails the archive instead of leaking
// that file into it. Close errors are reported because the zip central
// directory is only written when the writer is closed; ignoring them would
// leave a truncated archive behind.
func Zip(tilePath, destPath string) (err error) {
	root, err := os.OpenRoot(tilePath)
	if err != nil {
		return fmt.Errorf("Zip: opening %s: %w", tilePath, err)
	}
	defer func() { _ = root.Close() }() // read-only: a close error cannot lose data

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

	return fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		return addZipEntry(zw, root, name)
	})
}

// addZipEntry copies the file name (slash-separated, relative to root) into a
// new zip entry of the same name. PNGs are already deflate-compressed, so they
// are stored as-is: deflating them again costs CPU and saves next to nothing.
func addZipEntry(zw *zip.Writer, root *os.Root, name string) error {
	file, err := root.Open(name)
	if err != nil {
		return fmt.Errorf("Zip: opening %s: %w", name, err)
	}
	defer func() { _ = file.Close() }() // read-only: a close error cannot lose data

	method := zip.Deflate
	if path.Ext(name) == ".png" {
		method = zip.Store
	}

	writer, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
	if err != nil {
		return fmt.Errorf("Zip: creating zip entry %s: %w", name, err)
	}

	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("Zip: writing %s: %w", name, err)
	}

	return nil
}
