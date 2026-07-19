package pipeline

import (
	"strings"
	"testing"
)

// tokensOf builds a ParsedFile whose Content is exactly `tokens` tokens under
// estimateTokens (len/4). 4 bytes per token, so 4*tokens bytes => tokens tokens.
func tokensOf(path string, tier FileTier, tokens int) ParsedFile {
	return ParsedFile{
		Path:    path,
		Tier:    tier,
		Content: strings.Repeat("x", tokens*4),
	}
}

// TestBatchFiles_Count checks the core packing contract: how many batches come
// out for a given set of file sizes and a token budget.
func TestBatchFiles_Count(t *testing.T) {
	cases := []struct {
		name        string
		files       []ParsedFile
		maxTokens   int
		wantBatches int
	}{
		{
			name:        "empty input",
			files:       nil,
			maxTokens:   100,
			wantBatches: 0,
		},
		{
			name:        "one small file",
			files:       []ParsedFile{tokensOf("a", 0, 30)},
			maxTokens:   100,
			wantBatches: 1,
		},
		{
			name: "two files fit in one batch",
			files: []ParsedFile{
				tokensOf("a", 0, 30),
				tokensOf("b", 0, 50),
			},
			maxTokens:   100,
			wantBatches: 1,
		},
		{
			name: "two files need a split",
			files: []ParsedFile{
				tokensOf("a", 0, 60),
				tokensOf("b", 0, 60), // 60+60=120 > 100, forces second batch
			},
			maxTokens:   100,
			wantBatches: 2,
		},
		{
			// Documents the "keep it anyway" edge-case choice: a file larger
			// than the whole budget gets its own oversized batch.
			name:        "oversized single file",
			files:       []ParsedFile{tokensOf("huge", 0, 150)},
			maxTokens:   100,
			wantBatches: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BatchFiles(tc.files, tc.maxTokens)
			if len(got) != tc.wantBatches {
				t.Errorf("BatchFiles returned %d batches, want %d", len(got), tc.wantBatches)
			}
		})
	}
}

// TestBatchFiles_NoFileLost asserts every input file lands in exactly one
// batch — the packer must never silently drop a file (the classic
// forgot-to-append or forgot-to-drain bug).
func TestBatchFiles_NoFileLost(t *testing.T) {
	files := []ParsedFile{
		tokensOf("a", 0, 40),
		tokensOf("b", 0, 40),
		tokensOf("c", 0, 40), // three 40s into a 100 budget => 40+40=80, +40=120 split
	}

	batches := BatchFiles(files, 100)

	total := 0
	for _, b := range batches {
		total += len(b.Files)
	}
	if total != len(files) {
		t.Errorf("packed %d files across batches, want %d (a file was dropped)", total, len(files))
	}
}

// TestBatchFiles_TierPriority verifies Tier1 files are packed before Tier2, so
// the most important files land in the earliest batch and survive any
// downstream cap on Gemini calls.
func TestBatchFiles_TierPriority(t *testing.T) {
	const tier1, tier2 FileTier = 1, 2
	files := []ParsedFile{
		tokensOf("low-priority", tier2, 60),
		tokensOf("high-priority", tier1, 60), // sorts ahead despite later position
	}

	batches := BatchFiles(files, 100)

	if len(batches) == 0 || len(batches[0].Files) == 0 {
		t.Fatalf("expected at least one non-empty batch")
	}
	if got := batches[0].Files[0].Path; got != "high-priority" {
		t.Errorf("first batch leads with %q, want the Tier1 file %q", got, "high-priority")
	}
}
