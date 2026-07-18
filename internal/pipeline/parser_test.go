package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestParseFiles_ReadsEveryFile is the core contract: given N paths, ParseFiles
// returns N ParsedFiles whose Content matches what's on disk. We match by
// content (each file is unique) because the results arrive in a nondeterministic
// order — that's the nature of the concurrent fan-out.
func TestParseFiles_ReadsEveryFile(t *testing.T) {
	root := t.TempDir()

	files := map[string]string{
		"main.go":             "package main // unique-A",
		"internal/handler.go": "package internal // unique-B",
		"go.mod":              "module pinggoat // unique-C",
	}

	var paths []string
	wantContent := make(map[string]bool)
	wantPaths := make(map[string]bool)
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		paths = append(paths, full)
		wantContent[content] = true
		wantPaths[rel] = true
	}

	got, err := ParseFiles(context.Background(), root, paths, 4)
	if err != nil {
		t.Fatalf("ParseFiles returned error: %v", err)
	}

	if len(got) != len(paths) {
		t.Fatalf("ParseFiles returned %d files, want %d", len(got), len(paths))
	}

	for _, pf := range got {
		if !wantContent[pf.Content] {
			t.Errorf("unexpected content in result: %q", pf.Content)
		}
		// Path must be relative to root (e.g. "internal/handler.go"),
		// not the absolute temp-dir path.
		if !wantPaths[pf.Path] {
			t.Errorf("unexpected path in result: %q (want relative to root)", pf.Path)
		}
	}
}
