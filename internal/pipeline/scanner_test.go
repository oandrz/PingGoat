package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a file at root/relPath, making any parent dirs it needs.
// t.Helper() makes failures point at the caller's line, not this function.
func writeFile(t *testing.T, root, relPath string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte("// content"), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

// contains reports whether any path in got ends with wantSuffix. We match on
// suffix because ScanFiles returns absolute paths under a temp dir.
func contains(got []string, wantSuffix string) bool {
	for _, p := range got {
		if filepath.ToSlash(p) == wantSuffix || hasPathSuffix(p, wantSuffix) {
			return true
		}
	}
	return false
}

func hasPathSuffix(full, suffix string) bool {
	full = filepath.ToSlash(full)
	return len(full) >= len(suffix) && full[len(full)-len(suffix):] == suffix
}

func TestScanFiles(t *testing.T) {
	root := t.TempDir()

	// Files we expect to KEEP.
	writeFile(t, root, "main.go")
	writeFile(t, root, "internal/handler.go") // nested source is still found

	// Files we expect to SKIP (shouldSkipFile).
	writeFile(t, root, "go.sum")    // lock file
	writeFile(t, root, "logo.png")  // binary/image
	writeFile(t, root, "api.pb.go") // generated code

	// Files inside pruned directories — should never be visited.
	writeFile(t, root, ".git/config")
	writeFile(t, root, "vendor/dep.go")

	got, err := ScanFiles(root, 50)
	if err != nil {
		t.Fatalf("ScanFiles returned error: %v", err)
	}

	keep := []string{"main.go", "internal/handler.go"}
	for _, want := range keep {
		if !contains(got, want) {
			t.Errorf("expected %q to be kept, but it was missing from %v", want, got)
		}
	}

	skip := []string{"go.sum", "logo.png", "api.pb.go", ".git/config", "vendor/dep.go"}
	for _, bad := range skip {
		if contains(got, bad) {
			t.Errorf("expected %q to be skipped, but it appeared in %v", bad, got)
		}
	}
}

func TestScanFiles_RespectsMaxFiles(t *testing.T) {
	root := t.TempDir()

	// Create 10 keepable files — well over the cap we'll pass.
	const fileCount = 10
	for i := 0; i < fileCount; i++ {
		writeFile(t, root, fmt.Sprintf("file%d.go", i))
	}

	const maxFiles = 3
	got, err := ScanFiles(root, maxFiles)
	if err != nil {
		t.Fatalf("ScanFiles returned error: %v", err)
	}

	if len(got) > maxFiles {
		t.Errorf("ScanFiles returned %d files, want at most %d", len(got), maxFiles)
	}
}
