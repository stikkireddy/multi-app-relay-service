package ui

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed static
var content embed.FS

func CopyEmbeddedFiles(destDir string) error {
	// Get current working directory
	destPath := destDir
	if !filepath.IsAbs(destDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		destPath = filepath.Join(cwd, destDir)
	}
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
			return err
		}
	}

	// Walk through the embedded files
	return fs.WalkDir(content, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// If it's a directory, skip it
		if d.IsDir() {
			return nil
		}

		// Read the file content
		fileContent, err := content.ReadFile(path)
		if err != nil {
			return err
		}

		// Create the file in the destination directory
		relativePath, err := filepath.Rel("static", path)
		if err != nil {
			return err
		}
		destFilePath := filepath.Join(destPath, relativePath)
		if err := os.WriteFile(destFilePath, fileContent, os.ModePerm); err != nil {
			return err
		}

		return nil
	})
}
