package pipeline

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestClone_HappyPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ws, err := Clone(ctx, CloneOptions{
		RepoURL: "https://github.com/oandrz/Kotatsu",
		Branch:  "devel",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Cleanup()

	if len(ws.CommitSHA) != 40 {
		t.Errorf("bad SHA: %q", ws.CommitSHA)
	}

	if _, err := os.Stat(ws.Dir + "/README.md"); err != nil {
		t.Errorf("README missing: %v", err)
	}
}

func TestClone_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	_, err := Clone(ctx, CloneOptions{
		RepoURL: "https://github.com/oandrz/Kotatsu",
		Branch:  "devel",
	})
	if err == nil {
		t.Fatal("expected timeout, got nil")
	}
}
