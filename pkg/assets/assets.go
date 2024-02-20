package assets

import (
	"embed"
	"io"
	"os"
	"path/filepath"
)

//go:embed templates/* assets/*
var Assets embed.FS

func CopyTo(srcDir, destDir string) error {
	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// List all files and directories in the embedded FS
	entries, err := Assets.ReadDir(srcDir)
	if err != nil {
		return err
	}

	// Loop through each entry (file or directory)
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			// If it's a directory, recursively copy its contents
			if err := CopyTo(srcPath, destPath); err != nil {
				return err
			}
		} else {
			// If it's a file, copy it to the destination directory
			srcFile, err := Assets.Open(srcPath)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer destFile.Close()

			if _, err := io.Copy(destFile, srcFile); err != nil {
				return err
			}
		}
	}

	return nil
}

func ReadTemplate(name string) ([]byte, error) {
	return Assets.ReadFile(filepath.Join("templates", name))
}
