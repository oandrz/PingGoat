package pipeline

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func ScanFiles(root string, maxFiles int) ([]string, error) {
	var results []string

	err := filepath.WalkDir(
		root,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules" {
					return filepath.SkipDir
				}

				return nil
			}

			// File branch: skip the noise, collect the signal.
			if shouldSkipFile(d) {
				return nil
			}
			results = append(results, path)

			if len(results) >= maxFiles {
				return filepath.SkipAll
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// shouldSkipFile reports whether a regular file is noise we should NOT collect:
// lock files, binaries/images, and generated code (e.g. *.pb.go). PRD §8.
func shouldSkipFile(d fs.DirEntry) bool {
	name := d.Name()

	if name == "go.sum" || name == "yarn.lock" || name == "Cargo.lock" {
		return true
	}

	if strings.HasSuffix(name, ".png") ||
		strings.HasSuffix(name, ".jpg") ||
		strings.HasSuffix(name, ".pb.go") {
		return true
	}

	return false
}
