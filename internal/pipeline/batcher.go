package pipeline

import "sort"

// Batch is a group of files that together fit under one Gemini prompt's token
// budget. The generator (Stage 4) turns each Batch into exactly one API call.
type Batch struct {
	Files       []ParsedFile
	TotalTokens int
}

// estimateTokens approximates the token count of a string. Rule of thumb for
// code/English: ~4 characters per token. len() is bytes (>= runes), so we never
// under-estimate multibyte content — a safe upper bound.
func estimateTokens(s string) int {
	return len(s) / 4
}

// BatchFiles greedily packs files into the fewest batches that each stay under
// maxTokens. Tier1 files are placed first so they land in the earliest batches.
// Pure function: no I/O, no goroutines — fully deterministic and table-testable.
func BatchFiles(files []ParsedFile, maxTokens int) []Batch {
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].Tier < files[j].Tier
	})

	var result []Batch
	var current Batch

	for _, f := range files {
		tokens := estimateTokens(f.Content)

		if len(current.Files) > 0 && current.TotalTokens+tokens > maxTokens {
			result = append(result, current)
			current = Batch{}
		}

		current.Files = append(current.Files, f)
		current.TotalTokens += tokens
	}

	// Drain the buffer: the final batch is still in `current` when the loop
	// ends. Without this, the last batch is silently dropped.
	if len(current.Files) > 0 {
		result = append(result, current)
	}

	return result
}
